// Copyright 2022 Guan Jianchang. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goframe

import (
	"github.com/yxlib/httpsrv"
	"github.com/yxlib/odb"
	"github.com/yxlib/p2pnet"
	"github.com/yxlib/rpc"
	"github.com/yxlib/server"
	"github.com/yxlib/yx"
)

const (
	INTER_TYPE_PROTO = iota + 1
	INTER_TYPE_JSON
)

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

type RegCfg struct {
	SrvRegImpl  string `json:"impl"`
	RegNet      string `json:"reg_net"`
	PeerType    uint32 `json:"peer_type"`
	PeerNo      uint32 `json:"peer_no"`
	Network     string `json:"network"`
	Address     string `json:"address"`
	Port        uint16 `json:"port"`
	Timeout     uint32 `json:"timeout"`
	MaxReadQue  uint32 `json:"max_read_queue"`
	MaxWriteQue uint32 `json:"max_write_queue"`
}

type RpcSrvCfg struct {
	Srv          string       `json:"srv"`
	MaxReadQue   uint32       `json:"max_read_queue"`
	IsUseSrvConn bool         `json:"is_use_srv_conn"`
	RpcSrv       *rpc.SrvConf `json:"rpc"`
}

type P2pSrvCfg struct {
	SrvNet    string         `json:"srv_net"`
	InterType int            `json:"inter_type"`
	Server    *server.Config `json:"server"`
}

type HttpSrvCfg struct {
	Reader    string          `json:"reader"`
	Writer    string          `json:"writer"`
	InterType int             `json:"inter_type"`
	Http      *httpsrv.Config `json:"http"`
}

type SrvBuildCfg struct {
	PeerType    uint32         `json:"peer_type"`
	PeerNo      uint32         `json:"peer_no"`
	Name        string         `json:"name"`
	TimeZone    int32          `json:"time_zone"`
	IsDebugMode bool           `json:"debug_mode"`
	Log         *yx.LogConf    `json:"log"`
	Reg         *RegCfg        `json:"reg"`
	P2pConnCli  *P2pConnCliCfg `json:"p2p_cli"`
	P2pConnSrv  *P2pConnSrvCfg
	RpcSrv      *RpcSrvCfg
	P2pSrv      *P2pSrvCfg
	HttpSrv     *HttpSrvCfg
	Db          *odb.Config

	ShutdownFile      string `json:"shutdown_file"`
	ShutdownCheckIntv int64  `json:"shutdown_check_intv_sec"`
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
