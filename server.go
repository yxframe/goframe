// Copyright 2022 Guan Jianchang. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goframe

import (
	"errors"
	"strconv"
	"time"

	"github.com/yxlib/httpsrv"
	"github.com/yxlib/odb"
	"github.com/yxlib/p2pnet"
	"github.com/yxlib/rpc"
	"github.com/yxlib/server"
	"github.com/yxlib/yx"
)

type Server interface {
	GetName() string
	Build(cfg *SrvBuildCfg) error
	Start()
	Stop()
	Register() error
	Listen() error
	Close()
}

var SrvInst Server = nil

type BaseServer struct {
	name       string
	cfg        *SrvBuildCfg
	p2pConnCli *p2pnet.SimpleClient
	p2pConnSrv p2pnet.Server
	// headerFactory p2pnet.PackHeaderFactory
	srvReg *SrvReg
	rpcSrv rpc.Server
	http   *httpsrv.Server
	srv    *server.BaseServer
	dc     *odb.DataCenter

	objFactory *yx.ObjectFactory
	logger     *yx.Logger
	ec         *yx.ErrCatcher
}

func NewBaseServer() *BaseServer {
	return &BaseServer{
		name:       "",
		cfg:        nil,
		p2pConnCli: nil,
		p2pConnSrv: nil,
		// headerFactory: nil,
		srvReg: nil,
		rpcSrv: nil,
		http:   nil,
		srv:    nil,
		dc:     nil,

		objFactory: yx.NewObjectFactory(),
		logger:     nil,
		ec:         nil,
	}
}

func (s *BaseServer) GetObjFactory() *yx.ObjectFactory {
	return s.objFactory
}

func (s *BaseServer) GetP2pConnCli() *p2pnet.SimpleClient {
	return s.p2pConnCli
}

func (s *BaseServer) GetP2pConnSrv() p2pnet.Server {
	return s.p2pConnSrv
}

func (s *BaseServer) GetSrvReg() *SrvReg {
	return s.srvReg
}

func (s *BaseServer) GetRpcSrv() rpc.Server {
	return s.rpcSrv
}

func (s *BaseServer) GetHttpSrv() *httpsrv.Server {
	return s.http
}

func (s *BaseServer) GetSrv() *server.BaseServer {
	return s.srv
}

func (s *BaseServer) GetDataCenter() *odb.DataCenter {
	return s.dc
}

func (s *BaseServer) GetName() string {
	return s.name
}

