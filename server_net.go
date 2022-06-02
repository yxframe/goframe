// Copyright 2022 Guan Jianchang. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package basics

import (
	"errors"

	"github.com/yxlib/p2pnet"
	"github.com/yxlib/server"
	"github.com/yxlib/yx"
)

var (
	ErrSrvNetReadQueClose = errors.New("read queue closed")
)

type P2pServerNet interface {
	server.ServerNet
	HandleP2pPack(pack *p2pnet.Pack, recvPeerType uint32, recvPeerNo uint32) error
}

//========================
//    BaseP2pServerNet
//========================
type BaseP2pServerNet struct {
	chanRequest chan *server.Request
	logger      *yx.Logger
	ec          *yx.ErrCatcher
}

func NewBaseP2pServerNet(maxReadQue uint32) *BaseP2pServerNet {
	return &BaseP2pServerNet{
		chanRequest: make(chan *server.Request, maxReadQue),
		logger:      yx.NewLogger("BaseP2pServerNet"),
		ec:          yx.NewErrCatcher("BaseP2pServerNet"),
	}
}

// ServerNet
func (n *BaseP2pServerNet) ReadRequest() (*server.Request, error) {
	req, ok := <-n.chanRequest
	if !ok {
		return nil, ErrSrvNetReadQueClose
	}

	return req, nil
}

func (n *BaseP2pServerNet) WriteResponse(resp *server.Response) error {
	return nil
}

func (n *BaseP2pServerNet) Close() {
	close(n.chanRequest)
}

func (n *BaseP2pServerNet) AddRequest(req *server.Request) {
	n.chanRequest <- req
}

func (n *BaseP2pServerNet) HandleP2pPack(pack *p2pnet.Pack, recvPeerType uint32, recvPeerNo uint32) error {
	return nil
}
