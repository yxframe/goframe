// Copyright 2022 Guan Jianchang. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package basics

import (
	"github.com/yxlib/p2pnet"
	"github.com/yxlib/yx"
)

type NetListener struct {
	logger *yx.Logger
}

func NewNetListener() *NetListener {
	return &NetListener{
		logger: yx.NewLogger("NetListener"),
	}
}

func (l *NetListener) OnP2pNetOpenPeer(m p2pnet.PeerMgr, peerType uint32, peerNo uint32) {
	l.logger.I("OnP2pNetOpenPeer (", peerType, ", ", peerNo, ")")
}

func (l *NetListener) OnP2pNetClosePeer(m p2pnet.PeerMgr, peerType uint32, peerNo uint32, ipAddr string) {
	l.logger.I("OnP2pNetClosePeer (", peerType, ", ", peerNo, ")")
}

func (l *NetListener) OnP2pNetReadPack(m p2pnet.PeerMgr, pack *p2pnet.Pack, recvPeerType uint32, recvPeerNo uint32) bool {
	l.logger.I("OnP2pNetReadPack (", recvPeerType, ", ", recvPeerNo, ")")
	err := ServInst.HandleP2pPack(pack, recvPeerType, recvPeerNo)
	if err != nil {
		return false
	}

	m.ReusePack(pack, recvPeerType, recvPeerNo)
	return true
}

func (l *NetListener) OnP2pNetError(m p2pnet.PeerMgr, peerType uint32, peerNo uint32, err error) {
	l.logger.E("OnP2pNetError (", peerType, ", ", peerNo, "), err: ", err)
}
