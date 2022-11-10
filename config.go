// Copyright 2022 Guan Jianchang. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goframe

import (
	"github.com/yxlib/httpsrv"
	"github.com/yxlib/odb"
	"github.com/yxlib/p2pnet"
	"github.com/yxlib/server"
	"github.com/yxlib/yx"
)

const (
	INTER_TYPE_PROTO = iota + 1
	INTER_TYPE_JSON
)

type ShutdownCfg struct {
	File      string `json:"file"`
	CheckIntv int64  `json:"check_intv_sec"`
}

type P2pConnCliCfg struct {
	IsWsCli       bool   `json:"is_ws_cli"`
	HeaderFactory string `json:"header_factory"`
	MaxReadQue    uint32 `json:"max_read_queue"`
	MaxWriteQue   uint32 `json:"max_write_queue"`
}

type P2pConnSrvCfg struct {
	Sock          *p2pnet.SocketConfig  `json:"sock"`
	Websock       *p2pnet.WebSockConfig `json:"ws"`
	HeaderFactory string                `json:"header_factory"`
	// NetListeners  []string              `json:"net_listener"`
	// SrvNet           string                `json:"srv_net"`
	// ProtoInterceptor string                `json:"proto_interceptor"`
	// Server           *server.Config        `json:"server"`
}

type RegWatchSrvCfg struct {
	SrvType uint32 `json:"srv_type"`
	Data    string `json:"data"`
}

type RegWatchDataCfg struct {
	Key  string `json:"key"`
	Data string `json:"data"`
}

type RegCfg struct {
	RegCenterImpl string `json:"reg_center"`
	RegNet        string `json:"reg_net"`
	PeerType      uint32 `json:"peer_type"`
	PeerNo        uint32 `json:"peer_no"`
	Network       string `json:"network"`
	Address       string `json:"address"`
	Port          uint16 `json:"port"`
	Timeout       uint32 `json:"timeout"`
	MaxReadQue    uint32 `json:"max_read_queue"`
	MaxWriteQue   uint32 `json:"max_write_queue"`

	WatchSrvTypes []*RegWatchSrvCfg  `json:"watch_srv"`
	WatchDataKeys []*RegWatchDataCfg `json:"watch_data"`
}

// type RpcSrvCfg struct {
// 	// MaxReadQue   uint32       `json:"max_read_queue"`
// 	IsUseSrvConn bool           `json:"is_use_srv_conn"`
// 	SrvNet       string         `json:"srv_net"`
// 	InterType    int            `json:"inter_type"` // 0 none, 1 pb, 2 json
// 	RpcSrv       *server.Config `json:"server"`
// }

type P2pSrvCfg struct {
	SrvNet    string         `json:"srv_net"`
	InterType int            `json:"inter_type"` // 0 none, 1 pb, 2 json
	Server    *server.Config `json:"server"`
}

type HttpSrvCfg struct {
	Reader    string          `json:"reader"`
	Writer    string          `json:"writer"`
	InterType int             `json:"inter_type"` // 0 none, 1 pb, 2 json
	Http      *httpsrv.Config `json:"http"`
}

type SrvBuildCfg struct {
	PeerType    uint32         `json:"peer_type"`
	PeerNo      uint32         `json:"peer_no"`
	Name        string         `json:"name"`
	TimeZone    int32          `json:"time_zone"`
	IsDebugMode bool           `json:"debug_mode"`
	Shutdown    *ShutdownCfg   `json:"shutdown"`
	Log         *yx.LogConf    `json:"log"`
	Reg         *RegCfg        `json:"reg"`
	P2pConnCli  *P2pConnCliCfg `json:"p2p_cli"`
	P2pConnSrv  *P2pConnSrvCfg
	RpcSrv      *P2pSrvCfg
	P2pSrv      *P2pSrvCfg
	HttpSrv     *HttpSrvCfg
	Db          *odb.Config
}

func NewSrvBuildCfg() *SrvBuildCfg {
	return &SrvBuildCfg{
		// P2pConn: &P2pConnCfg{},
		// P2pSrv:  &P2pSrvCfg{},
		// HttpSrv: &HttpSrvCfg{},
		// RpcSrv:  &rpc.SrvConf{},
		// Db:      &odb.Config{},
	}
}

type SrvCfg interface {
	GetSrvBuildCfg() *SrvBuildCfg
}
