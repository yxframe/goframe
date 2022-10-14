// Copyright 2022 Guan Jianchang. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goframe

import (
	"github.com/yxlib/p2pnet"
	"github.com/yxlib/reg"
	"github.com/yxlib/rpc"
)

//========================
//     RegPushNetListener
//========================
type RegPushNetListener struct {
	*rpc.BaseNet
}

func NewRegPushNetListener(maxReadQue uint32) *RegPushNetListener {
	return &RegPushNetListener{
		BaseNet: rpc.NewBaseNet(maxReadQue),
	}
}

func (l *RegPushNetListener) OnP2pNetOpenPeer(m p2pnet.PeerMgr, peerType uint32, peerNo uint32) {
}

func (l *RegPushNetListener) OnP2pNetClosePeer(m p2pnet.PeerMgr, peerType uint32, peerNo uint32, ipAddr string) {
}

func (l *RegPushNetListener) OnP2pNetReadPack(m p2pnet.PeerMgr, pack *p2pnet.Pack, recvPeerType uint32, recvPeerNo uint32) bool {
	bCanHandle := canHandleRpcPack(l, pack, recvPeerType, recvPeerNo)
	if bCanHandle {
		buff, ok := pack.GetFramesJoin()
		if ok {
			l.AddReadPack(recvPeerType, recvPeerNo, buff)
		}

		pack.Reset()
		m.ReusePack(pack, recvPeerType, recvPeerNo)
	}

	return bCanHandle
}

func (l *RegPushNetListener) OnP2pNetError(m p2pnet.PeerMgr, peerType uint32, peerNo uint32, err error) {
}

//========================
//     RpcNetListener
//========================
type P2pRpcNetMgr interface {
	HandleOpenRpcPeer(peerType uint32, peerNo uint32, service string)
	HandleCloseRpcPeer(peerType uint32, peerNo uint32, service string)
	GetRpcHeaderFactory(peerType uint32, peerNo uint32, service string) p2pnet.PackHeaderFactory
	GetRpcPeerMgr(peerType uint32, peerNo uint32, service string) p2pnet.PeerMgr
}

type RpcNetListener struct {
	*rpc.BaseNet
	mgr P2pRpcNetMgr
}

func NewRpcNetListener(maxReadQue uint32, mgr P2pRpcNetMgr) *RpcNetListener {
	return &RpcNetListener{
		BaseNet: rpc.NewBaseNet(maxReadQue),
		mgr:     mgr,
	}
}

func (l *RpcNetListener) isCurRpcPeer(peerType uint32, peerNo uint32) bool {
	srvPeerType, srvPeerNo := l.GetPeerTypeAndNo()
	return (peerType == srvPeerType && peerNo == srvPeerNo)
}

func (l *RpcNetListener) OnP2pNetOpenPeer(m p2pnet.PeerMgr, peerType uint32, peerNo uint32) {
	if l.IsServerNet() || l.isCurRpcPeer(peerType, peerNo) {
		service := l.GetService()
		l.mgr.HandleOpenRpcPeer(peerType, peerNo, service)
	}
}

func (l *RpcNetListener) OnP2pNetClosePeer(m p2pnet.PeerMgr, peerType uint32, peerNo uint32, ipAddr string) {
	service := l.GetService()
	if l.IsServerNet() {
		l.mgr.HandleCloseRpcPeer(peerType, peerNo, service)
		return
	}

	if !l.isCurRpcPeer(peerType, peerNo) {
		return
	}

	if l.GetService() != reg.REG_SRV {
		m.RemoveTopPriorityListener(l)
	}

	l.mgr.HandleCloseRpcPeer(peerType, peerNo, service)
}

func (l *RpcNetListener) OnP2pNetReadPack(m p2pnet.PeerMgr, pack *p2pnet.Pack, recvPeerType uint32, recvPeerNo uint32) bool {
	bCanHandle := canHandleRpcPack(l, pack, recvPeerType, recvPeerNo)
	if bCanHandle {
		buff, ok := pack.GetFramesJoin()
		if ok {
			l.AddReadPack(recvPeerType, recvPeerNo, buff)
		}

		pack.Reset()
		m.ReusePack(pack, recvPeerType, recvPeerNo)
	}

	return bCanHandle
}

func (l *RpcNetListener) OnP2pNetError(m p2pnet.PeerMgr, peerType uint32, peerNo uint32, err error) {
}

func (l *RpcNetListener) WriteRpcPack(dstPeerType uint32, dstPeerNo uint32, payload ...[]byte) error {
	service := l.GetService()

	factory := l.mgr.GetRpcHeaderFactory(dstPeerType, dstPeerNo, service)
	h := factory.CreateHeader()
	pack := p2pnet.NewPack(h)
	pack.AddFrames(payload)
	pack.UpdatePayloadLen()

	mgr := l.mgr.GetRpcPeerMgr(dstPeerType, dstPeerNo, service)
	return mgr.SendByPeer(pack, dstPeerType, dstPeerNo)
}

func canHandleRpcPack(n rpc.Net, pack *p2pnet.Pack, recvPeerType uint32, recvPeerNo uint32) bool {
	mark := n.GetService()
	if !rpc.CheckRpcMark([]byte(mark), pack.Payload[0]) {
		return false
	}

	if n.IsServerNet() {
		return true
	}

	srvPeerType, srvPeerNo := n.GetPeerTypeAndNo()
	return (recvPeerType == srvPeerType && recvPeerNo == srvPeerNo)
}

//========================
//     TranNetListener
//========================
type TranNetListener struct {
	p2pnet.BaseP2pNetListner
	peerType uint32
	peerNo   uint32
}

func NewTranNetListener(peerType uint32, peerNo uint32) *TranNetListener {
	return &TranNetListener{
		peerType: peerType,
		peerNo:   peerNo,
	}
}

func (l *TranNetListener) OnP2pNetReadPack(m p2pnet.PeerMgr, pack *p2pnet.Pack, recvPeerType uint32, recvPeerNo uint32) bool {
	dstPeerType, dstPeerNo := pack.Header.GetDstPeer()
	if l.peerType == dstPeerType && l.peerNo == dstPeerNo {
		return false
	}

	m.SendByPeer(pack, dstPeerType, dstPeerNo)
	return true
}
