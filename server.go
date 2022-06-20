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
	srv    server.Server
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

func (s *BaseServer) GetSrv() server.Server {
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

// func (s *BaseServer) HandleCloseRpcPeer(peerType uint32, peerNo uint32) {
// 	// TODO
// }

// func (s *BaseServer) GetRpcHeaderFactory(mark string) p2pnet.PackHeaderFactory {
// 	// TODO
// 	return nil
// }

// func (s *BaseServer) GetRpcPeerMgr(mark string) p2pnet.PeerMgr {
// 	// TODO
// 	return nil
// }

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
	// TODO
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

// type Server interface {
// 	RegisterObject(obj interface{}, newFunc func() interface{})
// 	Build(cfg *SrvCfg) error
// 	StartDataCenter(cfg *odb.Config)
// 	ListenConn(cfg *P2pConnCfg) error
// 	ListenHttp(cfg *HttpSrvCfg) error
// 	Close()
// 	GetPeerMgr() (p2pnet.PeerMgr, bool)
// 	GetP2pSrv() (*server.BaseServer, bool)
// 	GetHttpSrv() (*httpsrv.Server, bool)
// 	GetPackHeaderFactory() (p2pnet.PackHeaderFactory, bool)
// 	GetDataCenter() (*odb.DataCenter, bool)
// 	HandleP2pPack(pack *p2pnet.Pack, recvPeerType uint32, recvPeerNo uint32) error
// }

// var ServInst Server = nil

// type BaseServer struct {
// 	sock          *p2pnet.SockServ
// 	ws            *p2pnet.WebSockServ
// 	srv           *server.BaseServer
// 	http          *httpsrv.Server
// 	headerFactory p2pnet.PackHeaderFactory
// 	dc            *odb.DataCenter
// 	objFactory    *yx.ObjectFactory
// }

// func NewBaseServer() *BaseServer {
// 	return &BaseServer{
// 		sock:          nil,
// 		ws:            nil,
// 		srv:           nil,
// 		http:          nil,
// 		headerFactory: nil,
// 		dc:            nil,
// 		objFactory:    yx.NewObjectFactory(),
// 	}
// }

// func (s *BaseServer) RegisterObject(obj interface{}, newFunc func() interface{}) {
// 	s.objFactory.RegisterObject(obj, newFunc, 0)
// }

// func (s *BaseServer) Build(cfg *SrvCfg) error {
// 	// p2p server
// 	if cfg.P2pSrv != nil {
// 		err := s.buildP2pSrv(cfg)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	// http server
// 	if cfg.HttpSrv != nil {
// 		err := s.buildHttpSrv(cfg)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	// db
// 	if cfg.Db != nil {
// 		s.dc = odb.NewDataCenter()
// 		odb.Builder.Build(s.dc, cfg.Db)
// 	}

// 	return nil
// }

// func (s *BaseServer) StartDataCenter(cfg *odb.Config) {
// 	if s.dc != nil {
// 		s.dc.Start(time.Minute*time.Duration(cfg.SaveIntv), time.Minute*time.Duration(cfg.ClearIntv))
// 	}
// }

// func (s *BaseServer) ListenConn(cfg *P2pConnCfg) error {
// 	if s.srv != nil {
// 		go s.srv.Start()
// 	}

// 	var peerMgr p2pnet.PeerMgr = nil
// 	sockAddr := ""
// 	wsAddr := ""
// 	if s.sock != nil {
// 		peerMgr = s.sock.GetPeerMgr()
// 		sockAddr = ":" + strconv.FormatUint(uint64(cfg.Sock.Port), 10)
// 	} else if s.ws != nil {
// 		peerMgr = s.sock.GetPeerMgr()
// 		wsAddr = ":" + strconv.FormatUint(uint64(cfg.Websock.Port), 10)
// 	} else {
// 		return errors.New("not init conn")
// 	}

// 	go peerMgr.Start()

// 	if s.ws == nil {
// 		return s.sock.Listen(cfg.Sock.Network, sockAddr)
// 	}

// 	go s.sock.Listen(cfg.Sock.Network, sockAddr)
// 	return s.ws.Listen(cfg.Websock.Network, wsAddr)
// }

// func (s *BaseServer) ListenHttp(cfg *HttpSrvCfg) error {
// 	if s.http == nil {
// 		return errors.New("not init http")
// 	}

// 	addr := ":" + strconv.FormatUint(uint64(cfg.Http.Port), 10)
// 	return s.http.Listen(addr)
// }

// func (s *BaseServer) Close() {
// 	if s.sock != nil {
// 		s.sock.Close()
// 	}

// 	if s.ws != nil {
// 		s.ws.Close()
// 	}

// 	peerMgr, ok := s.GetPeerMgr()
// 	if ok {
// 		peerMgr.Stop()
// 	}

// 	if s.srv != nil {
// 		s.srv.Stop()
// 	}

// 	if s.dc != nil {
// 		s.dc.Stop()
// 		s.dc.CloseAllDbDriver()
// 		s.dc.CloseAllCacheDriver()
// 	}
// }

// func (s *BaseServer) GetPeerMgr() (p2pnet.PeerMgr, bool) {
// 	if s.sock != nil {
// 		return s.sock.GetPeerMgr(), true
// 	}

// 	if s.ws != nil {
// 		return s.ws.GetPeerMgr(), true
// 	}

// 	return nil, false
// }

// func (s *BaseServer) GetP2pSrv() (*server.BaseServer, bool) {
// 	if s.srv != nil {
// 		return s.srv, true
// 	}

// 	return nil, false
// }

// func (s *BaseServer) GetHttpSrv() (*httpsrv.Server, bool) {
// 	if s.http != nil {
// 		return s.http, true
// 	}

// 	return nil, false
// }

// func (s *BaseServer) GetPackHeaderFactory() (p2pnet.PackHeaderFactory, bool) {
// 	if s.headerFactory != nil {
// 		return s.headerFactory, true
// 	}

// 	return nil, false
// }

// func (s *BaseServer) GetDataCenter() (*odb.DataCenter, bool) {
// 	if s.dc != nil {
// 		return s.dc, true
// 	}

// 	return nil, false
// }

// func (s *BaseServer) HandleP2pPack(pack *p2pnet.Pack, recvPeerType uint32, recvPeerNo uint32) error {
// 	srvNet := s.srv.GetNet().(P2pSrvNet)
// 	return srvNet.HandleP2pPack(pack, recvPeerType, recvPeerNo)
// }

// func (s *BaseServer) buildP2pSrv(cfg *SrvCfg) error {
// 	// header factory
// 	err := s.buildHeaderFactory(cfg)
// 	if err != nil {
// 		return err
// 	}

// 	// peer mgr
// 	var peerMgr *p2pnet.BasePeerMgr = nil
// 	if cfg.P2pSrv.Sock != nil || cfg.P2pSrv.Websock != nil {
// 		peerMgr = p2pnet.NewBasePeerMgr(cfg.PeerType, cfg.PeerNo)

// 		if cfg.P2pSrv.NetListener != "" {
// 			l, err := s.buildNetListener(cfg)
// 			if err != nil {
// 				return err
// 			}

// 			peerMgr.AddListener(l)
// 		}
// 	}

// 	// sock
// 	if cfg.P2pSrv.Sock != nil {
// 		sockCfg := cfg.P2pSrv.Sock
// 		s.sock = p2pnet.NewSockServ(peerMgr, sockCfg.IpConnCntLimit, s.headerFactory, sockCfg.MaxReadQue, sockCfg.MaxWriteQue)
// 		s.sock.SetAcceptBindPeerType(sockCfg.BindPeerType, sockCfg.MinPeerNo, sockCfg.MaxPeerNo)
// 	}

// 	// websocket
// 	if cfg.P2pSrv.Websock != nil {
// 		wsCfg := cfg.P2pSrv.Websock
// 		s.ws = p2pnet.NewWebSockServ(peerMgr, wsCfg.IpConnCntLimit, s.headerFactory, wsCfg.MaxReadQue, wsCfg.MaxWriteQue)
// 		s.ws.Init(wsCfg.Pattern, nil)
// 		s.ws.SetAcceptBindPeerType(wsCfg.BindPeerType, wsCfg.MinPeerNo, wsCfg.MaxPeerNo)
// 	}

// 	// server
// 	if cfg.P2pSrv.Server != nil {
// 		srvNet, err := s.buildSrvNet(cfg)
// 		if err != nil {
// 			return err
// 		}

// 		s.srv = server.NewBaseServer(cfg.Name, srvNet)
// 		server.Builder.Build(s.srv, cfg.P2pSrv.Server)

// 		if cfg.P2pSrv.ProtoInterceptor != "" {
// 			interceptor, err := s.buildProtoInterceptor(cfg.P2pSrv.ProtoInterceptor)
// 			if err != nil {
// 				return err
// 			}

// 			s.srv.AddGlobalInterceptor(interceptor)
// 		}

// 		s.srv.SetDebugMode(cfg.IsDebugMode)
// 	}

// 	return nil
// }

// func (s *BaseServer) buildHttpSrv(cfg *SrvCfg) error {
// 	r, err := s.buildHttpReader(cfg)
// 	if err != nil {
// 		return err
// 	}

// 	w, err := s.buildHttpWriter(cfg)
// 	if err != nil {
// 		return err
// 	}

// 	s.http = httpsrv.NewServer(r, w, cfg.HttpSrv.Http)
// 	httpsrv.Builder.Build(s.http, cfg.HttpSrv.Http)

// 	if cfg.HttpSrv.ProtoInterceptor != "" {
// 		interceptor, err := s.buildProtoInterceptor(cfg.HttpSrv.ProtoInterceptor)
// 		if err != nil {
// 			return err
// 		}

// 		s.http.AddGlobalInterceptor(interceptor)
// 	}

// 	s.http.SetDebugMode(cfg.IsDebugMode)
// 	return nil
// }

// func (s *BaseServer) buildHeaderFactory(cfg *SrvCfg) error {
// 	obj, err := s.objFactory.CreateObject(cfg.P2pSrv.HeaderFactory)
// 	if err != nil {
// 		return err
// 	}

// 	headerFactory, ok := obj.(p2pnet.PackHeaderFactory)
// 	if !ok {
// 		return errors.New("HeaderFactory register type error")
// 	}

// 	s.headerFactory = headerFactory
// 	return nil
// }

// func (s *BaseServer) buildNetListener(cfg *SrvCfg) (p2pnet.P2pNetListener, error) {
// 	obj, err := s.objFactory.CreateObject(cfg.P2pSrv.NetListener)
// 	if err != nil {
// 		return nil, err
// 	}

// 	l, ok := obj.(p2pnet.P2pNetListener)
// 	if !ok {
// 		return nil, errors.New("p2pnet.P2pNetListener register type error")
// 	}

// 	return l, nil
// }

// func (s *BaseServer) buildSrvNet(cfg *SrvCfg) (P2pSrvNet, error) {
// 	obj, err := s.objFactory.CreateObject(cfg.P2pSrv.SrvNet)
// 	if err != nil {
// 		return nil, err
// 	}

// 	srvNet, ok := obj.(P2pSrvNet)
// 	if !ok {
// 		return nil, errors.New("SrvNet register type error")
// 	}

// 	return srvNet, nil
// }

// func (s *BaseServer) buildHttpReader(cfg *SrvCfg) (httpsrv.Reader, error) {
// 	obj, err := s.objFactory.CreateObject(cfg.HttpSrv.Reader)
// 	if err != nil {
// 		return nil, err
// 	}

// 	reader, ok := obj.(httpsrv.Reader)
// 	if !ok {
// 		return nil, errors.New("httpsrv.Reader register type error")
// 	}

// 	return reader, nil
// }

// func (s *BaseServer) buildHttpWriter(cfg *SrvCfg) (httpsrv.Writer, error) {
// 	obj, err := s.objFactory.CreateObject(cfg.HttpSrv.Writer)
// 	if err != nil {
// 		return nil, err
// 	}

// 	writer, ok := obj.(httpsrv.Writer)
// 	if !ok {
// 		return nil, errors.New("httpsrv.Writer register type error")
// 	}

// 	return writer, nil
// }

// func (s *BaseServer) buildProtoInterceptor(name string) (server.Interceptor, error) {
// 	obj, err := s.objFactory.CreateObject(name)
// 	if err != nil {
// 		return nil, err
// 	}

// 	interceptor, ok := obj.(server.Interceptor)
// 	if !ok {
// 		return nil, errors.New("server.Interceptor register type error")
// 	}

// 	return interceptor, nil
// }