func (s *BaseServer) Build(cfg *SrvBuildCfg) error {
	s.name = cfg.Name
	s.cfg = cfg
	tag := "Server(" + s.name + ")"
	s.logger = yx.NewLogger(tag)
	s.ec = yx.NewErrCatcher(tag)

	var err error = nil
	defer s.ec.DeferThrow("Build", &err)

	if cfg.P2pConnCli != nil {
		err = s.buildP2pConnCli(cfg)
		if err != nil {
			return err
		}
	}

	if cfg.P2pConnSrv != nil {
		err = s.buildP2pConnSrv(cfg)
		if err != nil {
			return err
		}
	}

	if cfg.Reg != nil {
		err = s.buildReg(cfg)
		if err != nil {
			return err
		}
	}

	if cfg.RpcSrv != nil {
		err = s.buildRpcSrv(cfg)
		if err != nil {
			return err
		}
	}

	if cfg.P2pSrv != nil {
		err = s.buildP2pSrv(cfg)
		if err != nil {
			return err
		}
	}

	if cfg.HttpSrv != nil {
		err = s.buildHttpSrv(cfg)
		if err != nil {
			return err
		}
	}

	if cfg.Db != nil {
		err = s.buildDb(cfg)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *BaseServer) Start() {
	if s.p2pConnCli != nil {
		peerMgr := s.p2pConnCli.GetPeerMgr()
		go peerMgr.Start()
	}

	if s.p2pConnSrv != nil {
		peerMgr := s.p2pConnSrv.GetPeerMgr()
		go peerMgr.Start()
	}

	if s.rpcSrv != nil {
		go s.rpcSrv.Start()
	}

	if s.srv != nil {
		go s.srv.Start()
	}

	if s.dc != nil {
		s.dc.Start(time.Minute*time.Duration(s.cfg.Db.SaveIntv), time.Minute*time.Duration(s.cfg.Db.ClearIntv))
	}
}

func (s *BaseServer) Stop() {
	if s.p2pConnCli != nil {
		peerMgr := s.p2pConnCli.GetPeerMgr()
		peerMgr.Stop()
	}

	if s.p2pConnSrv != nil {
		peerMgr := s.p2pConnSrv.GetPeerMgr()
		peerMgr.Stop()
	}

	if s.srvReg != nil {
		s.srvReg.Stop()
	}

	if s.rpcSrv != nil {
		s.rpcSrv.Stop()
	}

	if s.srv != nil {
		s.srv.Stop()
	}

	if s.dc != nil {
		s.dc.Stop()
		s.dc.CloseAllDbDriver()
		s.dc.CloseAllCacheDriver()
	}
}

func (s *BaseServer) Register() error {
	if s.srvReg == nil {
		return nil
	}

	var err error = nil
	defer s.ec.DeferThrow("Register", &err)

	err = s.srvReg.Init(s.cfg.Reg)
	if err != nil {
		return err
	}

	err = s.srvReg.Start()
	return err
}

func (s *BaseServer) Listen() error {
	if s.p2pConnSrv != nil {
		network := ""
		port := uint16(0)
		if s.cfg.P2pConnSrv.Websock != nil {
			network = s.cfg.P2pConnSrv.Websock.Network
			port = s.cfg.P2pConnSrv.Websock.Port
		} else if s.cfg.P2pConnSrv.Sock != nil {
			network = s.cfg.P2pConnSrv.Sock.Network
			port = s.cfg.P2pConnSrv.Sock.Port
		}

		addr := ":" + strconv.FormatUint(uint64(port), 10)
		err := s.p2pConnSrv.Listen(network, addr)
		if err != nil {
			return s.ec.Throw("Listen", err)
		}
	} else if s.http != nil {
		addr := ":" + strconv.FormatUint(uint64(s.cfg.HttpSrv.Http.Port), 10)
		err := s.http.Listen(addr)
		if err != nil {
			return s.ec.Throw("Listen", err)
		}
	}

	return nil
}

func (s *BaseServer) Close() {
	if s.p2pConnSrv != nil {
		s.p2pConnSrv.Close()
	}
}

func (s *BaseServer) buildP2pConnCli(srvCfg *SrvBuildCfg) error {
	var err error = nil
	defer s.ec.DeferThrow("buildP2pConnCli", &err)

	cfg := srvCfg.P2pConnCli

	// header factory
	obj, err := s.objFactory.CreateObject(cfg.HeaderFactory)
	if err != nil {
		return err
	}

	headerFactory, ok := obj.(p2pnet.PackHeaderFactory)
	if !ok {
		err = errors.New("refect type is not p2pnet.PackHeaderFactory")
		return err
	}

	// client
	var cli p2pnet.Client = nil
	if cfg.IsWsCli {
		cli = p2pnet.NewWebSockClient()
	} else {
		cli = p2pnet.NewSockClient()
	}

	logNet := p2pnet.NewLogNetListener()
	peerMgr := p2pnet.NewBasePeerMgr(srvCfg.PeerType, srvCfg.PeerNo)
	peerMgr.AddListener(logNet)

	s.p2pConnCli = p2pnet.NewSimpleClient(cli, peerMgr, headerFactory, cfg.MaxReadQue, cfg.MaxWriteQue)
	return nil
}

func (s *BaseServer) buildP2pConnSrv(srvCfg *SrvBuildCfg) error {
	var err error = nil
	defer s.ec.DeferThrow("buildP2pConn", &err)

	// peer mgr
	cfg := srvCfg.P2pConnSrv

	logNet := p2pnet.NewLogNetListener()
	peerMgr := p2pnet.NewBasePeerMgr(srvCfg.PeerType, srvCfg.PeerNo)
	peerMgr.AddListener(logNet)

	// if len(cfg.NetListeners) > 0 {
	// 	for _, name := range cfg.NetListeners {
	// 		obj, err := s.objFactory.CreateObject(name)
	// 		if err != nil {
	// 			return err
	// 		}

	// 		l, ok := obj.(p2pnet.P2pNetListener)
	// 		if !ok {
	// 			err = errors.New("refect type is not p2pnet.P2pNetListener")
	// 			return err
	// 		}

	// 		peerMgr.AddListener(l)
	// 	}
	// }

	// header factory
	obj, err := s.objFactory.CreateObject(cfg.HeaderFactory)
	if err != nil {
		return err
	}

	headerFactory, ok := obj.(p2pnet.PackHeaderFactory)
	if !ok {
		err = errors.New("refect type is not p2pnet.PackHeaderFactory")
		return err
	}

	// s.headerFactory = headerFactory

	// p2p server
	if cfg.Sock != nil {
		s.p2pConnSrv = p2pnet.NewSockServ(peerMgr, cfg.Sock.IpConnCntLimit, headerFactory, cfg.Sock.MaxReadQue, cfg.Sock.MaxWriteQue)
		if cfg.Sock.BindPeerType > 0 {
			s.p2pConnSrv.SetAcceptBindPeerType(cfg.Sock.BindPeerType, cfg.Sock.MinPeerNo, cfg.Sock.MaxPeerNo)
		}
	} else {
		wsSrv := p2pnet.NewWebSockServ(peerMgr, cfg.Websock.IpConnCntLimit, headerFactory, cfg.Websock.MaxReadQue, cfg.Websock.MaxWriteQue)
		wsSrv.Init(cfg.Websock.Pattern, nil)
		if cfg.Websock.BindPeerType > 0 {
			wsSrv.SetAcceptBindPeerType(cfg.Websock.BindPeerType, cfg.Websock.MinPeerNo, cfg.Websock.MaxPeerNo)
		}
		s.p2pConnSrv = wsSrv
	}

	return nil
}

func (s *BaseServer) buildReg(srvCfg *SrvBuildCfg) error {
	var err error = nil
	defer s.ec.DeferThrow("buildReg", &err)

	cfg := srvCfg.Reg
	// server
	obj, err := s.objFactory.CreateObject(cfg.SrvRegImpl)
	if err != nil {
		return err
	}

	impl, ok := obj.(SrvRegImpl)
	if !ok {
		err = errors.New("refect type is not SrvRegImpl")
		return err
	}

	// regNet := NewRpcNetListener(cfg.MaxReadQue, s.rpcNetMgr)
	obj, err = s.objFactory.CreateObject(cfg.RegNet)
	if err != nil {
		return err
	}

	regNet, ok := obj.(*RpcNetListener)
	if !ok {
		err = errors.New("refect type is not RpcNetListener")
		return err
	}

	observerNet := NewRegPushNetListener(cfg.MaxReadQue)

	peerMgr := s.p2pConnCli.GetPeerMgr()
	peerMgr.AddListener(regNet)
	peerMgr.AddListener(observerNet)

	srvReg := NewSrvReg(impl)
	srvReg.SetNets(regNet, observerNet)
	s.srvReg = srvReg
	return nil
}

func (s *BaseServer) buildRpcSrv(srvCfg *SrvBuildCfg) error {
	var err error = nil
	defer s.ec.DeferThrow("buildRpcSrv", &err)

	cfg := srvCfg.RpcSrv

	// server
	obj, err := s.objFactory.CreateObject(cfg.Srv)
	if err != nil {
		return err
	}

	srv, ok := obj.(rpc.Server)
	if !ok {
		err = errors.New("refect type is not rpc.Server")
		return err
	}

	net := srv.GetRpcNet()
	rpcSrvNet, ok := net.(*RpcNetListener)
	if ok {
		var peerMgr p2pnet.PeerMgr = nil
		if cfg.IsUseSrvConn {
			peerMgr = s.p2pConnSrv.GetPeerMgr()
		} else {
			peerMgr = s.p2pConnCli.GetPeerMgr()
		}

		peerMgr.AddListener(rpcSrvNet)
	}

	rpc.Builder.BuildSrv(srv, cfg.RpcSrv)
	srv.SetDebugMode(srvCfg.IsDebugMode)
	s.rpcSrv = srv
	return nil
}

func (s *BaseServer) buildP2pSrv(srvCfg *SrvBuildCfg) error {
	var err error = nil
	defer s.ec.DeferThrow("buildP2pSrv", &err)

	cfg := srvCfg.P2pSrv

	// net
	obj, err := s.objFactory.CreateObject(cfg.SrvNet)
	if err != nil {
		return err
	}

	n, ok := obj.(server.Net)
	if !ok {
		err = errors.New("refect type is not server.Net")
		return err
	}

	// peerMgr := s.p2pConnSrv.GetPeerMgr()
	// peerMgr.AddListener(n)

	// server
	srv := server.NewBaseServer("p2psrv", n)
	if cfg.InterType == INTER_TYPE_JSON {
		srv.AddGlobalInterceptor(&server.JsonInterceptor{})
	} else if cfg.InterType == INTER_TYPE_PROTO {
		srv.AddGlobalInterceptor(&PbInterceptor{})
	}

	srv.SetDebugMode(srvCfg.IsDebugMode)

	server.Builder.Build(srv, cfg.Server)
	s.srv = srv
	return nil
}

func (s *BaseServer) buildHttpSrv(srvCfg *SrvBuildCfg) error {
	var err error = nil
	defer s.ec.DeferThrow("buildHttpSrv", &err)

	cfg := srvCfg.HttpSrv

	// reader
	obj, err := s.objFactory.CreateObject(cfg.Reader)
	if err != nil {
		return err
	}

	r, ok := obj.(httpsrv.Reader)
	if !ok {
		err = errors.New("refect type is not httpsrv.Reader")
		return err
	}

	// writer
	obj, err = s.objFactory.CreateObject(cfg.Writer)
	if err != nil {
		return err
	}

	w, ok := obj.(httpsrv.Writer)
	if !ok {
		err = errors.New("refect type is not httpsrv.Writer")
		return err
	}

	// server
	http := httpsrv.NewServer(r, w, cfg.Http)
	if cfg.InterType == INTER_TYPE_JSON {
		http.AddGlobalInterceptor(&server.JsonInterceptor{})
	} else if cfg.InterType == INTER_TYPE_PROTO {
		http.AddGlobalInterceptor(&PbInterceptor{})
	}

	http.SetDebugMode(srvCfg.IsDebugMode)

	httpsrv.Builder.Build(http, cfg.Http)
	s.http = http
	return nil
}

func (s *BaseServer) buildDb(srvCfg *SrvBuildCfg) error {
	dc := odb.NewDataCenter()
	odb.Builder.Build(dc, srvCfg.Db)
	s.dc = dc
	return nil
}
