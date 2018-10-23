/*
 * @file
 * @copyright defined in aergo/LICENSE.txt
 */

package p2p

import (
	"bytes"
	"fmt"
	"github.com/aergoio/aergo-lib/log"
	"github.com/aergoio/aergo/internal/enc"
	"github.com/aergoio/aergo/message"
	"github.com/aergoio/aergo/types"
	"github.com/hashicorp/golang-lru"
	"github.com/libp2p/go-libp2p-peer"
	"reflect"
	"sync"
)

type SyncManager interface {
	HandleNewBlockNotice(peer RemotePeer, hash BlkHash, data *types.NewBlockNotice)
	HandleGetBlockResponse(peer RemotePeer, msg Message, resp *types.GetBlockResponse)
	HandleNewTxNotice(peer RemotePeer, hashes []TxHash, data *types.NewTransactionsNotice)

	DoSync(peer RemotePeer, hashes []message.BlockHash, stopHash message.BlockHash)
}

type syncManager struct {
	logger *log.Logger
	actor  ActorService
	pm     PeerManager

	blkCache *lru.Cache
	txCache  *lru.Cache

	syncLock *sync.Mutex
	syncing  bool
	sw       *syncWorker
}

func newSyncManager(actor ActorService, pm PeerManager, logger *log.Logger) SyncManager {
	var err error
	sm := &syncManager{actor: actor, pm: pm, logger: logger, syncLock: &sync.Mutex{}}

	sm.blkCache, err = lru.New(DefaultGlobalBlockCacheSize)
	if err != nil {
		panic("Failed to create peermanager " + err.Error())
	}
	sm.txCache, err = lru.New(DefaultGlobalTxCacheSize)
	if err != nil {
		panic("Failed to create peermanager " + err.Error())
	}

	return sm
}

func (sm *syncManager) checkWorkToken() bool {
	sm.syncLock.Lock()
	defer sm.syncLock.Unlock()
	return !sm.syncing
}

func (sm *syncManager) getWorker(peerID peer.ID) (*syncWorker, bool) {
	sm.syncLock.Lock()
	defer sm.syncLock.Unlock()
	if !sm.syncing {
		return nil, false
	}

	sm.syncing = true
	return sm.sw, true
}

func (sm *syncManager) removeWorker() {
	sm.syncLock.Lock()
	defer sm.syncLock.Unlock()
	sm.syncing = false
	sm.sw = nil
}

func (sm *syncManager) HandleNewBlockNotice(peer RemotePeer, hashArr BlkHash, data *types.NewBlockNotice) {
	peerID := peer.ID()
	if !sm.checkWorkToken() {
		// just ignore it
		sm.logger.Debug().Str(LogBlkHash, enc.ToString(data.BlockHash)).Str(LogPeerID, peerID.Pretty()).Msg("Ignoring newBlock notice sync syncManager is busy now.")
		return
	}

	// TODO check if evicted return value is needed.
	ok, _ := sm.blkCache.ContainsOrAdd(hashArr, cachePlaceHolder)
	if ok {
		// Kickout duplicated notice log.
		// if sm.logger.IsDebugEnabled() {
		// 	sm.logger.Debug().Str(LogBlkHash, enc.ToString(data.BlkHash)).Str(LogPeerID, peerID.Pretty()).Msg("Got NewBlock notice, but sent already from other peer")
		// }
		// this notice is already sent to chainservice
		return
	}

	// request block info if selfnode does not have block already
	rawResp, err := sm.actor.CallRequest(message.ChainSvc, &message.GetBlock{BlockHash: message.BlockHash(data.BlockHash)})
	if err != nil {
		sm.logger.Warn().Err(err).Msg("actor return error on getblock")
		return
	}
	resp, ok := rawResp.(message.GetBlockRsp)
	if !ok {
		sm.logger.Warn().Str("expected", "message.GetBlockRsp").Str("actual", reflect.TypeOf(rawResp).Name()).Msg("chainservice returned unexpected type")
		return
	}
	if resp.Err != nil {
		//sm.logger.Debug().Str(LogBlkHash, enc.ToString(data.BlockHash)).Str(LogPeerID, peerID.Pretty()).Msg("chainservice responded that block not found. request back to notifier")
		sm.actor.SendRequest(message.P2PSvc, &message.GetBlockInfos{ToWhom: peerID,
			Hashes: []message.BlockHash{message.BlockHash(data.BlockHash)}})
	}
}

// HandleGetBlockResponse handle when remote peer send a block information.
func (sm *syncManager) HandleGetBlockResponse(peer RemotePeer, msg Message, resp *types.GetBlockResponse) {
	blocks := resp.Blocks
	peerID := peer.ID()
	worker, found := sm.getWorker(peerID)
	if found {
		worker.putAddBlock(msg, blocks, resp.HasNext)
	} else {
		// send to chainservice if no actor is found.
		for _, block := range blocks {
			sm.actor.TellRequest(message.ChainSvc, &message.AddBlock{PeerID: peerID, Block: block, Bstate: nil})
		}
	}
}

