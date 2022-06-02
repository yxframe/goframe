// Copyright 2022 Guan Jianchang. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package basics

import (
	"github.com/yxlib/httpsrv"
	"github.com/yxlib/odb"
	"github.com/yxlib/p2pnet"
	"github.com/yxlib/server"
	"github.com/yxlib/yx"
)

type P2pSrvCfg struct {
	Sock             *p2pnet.SocketConfig  `json:"sock"`
	Websock          *p2pnet.WebSockConfig `json:"ws"`
	HeaderFactory    string                `json:"header_factory"`
	NetListener      string                `json:"net_listener"`
	SrvNet           string                `json:"srv_net"`
	ProtoInterceptor string                `json:"proto_interceptor"`
	Server           *server.Config        `json:"server"`
}

type HttpSrvCfg struct {
	Http             *httpsrv.Config `json:"http"`
	Reader           string          `json:"reader"`
	Writer           string          `json:"writer"`
	ProtoInterceptor string          `json:"proto_interceptor"`
}

type SrvCfg struct {
	PeerType    uint32      `json:"peer_type"`
	PeerNo      uint32      `json:"peer_no"`
	Name        string      `json:"name"`
	IsDebugMode bool        `json:"debug_mode"`
	Log         *yx.LogConf `json:"log"`
	P2pSrv      *P2pSrvCfg
	HttpSrv     *HttpSrvCfg
	Db          *odb.Config
}

func NewSrvCfg() *SrvCfg {
	return &SrvCfg{}
}
