// Code generated by mockery v1.0.0. DO NOT EDIT.
package p2p

import mock "github.com/stretchr/testify/mock"
import peer "github.com/libp2p/go-libp2p-peer"
import types "github.com/aergoio/aergo/types"

// MockRemotePeer is an autogenerated mock type for the RemotePeer type
type MockRemotePeer struct {
	mock.Mock
}

// consumeRequest provides a mock function with given fields: msgID
func (_m *MockRemotePeer) consumeRequest(msgID MsgID) {
	_m.Called(msgID)
}

// pushTxsNotice provides a mock function with given fields: txHashes
func (_m *MockRemotePeer) pushTxsNotice(txHashes []TxHash) {
	_m.Called(txHashes)
}

// runPeer provides a mock function with given fields:
func (_m *MockRemotePeer) runPeer() {
	_m.Called()
}

// sendMessage provides a mock function with given fields: msg
func (_m *MockRemotePeer) sendMessage(msg msgOrder) {
	_m.Called(msg)
}

// stop provides a mock function with given fields:
func (_m *MockRemotePeer) stop() {
	_m.Called()
}

// updateBlkCache provides a mock function with given fields: hash
func (_m *MockRemotePeer) updateBlkCache(hash BlkHash) bool {
	ret := _m.Called(hash)

	var r0 bool
	if rf, ok := ret.Get(0).(func(BlkHash) bool); ok {
		r0 = rf(hash)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// updateTxCache provides a mock function with given fields: hashes
func (_m *MockRemotePeer) updateTxCache(hashes []TxHash) []TxHash {
	ret := _m.Called(hashes)

	var r0 []TxHash
	if rf, ok := ret.Get(0).(func([]TxHash) []TxHash); ok {
		r0 = rf(hashes)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]TxHash)
		}
	}

	return r0
}

// ID provides a mock function with given fields:
func (_m *MockRemotePeer) ID() peer.ID {
	ret := _m.Called()

	var r0 peer.ID
	if rf, ok := ret.Get(0).(func() peer.ID); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(peer.ID)
	}

	return r0
}

// MF provides a mock function with given fields:
func (_m *MockRemotePeer) MF() moFactory {
	ret := _m.Called()

	var r0 moFactory
	if rf, ok := ret.Get(0).(func() moFactory); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(moFactory)
		}
	}

	return r0
}

// Meta provides a mock function with given fields:
func (_m *MockRemotePeer) Meta() PeerMeta {
	ret := _m.Called()

	var r0 PeerMeta
	if rf, ok := ret.Get(0).(func() PeerMeta); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(PeerMeta)
	}

	return r0
}

// State provides a mock function with given fields:
func (_m *MockRemotePeer) State() types.PeerState {
	ret := _m.Called()

	var r0 types.PeerState
	if rf, ok := ret.Get(0).(func() types.PeerState); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(types.PeerState)
	}

	return r0
}