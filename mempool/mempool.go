/**
 *  @file
 *  @copyright defined in aergo/LICENSE.txt
 */

package mempool

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"io"
	"math/big"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aergoio/aergo-actor/actor"
	"github.com/aergoio/aergo-actor/router"
	"github.com/aergoio/aergo-lib/log"
	"github.com/aergoio/aergo/account/key"
	cfg "github.com/aergoio/aergo/config"
	"github.com/aergoio/aergo/contract/system"
	"github.com/aergoio/aergo/internal/enc"
	"github.com/aergoio/aergo/message"
	"github.com/aergoio/aergo/pkg/component"
	"github.com/aergoio/aergo/state"
	"github.com/aergoio/aergo/types"
	"github.com/golang/protobuf/proto"
)

const (
	initial = iota
	loading = iota
	running = iota
)

// MemPool is main structure of mempool service
type MemPool struct {
	*component.BaseComponent

	sync.RWMutex
	cfg *cfg.Config

	//curBestBlockHash
	sdb         *state.ChainStateDB
	bestBlockID types.BlockID
	stateDB     *state.StateDB
	verifier    *actor.PID
	orphan      int
	cache       map[types.TxID]*types.Tx
	pool        map[types.AccountID]*TxList
	dumpPath    string
	status      int32
	// followings are for test
	testConfig bool
	deadtx     int
}

// NewMemPoolService create and return new MemPool
func NewMemPoolService(cfg *cfg.Config, sdb *state.ChainStateDB) *MemPool {
	actor := &MemPool{
		cfg:      cfg,
		sdb:      sdb,
		cache:    map[types.TxID]*types.Tx{},
		pool:     map[types.AccountID]*TxList{},
		dumpPath: cfg.Mempool.DumpFilePath,
		status:   initial,
		verifier: nil,
		//testConfig:    true, // FIXME test config should be removed
	}

	actor.BaseComponent = component.NewBaseComponent(message.MemPoolSvc, actor, log.NewLogger("mempool"))

	return actor
}

// Start runs mempool servivce
func (mp *MemPool) BeforeStart() {
	if mp.testConfig {
		initStubData()
		mp.bestBlockID = getCurrentBestBlockNoMock()
	}
	if mp.cfg.Mempool.ShowMetrics {
		go func() {
			for range time.Tick(1e9) {
				l, o := mp.Size()
				mp.Info().Int("len", l).Int("orphan", o).Int("acc", len(mp.pool)).Msg("mempool metrics")
			}
		}()
	}
	//mp.Info("mempool start on: current Block :", mp.curBestBlockNo)
}
func (mp *MemPool) AfterStart() {

	mp.Info().Int("number of verifier", mp.cfg.Mempool.VerifierNumber).Msg("init")
	mp.verifier = actor.Spawn(router.NewRoundRobinPool(mp.cfg.Mempool.VerifierNumber).
		WithInstance(NewTxVerifier(mp)))

	rsp, err := mp.RequestToFuture(message.ChainSvc, &message.GetBestBlock{}, time.Second*2).Result()
	if err != nil {
		mp.Error().Err(err).Msg("failed to get best block")
		panic("Mempool AfterStart Failed")
	}
	bestblock := rsp.(message.GetBestBlockRsp).Block
	mp.setStateDB(bestblock) // nolint: errcheck
}

// Stop handles clean-up for mempool service
func (mp *MemPool) BeforeStop() {
	if mp.verifier != nil {
		mp.verifier.GracefulStop()
	}
	mp.dumpTxsToFile()
}

// Size returns current maintaining number of transactions
// and number of orphan transaction
func (mp *MemPool) Size() (int, int) {
	mp.RLock()
	defer mp.RUnlock()
	return len(mp.cache), mp.orphan
}

