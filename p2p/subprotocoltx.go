/*
 * @file
 * @copyright defined in aergo/LICENSE.txt
 */

package p2p

import (
	"github.com/aergoio/aergo-lib/log"
	"github.com/aergoio/aergo/message"
	"github.com/aergoio/aergo/types"
	"github.com/golang/protobuf/proto"
)

type txRequestHandler struct {
	BaseMsgHandler
	msgHelper message.Helper
}

var _ MessageHandler = (*txRequestHandler)(nil)

type txResponseHandler struct {
	BaseMsgHandler
}

var _ MessageHandler = (*txResponseHandler)(nil)

type newTxNoticeHandler struct {
	BaseMsgHandler
}

var _ MessageHandler = (*newTxNoticeHandler)(nil)

// newTxReqHandler creates handler for GetTransactionsRequest
func newTxReqHandler(pm PeerManager, peer RemotePeer, logger *log.Logger, actor ActorService) *txRequestHandler {
	th := &txRequestHandler{BaseMsgHandler: BaseMsgHandler{protocol: GetTXsRequest, pm: pm, peer: peer, actor: actor, logger: logger}}
	th.msgHelper = message.GetHelper()
	return th
}

func (th *txRequestHandler) parsePayload(rawbytes []byte) (proto.Message, error) {
	return unmarshalAndReturn(rawbytes, &types.GetTransactionsRequest{})
}

func (th *txRequestHandler) handle(msg Message, msgBody proto.Message) {
	peerID := th.peer.ID()
	remotePeer := th.peer
	data := msgBody.(*types.GetTransactionsRequest)
	debugLogReceiveMsg(th.logger, th.protocol, msg.ID().String(), peerID, len(data.Hashes))

	// TODO consider to make async if deadlock with remote peer can occurs
	// NOTE size estimation is tied to protobuf3 it should be changed when protobuf is changed.
	// find transactions from chainservice
	idx := 0
	status := types.ResultStatus_OK
	hashes := make([][]byte, 0, 100)
	txInfos := make([]*types.Tx, 0, 100)
	payloadSize := EmptyGetBlockResponseSize
	var txSize, fieldSize int
	for _, hash := range data.Hashes {
		tx, err := th.msgHelper.ExtractTxFromResponseAndError(th.actor.CallRequestDefaultTimeout(message.MemPoolSvc,
			&message.MemPoolExist{Hash: hash}))
		if err != nil {
			// response error to peer
			resp := &types.GetTransactionsResponse{Status: types.ResultStatus_INTERNAL}
			remotePeer.sendMessage(remotePeer.MF().newMsgResponseOrder(msg.ID(), GetTxsResponse, resp))
			return
		}
		if tx == nil {
			// ignore not existing hash
			continue
		}
		txSize = proto.Size(tx)
		fieldSize = txSize + calculateFieldDescSize(txSize)
		fieldSize += len(hash)+calculateFieldDescSize(len(hash))
		if (payloadSize + fieldSize)  > MaxPayloadLength {
			// send partial list
			resp := &types.GetTransactionsResponse{
				Status: status,
				Hashes: hashes,
				Txs:    txInfos, HasNext:true}
			th.logger.Debug().Int(LogTxCount, len(hashes)).Str("req_id",msg.ID().String()).Msg("Sending partial response")
			remotePeer.sendMessage(remotePeer.MF().newMsgResponseOrder(msg.ID(), GetTxsResponse, resp))
			// reset list
			hashes = make([][]byte, 0, 100)
			txInfos = make([]*types.Tx, 0, 100)
			payloadSize = EmptyGetBlockResponseSize
		}
		hashes = append(hashes, hash)
		txInfos = append(txInfos, tx)
		payloadSize += fieldSize
		idx++
	}

	// send remained blocks
	if 0 == idx {
		status = types.ResultStatus_NOT_FOUND
	}
	th.logger.Debug().Int(LogTxCount, len(hashes)).Str("req_id",msg.ID().String()).Msg("Sending last part response")
	// generate response message
	resp := &types.GetTransactionsResponse{
		Status: status,
		Hashes: hashes,
		Txs:    txInfos, HasNext:false}

	remotePeer.sendMessage(remotePeer.MF().newMsgResponseOrder(msg.ID(), GetTxsResponse, resp))
}

// newTxRespHandler creates handler for GetTransactionsResponse
func newTxRespHandler(pm PeerManager, peer RemotePeer, logger *log.Logger, actor ActorService) *txResponseHandler {
	th := &txResponseHandler{BaseMsgHandler: BaseMsgHandler{protocol: GetTxsResponse, pm: pm, peer: peer, actor: actor, logger: logger}}
	return th
}

func (th *txResponseHandler) parsePayload(rawbytes []byte) (proto.Message, error) {
	return unmarshalAndReturn(rawbytes, &types.GetTransactionsResponse{})
}

func (th *txResponseHandler) handle(msg Message, msgBody proto.Message) {
	peerID := th.peer.ID()
	data := msgBody.(*types.GetTransactionsResponse)
	debugLogReceiveResponseMsg(th.logger, th.protocol, msg.ID().String(), msg.OriginalID().String(), peerID, len(data.Txs))

	// TODO: Is there any better solution than passing everything to mempool service?
	if len(data.Txs) > 0 {
		th.logger.Debug().Int(LogTxCount, len(data.Txs)).Msg("Request mempool to add txs")
		//th.actor.SendRequest(message.MemPoolSvc, &message.MemPoolPut{Txs: data.Txs})
		for _, tx := range data.Txs {
			th.actor.SendRequest(message.MemPoolSvc, &message.MemPoolPut{Tx: tx})
		}
	}
}

// newNewTxNoticeHandler creates handler for GetTransactionsResponse
func newNewTxNoticeHandler(pm PeerManager, peer RemotePeer, logger *log.Logger, actor ActorService, sm SyncManager) *newTxNoticeHandler {
	th := &newTxNoticeHandler{BaseMsgHandler: BaseMsgHandler{protocol: NewTxNotice, pm: pm, sm: sm, peer: peer, actor: actor, logger: logger}}
	return th
}

func (th *newTxNoticeHandler) parsePayload(rawbytes []byte) (proto.Message, error) {
	return unmarshalAndReturn(rawbytes, &types.NewTransactionsNotice{})
}

func (th *newTxNoticeHandler) handle(msg Message, msgBody proto.Message) {
	peerID := th.peer.ID()
	data := msgBody.(*types.NewTransactionsNotice)
	// remove to verbose log
	if th.logger.IsDebugEnabled() {
		debugLogReceiveMsg(th.logger, th.protocol, msg.ID().String(), peerID, bytesArrToString(data.TxHashes))
	}

	if len(data.TxHashes) == 0 {
		return
	}
	// lru cache can accept hashable key
	hashes := make([]TxHash, len(data.TxHashes))
	for i, hash := range data.TxHashes {
		copy(hashes[i][:], hash)
	}
	added := th.peer.updateTxCache(hashes)
	if len(added) > 0 {
		th.sm.HandleNewTxNotice(th.peer, added, data)
	}
}
