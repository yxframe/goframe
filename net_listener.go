// Copyright 2022 Guan Jianchang. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goframe

import (
	"github.com/yxlib/p2pnet"
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
	HandleOpenRpcPeer(mark string, peerType uint32, peerNo uint32)
	HandleCloseRpcPeer(mark string, peerType uint32, peerNo uint32)
	GetRpcHeaderFactory(mark string) p2pnet.PackHeaderFactory
	GetRpcPeerMgr(mark string) p2pnet.PeerMgr
}

type RpcNetListener struct {
	*rpc.BaseNet
	mgr    P2pRpcNetMgr
	bShare bool
}

//
// @param maxReadQue, max size of read queue.
// @param mgr.
// @param bShare, reg and server should be true.
// @return *RpcNetListener.
func NewRpcNetListener(maxReadQue uint32, mgr P2pRpcNetMgr, bShare bool) *RpcNetListener {
	return &RpcNetListener{
		BaseNet: rpc.NewBaseNet(maxReadQue),
		mgr:     mgr,
		bShare:  bShare,
	}
}

func (l *RpcNetListener) isCurRpcPeer(peerType uint32, peerNo uint32) bool {
	srvPeerType, srvPeerNo := l.GetPeerTypeAndNo()
	return (peerType == srvPeerType && peerNo == srvPeerNo)
}

func (l *RpcNetListener) OnP2pNetOpenPeer(m p2pnet.PeerMgr, peerType uint32, peerNo uint32) {
	if l.IsSrvNet() || l.isCurRpcPeer(peerType, peerNo) {
		mark := l.GetReadMark()
		l.mgr.HandleOpenRpcPeer(mark, peerType, peerNo)
	}
}

func (l *RpcNetListener) OnP2pNetClosePeer(m p2pnet.PeerMgr, peerType uint32, peerNo uint32, ipAddr string) {
	if !l.IsSrvNet() && !l.isCurRpcPeer(peerType, peerNo) {
		return
	}

	if !l.IsSrvNet() && !l.bShare {
		m.RemoveListener(l)
	}

	mark := l.GetReadMark()
	l.mgr.HandleCloseRpcPeer(mark, peerType, peerNo)
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

func (l *RpcNetListener) WriteRpcPack(payload []rpc.ByteArray, dstPeerType uint32, dstPeerNo uint32) error {
	mark := l.GetReadMark()

	factory := l.mgr.GetRpcHeaderFactory(mark)
	h := factory.CreateHeader()
	pack := p2pnet.NewPack(h)
	pack.AddFrames(payload)
	pack.UpdatePayloadLen()

	mgr := l.mgr.GetRpcPeerMgr(mark)
	return mgr.SendByPeer(pack, dstPeerType, dstPeerNo)
}

func canHandleRpcPack(n rpc.Net, pack *p2pnet.Pack, recvPeerType uint32, recvPeerNo uint32) bool {
	mark := n.GetReadMark()
	if !rpc.CheckRpcMark([]byte(mark), pack.Payload[0]) {
		return false
	}

	if n.IsSrvNet() {
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