// Receive handles requested messages from other services
func (mp *MemPool) Receive(context actor.Context) {

	switch msg := context.Message().(type) {
	case *message.MemPoolPut:
		mp.verifier.Request(msg.Tx, context.Sender())
	case *message.MemPoolGet:
		txs, err := mp.get(msg.MaxBlockBodySize)
		context.Respond(&message.MemPoolGetRsp{
			Txs: txs,
			Err: err,
		})
	case *message.MemPoolDel:
		errs := mp.removeOnBlockArrival(msg.Block)
		context.Respond(&message.MemPoolDelRsp{
			Err: errs,
		})
	case *message.MemPoolExist:
		tx := mp.exists(msg.Hash)
		context.Respond(&message.MemPoolExistRsp{
			Tx: tx,
		})
	case *actor.Started:
		mp.loadTxs() // FIXME :work-around for actor settled

	default:
		//mp.Debug().Str("type", reflect.TypeOf(msg).String()).Msg("unhandled message")
	}
}

func (mp *MemPool) Statistics() *map[string]interface{} {
	return &map[string]interface{}{
		"total":  len(mp.cache),
		"orphan": mp.orphan,
		"dead":   mp.deadtx,
	}
}

func (mp *MemPool) get(maxBlockBodySize uint32) ([]*types.Tx, error) {
	start := time.Now()
	mp.RLock()
	defer mp.RUnlock()
	count := 0
	size := 0
	txs := make([]*types.Tx, 0)
Gather:
	for _, list := range mp.pool {
		for _, tx := range list.Get() {
			if size += proto.Size(tx); uint32(size) > maxBlockBodySize {
				break Gather
			}
			txs = append(txs, tx)
			count++
		}
	}
	elapsed := time.Since(start)
	mp.Debug().Int("size", size).Uint32("max", maxBlockBodySize).Msg("chris2nd")
	mp.Debug().Str("elapsed", elapsed.String()).Int("len", len(mp.cache)).Int("orphan", mp.orphan).Int("count", count).Msg("total tx returned")
	return txs, nil
}

// check existence.
// validate
// add pool if possible, else pendings
func (mp *MemPool) put(tx *types.Tx) error {
	id := types.ToTxID(tx.GetHash())
	acc := tx.GetBody().GetAccount()

	mp.Lock()
	defer mp.Unlock()
	if _, found := mp.cache[id]; found {
		return types.ErrTxAlreadyInMempool
	}
	/*
		err := mp.verifyTx(tx)
		if err != nil {
			return err
		}
	*/
	err := mp.validateTx(tx)
	if err != nil && err != types.ErrTxNonceToohigh {
		return err
	}

	list, err := mp.acquireMemPoolList(acc)
	if err != nil {
		return err
	}
	defer mp.releaseMemPoolList(list)
	diff, err := list.Put(tx)
	if err != nil {
		mp.Debug().Err(err).Msg("fail to put at a mempool list")
		return err
	}

	mp.orphan -= diff
	mp.cache[id] = tx
	//mp.Debugf("tx add-ed size(%d, %d)[%s]", len(mp.cache), mp.orphan, tx.GetBody().String())

	if !mp.testConfig {
		mp.notifyNewTx(*tx)
	}
	return nil
}
func (mp *MemPool) puts(txs ...*types.Tx) []error {
	errs := make([]error, len(txs))
	for i, tx := range txs {
		errs[i] = mp.put(tx)
	}
	return errs
}

func (mp *MemPool) setStateDB(block *types.Block) bool {
	if mp.testConfig {
		return true
	}

	newBlockID := types.ToBlockID(block.BlockHash())
	parentBlockID := types.ToBlockID(block.GetHeader().GetPrevBlockHash())
	normal := true

	if types.HashID(newBlockID).Compare(types.HashID(mp.bestBlockID)) != 0 {
		if types.HashID(parentBlockID).Compare(types.HashID(mp.bestBlockID)) != 0 {
			normal = false
		}
		mp.bestBlockID = newBlockID

		stateRoot := block.GetHeader().GetBlocksRootHash()
		if mp.stateDB == nil {
			mp.stateDB = mp.sdb.OpenNewStateDB(stateRoot)
			mp.Debug().Str("Hash", newBlockID.String()).
				Str("StateRoot", types.ToHashID(stateRoot).String()).
				Msg("new StateDB opened")
		} else if !bytes.Equal(mp.stateDB.GetRoot(), stateRoot) {
			if err := mp.stateDB.SetRoot(stateRoot); err != nil {
				mp.Error().Err(err).Msg("failed to set root of StateDB")
			}
		}
	}
	return normal
}

