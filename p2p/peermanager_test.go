/*
 * @file
 * @copyright defined in aergo/LICENSE.txt
 */
package p2p

import (
	"fmt"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aergoio/aergo-lib/log"
	cfg "github.com/aergoio/aergo/config"
	"github.com/aergoio/aergo/message"
	"github.com/aergoio/aergo/types"
	"github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
)

// Ignoring test for now, for lack of abstraction on AergoPeer struct
func IgrenoreTestP2PServiceRunAddPeer(t *testing.T) {
	mockActor := new(MockActorService)
	dummyBlock := types.Block{Hash: dummyBlockHash, Header: &types.BlockHeader{BlockNo: dummyBlockHeight}}
	mockActor.On("CallRequest", mock.Anything, mock.Anything).Return(message.GetBlockRsp{Block: &dummyBlock}, nil)
	mockMF := new(MockMoFactory)
	target := NewPeerManager(nil, mockActor,
		cfg.NewServerContext("", "").GetDefaultConfig().(*cfg.Config),
		nil, new(MockReconnectManager), nil,
		log.NewLogger("test.p2p"), mockMF).(*peerManager)

	target.Host = &mockHost{pstore.NewPeerstore(pstoremem.NewKeyBook(), pstoremem.NewAddrBook(), pstoremem.NewPeerMetadata())}
	target.selfMeta.ID = peer.ID("gwegw")
	go target.runManagePeers()

	sampleAddr1 := PeerMeta{ID: "ddd", IPAddress: "192.168.0.1", Port: 33888, Outbound: true}
	sampleAddr2 := PeerMeta{ID: "fff", IPAddress: "192.168.0.2", Port: 33888, Outbound: true}
	target.AddNewPeer(sampleAddr1)
	target.AddNewPeer(sampleAddr1)
	time.Sleep(time.Second)
	if len(target.Peerstore().Peers()) != 1 {
		t.Errorf("Peer count : Expected %d, Actually %d", 1, len(target.Peerstore().Peers()))
	}
	target.AddNewPeer(sampleAddr2)
	time.Sleep(time.Second * 1)
	if len(target.Peerstore().Peers()) != 2 {
		t.Errorf("Peer count : Expected %d, Actually %d", 2, len(target.Peerstore().Peers()))
	}
}

func FailTestGetPeers(t *testing.T) {
	mockActorServ := &MockActorService{}
	dummyBlock := types.Block{Hash: dummyBlockHash, Header: &types.BlockHeader{BlockNo: dummyBlockHeight}}
	mockActorServ.On("CallRequest", mock.Anything, mock.Anything).Return(message.GetBlockRsp{Block: &dummyBlock}, nil)
	mockMF := new(MockMoFactory)
	target := NewPeerManager(nil, mockActorServ,
		cfg.NewServerContext("", "").GetDefaultConfig().(*cfg.Config),
		nil, new(MockReconnectManager), nil,
		log.NewLogger("test.p2p"), mockMF).(*peerManager)

	iterSize := 500
	wg := sync.WaitGroup{}
	waitChan := make(chan int)
	wg.Add(1)
	go func() {
		for i := 0; i < iterSize; i++ {
			peerID := peer.ID(strconv.Itoa(i))
			peerMeta := PeerMeta{ID: peerID}
			target.remotePeers[peerID] = newRemotePeer(peerMeta, target, mockActorServ, logger, nil, nil, nil)
			if i == (iterSize >> 2) {
				wg.Done()
			}
		}
	}()

	go func() {
		wg.Wait()
		for key, val := range target.remotePeers {
			fmt.Printf("%s is %s\n", key.String(), val.State().String())
		}
		waitChan <- 0
	}()

	<-waitChan
}

