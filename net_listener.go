// Copyright 2022 Guan Jianchang. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goframe

import (
	"github.com/yxlib/p2pnet"
	"github.com/yxlib/reg"
	"github.com/yxlib/rpc"
	"github.com/yxlib/server"
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
	HandleOpenRpcPeer(peerType uint32, peerNo uint32, mark string)
	HandleCloseRpcPeer(peerType uint32, peerNo uint32, mark string)
	GetRpcHeaderFactory(peerType uint32, peerNo uint32, mark string) p2pnet.PackHeaderFactory
	GetRpcPeerMgr(peerType uint32, peerNo uint32, mark string) p2pnet.PeerMgr
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
	if l.isCurRpcPeer(peerType, peerNo) {
		mark := l.GetMark()
		l.mgr.HandleOpenRpcPeer(peerType, peerNo, mark)
	}
}

func (l *RpcNetListener) OnP2pNetClosePeer(m p2pnet.PeerMgr, peerType uint32, peerNo uint32, ipAddr string) {
	if !l.isCurRpcPeer(peerType, peerNo) {
		return
	}

	mark := l.GetMark()
	if mark != reg.REG_SRV {
		m.RemoveTopPriorityListener(l)
	}

	l.mgr.HandleCloseRpcPeer(peerType, peerNo, mark)
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
	mark := l.GetMark()

	factory := l.mgr.GetRpcHeaderFactory(dstPeerType, dstPeerNo, mark)
	h := factory.CreateHeader()
	pack := p2pnet.NewPack(h)
	pack.AddFrames(payload...)
	pack.UpdatePayloadLen()

	mgr := l.mgr.GetRpcPeerMgr(dstPeerType, dstPeerNo, mark)
	return mgr.SendByPeer(pack, dstPeerType, dstPeerNo)
}

func canHandleRpcPack(n rpc.Net, pack *p2pnet.Pack, recvPeerType uint32, recvPeerNo uint32) bool {
	if len(pack.Payload) == 0 {
		return false
	}

	mark := n.GetMark()
	if !rpc.CheckRpcMark([]byte(mark), pack.Payload[0]) {
		return false
	}

	srvPeerType, srvPeerNo := n.GetPeerTypeAndNo()
	return (recvPeerType == srvPeerType && recvPeerNo == srvPeerNo)
}

type RpcSrvNetListener struct {
	server.BaseNet
	mark string
	mgr  P2pRpcNetMgr
}

func NewRpcSrvNetListener(maxReadQue uint32, mark string, mgr P2pRpcNetMgr) *RpcSrvNetListener {
	return &RpcSrvNetListener{
		BaseNet: *server.NewBaseNet(maxReadQue),
		mark:    mark,
		mgr:     mgr,
	}
}

func (l *RpcSrvNetListener) OnP2pNetOpenPeer(m p2pnet.PeerMgr, peerType uint32, peerNo uint32) {
	l.mgr.HandleOpenRpcPeer(peerType, peerNo, l.mark)
}

func (l *RpcSrvNetListener) OnP2pNetClosePeer(m p2pnet.PeerMgr, peerType uint32, peerNo uint32, ipAddr string) {
	l.mgr.HandleCloseRpcPeer(peerType, peerNo, l.mark)
}

func (l *RpcSrvNetListener) OnP2pNetReadPack(m p2pnet.PeerMgr, pack *p2pnet.Pack, recvPeerType uint32, recvPeerNo uint32) bool {
	if len(pack.Payload) == 0 {
		return false
	}

	if !rpc.CheckRpcMark([]byte(l.mark), pack.Payload[0]) {
		return false
	}

	err := l.handleRpcPack(pack, recvPeerType, recvPeerNo)
	if err != nil {
		return false
	}

	pack.Reset()
	m.ReusePack(pack, recvPeerType, recvPeerNo)
	return true
}

func (l *RpcSrvNetListener) OnP2pNetError(m p2pnet.PeerMgr, peerType uint32, peerNo uint32, err error) {
}

func (l *RpcSrvNetListener) WriteResponse(resp *server.Response) error {
	funcNo := server.GetProtoNo(resp.Mod, resp.Cmd)
	rpcHeader := rpc.NewPackHeader(l.mark, resp.SerialNo, funcNo)
	rpcHeader.Code = resp.Code
	headerData, err := rpcHeader.Marshal()
	if err != nil {
		return err
	}

	peerType := uint32(resp.Dst.PeerType)
	peerNo := uint32(resp.Dst.PeerNo)
	factory := l.mgr.GetRpcHeaderFactory(peerType, peerNo, l.mark)
	h := factory.CreateHeader()
	pack := p2pnet.NewPack(h)
	pack.AddFrames(headerData, resp.Payload)
	pack.UpdatePayloadLen()

	mgr := l.mgr.GetRpcPeerMgr(peerType, peerNo, l.mark)
	return mgr.SendByPeer(pack, peerType, peerNo)
}

func (l *RpcSrvNetListener) handleRpcPack(pack *p2pnet.Pack, recvPeerType uint32, recvPeerNo uint32) error {
	buff, _ := pack.GetFramesJoin()

	h := rpc.NewPackHeader(l.mark, 0, 0)
	err := h.Unmarshal(buff)
	if err != nil {
		return err
	}

	connId := rpc.GetPeerId(recvPeerType, recvPeerNo)
	req := server.NewRequest(uint64(connId))
	req.SerialNo = h.SerialNo
	req.Mod = server.GetMod(h.FuncNo)
	req.Cmd = server.GetCmd(h.FuncNo)
	req.Src = &server.PeerInfo{
		PeerType: uint8(recvPeerType),
		PeerNo:   uint16(recvPeerNo),
	}

	headerLen := h.GetHeaderLen()
	req.Payload = buff[headerLen:]
	l.AddRequest(req)

	return nil
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