// input tx based ? or pool based?
// concurrency consideration,
func (mp *MemPool) removeOnBlockArrival(block *types.Block) error {
	start := time.Now()
	mp.Lock()
	defer mp.Unlock()

	check := 0
	all := false
	dirty := map[types.AccountID]bool{}

	if !mp.setStateDB(block) {
		all = true
		mp.Debug().Int("cnt", len(mp.pool)).Msg("going to check all account's state")
	} else {
		for _, tx := range block.GetBody().GetTxs() {
			dirty[types.ToAccountID(tx.GetBody().GetAccount())] = true
			dirty[types.ToAccountID(tx.GetBody().GetRecipient())] = true
		}
	}

	for acc, list := range mp.pool {
		if !all && dirty[acc] == false {
			continue
		}
		ns, err := mp.getAccountState(list.GetAccount())
		if err != nil {
			mp.Error().Err(err).Msg("getting Account status failed during removal")
			// TODO : ????
			continue
		}
		diff, delTxs := list.FilterByState(ns)
		mp.orphan -= diff
		for _, tx := range delTxs {
			delete(mp.cache, types.ToTxID(tx.GetHash())) // need lock
		}
		mp.releaseMemPoolList(list)
		check++
	}

	//FOR TEST
	for _, tx := range block.GetBody().GetTxs() {
		hid := types.ToTxID(tx.GetHash())
		if _, ok := mp.cache[hid]; !ok {
			continue
		}
		mp.Warn().Uint64("nonce on tx", tx.GetBody().GetNonce()).
			Msg("mismatch ditected")
		mp.deadtx++
	}
	elapse := time.Since(start)
	mp.Debug().Int("given", len(block.GetBody().GetTxs())).
		Int("check", check).
		Str("elapse", elapse.String()).
		Msg("delete txs on block")
	return nil
}

// signiture verification
func (mp *MemPool) verifyTx(tx *types.Tx) error {
	err := tx.Validate()
	if err != nil {
		return err
	}
	err = key.VerifyTx(tx)
	if err != nil {
		return err
	}
	return nil
}

// check tx sanity
// check if sender has enough balance
// check tx account is lower than known value
func (mp *MemPool) validateTx(tx *types.Tx) error {
	account := tx.GetBody().GetAccount()
	ns, err := mp.getAccountState(account)
	if err != nil {
		return err
	}
	err = tx.ValidateWithSenderState(ns)
	if err != nil {
		return err
	}
	switch tx.GetBody().GetType() {
	//case types.TxType_NORMAL:
	case types.TxType_GOVERNANCE:
		aergoSystemState, err := mp.getAccountState(tx.GetBody().GetRecipient())
		if err != nil {
			return err
		}
		aid := types.ToAccountID(tx.GetBody().GetRecipient())
		scs, err := mp.stateDB.OpenContractState(aid, aergoSystemState)
		if err != nil {
			return err
		}
		err = system.ValidateSystemTx(tx.GetBody(), scs, system.FutureBlockNo)
		if err != nil {
			return err
		}
	}
	return nil
}

func (mp *MemPool) exists(hash []byte) *types.Tx {
	mp.RLock()
	defer mp.RUnlock()
	if v, ok := mp.cache[types.ToTxID(hash)]; ok {
		return v
	}
	return nil
}

func (mp *MemPool) acquireMemPoolList(acc []byte) (*TxList, error) {
	list := mp.getMemPoolList(acc)
	if list != nil {
		return list, nil
	}
	ns, err := mp.getAccountState(acc)
	if err != nil {
		return nil, err
	}
	id := types.ToAccountID(acc)
	mp.pool[id] = NewTxList(acc, ns)
	return mp.pool[id], nil
}

func (mp *MemPool) releaseMemPoolList(list *TxList) {
	if list.Empty() {
		id := types.ToAccountID(list.account)
		delete(mp.pool, id)
	}
}