func (sm *syncManager) HandleNewTxNotice(peer RemotePeer, hashArrs []TxHash, data *types.NewTransactionsNotice) {
	peerID := peer.ID()

	// TODO it will cause problem if getTransaction failed. (i.e. remote peer was sent notice, but not response getTransaction)
	toGet := make([]message.TXHash, 0, len(data.TxHashes))
	for _, hashArr := range hashArrs {
		ok, _ := sm.txCache.ContainsOrAdd(hashArr, cachePlaceHolder)
		if ok {
			// Kickout duplicated notice log.
			// if sm.logger.IsDebugEnabled() {
			// 	sm.logger.Debug().Str(LogTxHash, enc.ToString(hashArr[:])).Str(LogPeerID, peerID.Pretty()).Msg("Got NewTx notice, but sent already from other peer")
			// }
			// this notice is already sent to chainservice
			continue
		}
		hash := make([]byte, txhashLen)
		copy(hash, hashArr[:])
		toGet = append(toGet, hash)
	}
	if len(toGet) == 0 {
		// sm.logger.Debug().Str(LogPeerID, peerID.Pretty()).Msg("No new tx found in tx notice")
		return
	}
	sm.logger.Debug().Str("hashes", txHashArrToString(toGet)).Msg("syncmanager request back unknown tx hashes")
	// create message data
	sm.actor.SendRequest(message.P2PSvc, &message.GetTransactions{ToWhom: peerID, Hashes: toGet})
}

func (sm *syncManager) DoSync(peer RemotePeer, hashes []message.BlockHash, stopHash message.BlockHash) {
	sm.syncLock.Lock()
	if sm.sw != nil {
		sm.syncLock.Unlock()
		sm.logger.Debug().Str(LogPeerID, peer.ID().Pretty()).Msg("ignore sync work")
		return
	}
	sm.sw = newSyncWorker(sm, peer, hashes, stopHash)
	sm.syncLock.Unlock()
	sm.logger.Debug().Str(LogPeerID, peer.ID().Pretty()).Str("my_hashes",blockHashArrToStringWithLimit(hashes, len(hashes))).Str("stop_hash", enc.ToString(stopHash)).Msg("Starting sync work to ")
	go sm.sw.runWorker()
}

func blockHashArrToString(bbarray []message.BlockHash) string {
	return blockHashArrToStringWithLimit(bbarray, 10)
}

func blockHashArrToStringWithLimit(bbarray []message.BlockHash, limit int ) string {
	var buf bytes.Buffer
	buf.WriteByte('[')
	var arrSize = len(bbarray)
	if limit > arrSize {
		limit = arrSize
	}
	for i :=0; i < limit; i++ {
		hash := bbarray[i]
		buf.WriteByte('"')
		buf.WriteString(enc.ToString([]byte(hash)))
		buf.WriteByte('"')
		buf.WriteByte(',')
	}
	if arrSize > limit {
		buf.WriteString(fmt.Sprintf(" (and %d more), ",  arrSize - limit))
	}
	buf.WriteByte(']')
	return buf.String()
}


// bytesArrToString converts array of byte array to json array of b58 encoded string.
func txHashArrToString(bbarray []message.TXHash) string {
	return txHashArrToStringWithLimit(bbarray, 10)
}

func txHashArrToStringWithLimit(bbarray []message.TXHash, limit int) string {
	var buf bytes.Buffer
	buf.WriteByte('[')
	var arrSize = len(bbarray)
	if limit > arrSize {
		limit = arrSize
	}
	for i := 0; i < limit; i++ {
		hash := bbarray[i]
		buf.WriteByte('"')
		buf.WriteString(enc.ToString([]byte(hash)))
		buf.WriteByte('"')
		buf.WriteByte(',')
	}
	if arrSize > limit {
		buf.WriteString(fmt.Sprintf(" (and %d more), ", arrSize-limit))
	}
	buf.WriteByte(']')
	return buf.String()
}

// bytesArrToString converts array of byte array to json array of b58 encoded string.
func P2PTxHashArrToString(bbarray []TxHash) string {
	return P2PTxHashArrToStringWithLimit(bbarray, 10)
}
func P2PTxHashArrToStringWithLimit(bbarray []TxHash, limit int) string {
	var buf bytes.Buffer
	buf.WriteByte('[')
	var arrSize = len(bbarray)
	if limit > arrSize {
		limit = arrSize
	}
	for i := 0; i < limit; i++ {
		hash := bbarray[i]
		buf.WriteByte('"')
		buf.WriteString(enc.ToString(hash[:]))
		buf.WriteByte('"')
		buf.WriteByte(',')
	}
	if arrSize > limit {
		buf.WriteString(fmt.Sprintf(" (and %d more), ", arrSize-limit))
	}
	buf.WriteByte(']')
	return buf.String()
}