func TestPeerManager_GetPeers(t *testing.T) {
	mockActorServ := &MockActorService{}
	dummyBlock := types.Block{Hash: dummyBlockHash, Header: &types.BlockHeader{BlockNo: dummyBlockHeight}}
	mockActorServ.On("CallRequest", mock.Anything, mock.Anything).Return(message.GetBlockRsp{Block: &dummyBlock}, nil)
	mockMF := new(MockMoFactory)

	tLogger := log.NewLogger("test.p2p")
	tConfig := cfg.NewServerContext("", "").GetDefaultConfig().(*cfg.Config)
	InitNodeInfo(tConfig.P2P, tLogger)
	target := NewPeerManager(nil, mockActorServ,
		tConfig,
		nil, new(MockReconnectManager), nil,
		tLogger, mockMF).(*peerManager)

	iterSize := 500
	wg := &sync.WaitGroup{}
	wgAll := &sync.WaitGroup{}
	waitChan := make(chan int)
	wg.Add(1)
	wgAll.Add(1)
	go func() {
		for i := 0; i < iterSize; i++ {
			peerID := peer.ID(strconv.Itoa(i))
			peerMeta := PeerMeta{ID: peerID}
			target.insertPeer(peerID, newRemotePeer(peerMeta, target, mockActorServ, logger, nil, nil,nil))
			if i == (iterSize >> 2) {
				wg.Done()
			}
		}
		wgAll.Done()
	}()

	cnt := 0
	go func() {
		wg.Wait()
		for _ = range target.GetPeers() {
			cnt++
		}
		assert.True(t, cnt > (iterSize>>2))
		waitChan <- 0
	}()

	<-waitChan

	wgAll.Wait()
	assert.True(t, iterSize == len(target.GetPeers()))
}

func TestPeerManager_GetPeerAddresses(t *testing.T) {
	peersLen := 3
	samplePeers := make([]*remotePeerImpl, peersLen)
	samplePeers[0] = &remotePeerImpl{meta:PeerMeta{ID:dummyPeerID}}
	samplePeers[1] = &remotePeerImpl{meta:PeerMeta{ID:dummyPeerID2}}
	samplePeers[2] = &remotePeerImpl{meta:PeerMeta{ID:dummyPeerID3}}
	tests := []struct {
		name string
	}{
		// TODO: test cases
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pm := &peerManager{remotePeers:make(map[peer.ID]*remotePeerImpl)}
			for _, peer := range samplePeers {
				pm.remotePeers[peer.ID()] = peer
			}

			actPeers, actBklNotices, actStates := pm.GetPeerAddresses()
			assert.Equal(t, peersLen, len(actPeers))
			assert.Equal(t, peersLen, len(actBklNotices))
			assert.Equal(t, peersLen, len(actStates))
		})
	}
}

func TestPeerManager_init(t *testing.T) {
	tConfig := cfg.NewServerContext("", "").GetDefaultConfig().(*cfg.Config)
	defaultCfg := tConfig.P2P
	InitNodeInfo(defaultCfg, logger)
	localIP, _ := externalIP()

	tests := []struct {
		name string
		inCfg *cfg.P2PConfig
		expectProtoAddr string
		expectProtoPort uint32
		expectBindAddr string
		expectBindPort uint32
		expectPanic bool
	}{
		{"TDefault",defaultCfg, localIP.String(), uint32(defaultCfg.NetProtocolPort), localIP.String(), uint32(defaultCfg.NetProtocolPort), false},
		// wrong ProtocolAddress 0.0.0.0
		{"TUnspecifiedAddr",&cfg.P2PConfig{NetProtocolAddr:"0.0.0.0",NetProtocolPort:7846}, localIP.String(), 7846, localIP.String(), uint32(defaultCfg.NetProtocolPort), true},
		// wrong ProtocolAddress
		{"TWrongAddr",&cfg.P2PConfig{NetProtocolAddr:"24558.30.0.0",NetProtocolPort:7846}, localIP.String(), 7846, localIP.String(), 7846, true},
		// bind all address
		{"TBindAll",&cfg.P2PConfig{NetProtocolAddr:"",NetProtocolPort:7846, NPBindAddr:"0.0.0.0"}, localIP.String(), 7846, "0.0.0.0", 7846, false},
		// bind differnt address
		{"TBindDifferAddr",&cfg.P2PConfig{NetProtocolAddr:"",NetProtocolPort:7846, NPBindAddr:"172.21.1.2"}, localIP.String(), 7846, "172.21.1.2", 7846, false},
		// bind different port
		{"TDifferPort",&cfg.P2PConfig{NetProtocolAddr:"",NetProtocolPort:7846, NPBindPort:12345}, localIP.String(), 7846, localIP.String(), 12345, false},
		// bind different address and port
		{"TBindDiffer",&cfg.P2PConfig{NetProtocolAddr:"",NetProtocolPort:7846, NPBindAddr:"172.21.1.2", NPBindPort:12345}, localIP.String(), 7846, "172.21.1.2", 12345, false},
		// TODO: test cases
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.expectPanic {
				defer func() {
					if r := recover(); r != nil {
						fmt.Println(test.name, " expected panic occurred ", r)
					}
				}()
				pm := peerManager{conf:test.inCfg}

				pm.init()
			}
		})
	}
}