func (mp *MemPool) getMemPoolList(acc []byte) *TxList {
	id := types.ToAccountID(acc)
	return mp.pool[id]
}

func (mp *MemPool) getAccountState(acc []byte) (*types.State, error) {
	if mp.testConfig {
		aid := types.ToAccountID(acc)
		strAcc := aid.String()
		bal := getBalanceByAccMock(strAcc)
		nonce := getNonceByAccMock(strAcc)
		//mp.Error().Str("acc:", strAcc).Int("nonce", int(nonce)).Msg("")
		return &types.State{Balance: new(big.Int).SetUint64(bal).Bytes(), Nonce: nonce}, nil
	}

	state, err := mp.stateDB.GetAccountState(types.ToAccountID(acc))

	if err != nil {
		mp.Fatal().Err(err).Str("sroot", enc.ToString(mp.stateDB.GetRoot())).Msg("failed to get state")

		//FIXME PANIC?
		//mp.Fatal().Err(err).Msg("failed to get state")
		return nil, err
	}
	/*
		if state.Balance == 0 {
			strAcc := types.EncodeAddress(acc)
			mp.Info().Str("address", strAcc).Msg("w t f")

		}
	*/
	return state, nil
}

func (mp *MemPool) notifyNewTx(tx types.Tx) {
	mp.RequestTo(message.P2PSvc, &message.NotifyNewTransactions{
		Txs: []*types.Tx{&tx},
	})
}

func (mp *MemPool) loadTxs() {
	time.Sleep(time.Second) // FIXME
	if !atomic.CompareAndSwapInt32(&mp.status, initial, loading) {
		return
	}
	defer atomic.StoreInt32(&mp.status, running)
	file, err := os.Open(mp.dumpPath)
	if err != nil {
		if !os.IsNotExist(err) {
			mp.Error().Err(err).Msg("Unable to open dump file")
		}
		return
	}
	reader := csv.NewReader(bufio.NewReader(file))

	var count int
	for {
		buf := types.Tx{}
		rc, err := reader.Read()
		if err != nil {
			if err != io.EOF {
				mp.Error().Err(err).Msg("err on read file during loading")
			}
			break
		}
		count++
		dataBuf, err := enc.ToBytes(rc[0])
		if err != nil {
			mp.Error().Err(err).Msg("err on decoding tx during loading")
			continue
		}
		err = proto.Unmarshal(dataBuf, &buf)
		if err != nil {
			mp.Error().Err(err).Msg("errr on unmarshalling tx during loading")
			continue
		}
		mp.put(&buf) // nolint: errcheck
	}

	mp.Info().Int("try", count).
		Int("drop", count-len(mp.cache)-mp.orphan).
		Int("suceed", len(mp.cache)).
		Int("orphan", mp.orphan).
		Msg("loading mempool done")
}

func (mp *MemPool) isRunning() bool {
	if atomic.LoadInt32(&mp.status) != running {
		mp.Info().Msg("skip to dump txs because mempool is not running yet")
		return false
	}
	return true
}
func (mp *MemPool) dumpTxsToFile() {

	if !mp.isRunning() {
		return
	}
	if len, _ := mp.Size(); len == 0 {
		os.Remove(mp.dumpPath) // nolint: errcheck
		return
	}

	file, err := os.Create(mp.dumpPath)
	if err != nil {
		mp.Error().Err(err).Msg("Unable to create file")
		return
	}
	defer file.Close() // nolint: errcheck

	writer := csv.NewWriter(bufio.NewWriter(file))
	defer writer.Flush() //nolint: errcheck

	mp.Lock()
	defer mp.Unlock()
	count := 0
	for _, list := range mp.pool {
		for _, v := range list.GetAll() {
			data, err := proto.Marshal(v)
			if err != nil {
				continue
			}

			strData := enc.ToString(data)
			err = writer.Write([]string{strData})
			if err != nil {
				mp.Info().Err(err).Msg("writing encoded tx fail")
				break
			}
			count++
		}
	}
	mp.Info().Int("count", count).Str("path", mp.dumpPath).Msg("dump txs")
}
