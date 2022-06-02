// Copyright 2022 Guan Jianchang. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package basics

import (
	"errors"
	"strconv"
	"time"

	"github.com/yxlib/httpsrv"
	"github.com/yxlib/odb"
	"github.com/yxlib/p2pnet"
	"github.com/yxlib/server"
	"github.com/yxlib/yx"
)

type Server interface {
	RegisterObject(obj interface{}, newFunc func() interface{})
	Build(cfg *SrvCfg) error
	StartDataCenter(cfg *odb.Config)
	ListenConn(cfg *P2pSrvCfg) error
	ListenHttp(cfg *HttpSrvCfg) error
	Close()
	GetPeerMgr() (p2pnet.PeerMgr, bool)
	GetP2pSrv() (*server.BaseServer, bool)
	GetHttpSrv() (*httpsrv.Server, bool)
	GetPackHeaderFactory() (p2pnet.PackHeaderFactory, bool)
	GetDataCenter() (*odb.DataCenter, bool)
	HandleP2pPack(pack *p2pnet.Pack, recvPeerType uint32, recvPeerNo uint32) error
}

var ServInst Server = nil

type BaseServer struct {
	sock          *p2pnet.SockServ
	ws            *p2pnet.WebSockServ
	srv           *server.BaseServer
	http          *httpsrv.Server
	headerFactory p2pnet.PackHeaderFactory
	dc            *odb.DataCenter
	objFactory    *yx.ObjectFactory
}

func NewBaseServer() *BaseServer {
	return &BaseServer{
		sock:          nil,
		ws:            nil,
		srv:           nil,
		http:          nil,
		headerFactory: nil,
		dc:            nil,
		objFactory:    yx.NewObjectFactory(),
	}
}

func (s *BaseServer) RegisterObject(obj interface{}, newFunc func() interface{}) {
	s.objFactory.RegisterObject(obj, newFunc, 0)
}

func (s *BaseServer) Build(cfg *SrvCfg) error {
	// p2p server
	if cfg.P2pSrv != nil {
		err := s.buildP2pSrv(cfg)
		if err != nil {
			return err
		}
	}

	// http server
	if cfg.HttpSrv != nil {
		err := s.buildHttpSrv(cfg)
		if err != nil {
			return err
		}
	}

	// db
	if cfg.Db != nil {
		s.dc = odb.NewDataCenter()
		odb.Builder.Build(s.dc, cfg.Db)
	}

	return nil
}

func (s *BaseServer) StartDataCenter(cfg *odb.Config) {
	if s.dc != nil {
		s.dc.Start(time.Minute*time.Duration(cfg.SaveIntv), time.Minute*time.Duration(cfg.ClearIntv))
	}
}

func (s *BaseServer) ListenConn(cfg *P2pSrvCfg) error {
	if s.srv != nil {
		go s.srv.Start()
	}

	var peerMgr p2pnet.PeerMgr = nil
	sockAddr := ""
	wsAddr := ""
	if s.sock != nil {
		peerMgr = s.sock.GetPeerMgr()
		sockAddr = ":" + strconv.FormatUint(uint64(cfg.Sock.Port), 10)
	} else if s.ws != nil {
		peerMgr = s.sock.GetPeerMgr()
		wsAddr = ":" + strconv.FormatUint(uint64(cfg.Websock.Port), 10)
	} else {
		return errors.New("not init conn")
	}

	go peerMgr.Start()

	if s.ws == nil {
		return s.sock.Listen(cfg.Sock.Network, sockAddr)
	}

	go s.sock.Listen(cfg.Sock.Network, sockAddr)
	return s.ws.Listen(cfg.Websock.Network, wsAddr)
}

func (s *BaseServer) ListenHttp(cfg *HttpSrvCfg) error {
	if s.http == nil {
		return errors.New("not init http")
	}

	addr := ":" + strconv.FormatUint(uint64(cfg.Http.Port), 10)
	return s.http.Listen(addr)
}

func (s *BaseServer) Close() {
	if s.sock != nil {
		s.sock.Close()
	}

	if s.ws != nil {
		s.ws.Close()
	}

	peerMgr, ok := s.GetPeerMgr()
	if ok {
		peerMgr.Stop()
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

func (s *BaseServer) GetPeerMgr() (p2pnet.PeerMgr, bool) {
	if s.sock != nil {
		return s.sock.GetPeerMgr(), true
	}

	if s.ws != nil {
		return s.ws.GetPeerMgr(), true
	}

	return nil, false
}

func (s *BaseServer) GetP2pSrv() (*server.BaseServer, bool) {
	if s.srv != nil {
		return s.srv, true
	}

	return nil, false
}

func (s *BaseServer) GetHttpSrv() (*httpsrv.Server, bool) {
	if s.http != nil {
		return s.http, true
	}

	return nil, false
}

func (s *BaseServer) GetPackHeaderFactory() (p2pnet.PackHeaderFactory, bool) {
	if s.headerFactory != nil {
		return s.headerFactory, true
	}

	return nil, false
}

func (s *BaseServer) GetDataCenter() (*odb.DataCenter, bool) {
	if s.dc != nil {
		return s.dc, true
	}

	return nil, false
}

func (s *BaseServer) HandleP2pPack(pack *p2pnet.Pack, recvPeerType uint32, recvPeerNo uint32) error {
	srvNet := s.srv.GetSrvNet().(P2pServerNet)
	return srvNet.HandleP2pPack(pack, recvPeerType, recvPeerNo)
}

func (s *BaseServer) buildP2pSrv(cfg *SrvCfg) error {
	// header factory
	err := s.buildHeaderFactory(cfg)
	if err != nil {
		return err
	}

	// peer mgr
	var peerMgr *p2pnet.BasePeerMgr = nil
	if cfg.P2pSrv.Sock != nil || cfg.P2pSrv.Websock != nil {
		peerMgr = p2pnet.NewBasePeerMgr(cfg.PeerType, cfg.PeerNo)

		if cfg.P2pSrv.NetListener != "" {
			l, err := s.buildNetListener(cfg)
			if err != nil {
				return err
			}

			peerMgr.AddListener(l)
		}
	}

	// sock
	if cfg.P2pSrv.Sock != nil {
		sockCfg := cfg.P2pSrv.Sock
		s.sock = p2pnet.NewSockServ(peerMgr, sockCfg.IpConnCntLimit, s.headerFactory, sockCfg.MaxReadQue, sockCfg.MaxWriteQue)
		s.sock.SetAcceptBindPeerType(sockCfg.BindPeerType, sockCfg.MinPeerNo, sockCfg.MaxPeerNo)
	}

	// websocket
	if cfg.P2pSrv.Websock != nil {
		wsCfg := cfg.P2pSrv.Websock
		s.ws = p2pnet.NewWebSockServ(peerMgr, wsCfg.IpConnCntLimit, s.headerFactory, wsCfg.MaxReadQue, wsCfg.MaxWriteQue)
		s.ws.Init(wsCfg.Pattern, nil)
		s.ws.SetAcceptBindPeerType(wsCfg.BindPeerType, wsCfg.MinPeerNo, wsCfg.MaxPeerNo)
	}

	// server
	if cfg.P2pSrv.Server != nil {
		srvNet, err := s.buildSrvNet(cfg)
		if err != nil {
			return err
		}

		s.srv = server.NewBaseServer(cfg.Name, srvNet)
		server.Builder.Build(s.srv, cfg.P2pSrv.Server)

		if cfg.P2pSrv.ProtoInterceptor != "" {
			interceptor, err := s.buildProtoInterceptor(cfg.P2pSrv.ProtoInterceptor)
			if err != nil {
				return err
			}

			s.srv.AddGlobalInterceptor(interceptor)
		}

		s.srv.SetDebugMode(cfg.IsDebugMode)
	}

	return nil
}

func (s *BaseServer) buildHttpSrv(cfg *SrvCfg) error {
	r, err := s.buildHttpReader(cfg)
	if err != nil {
		return err
	}

	w, err := s.buildHttpWriter(cfg)
	if err != nil {
		return err
	}

	s.http = httpsrv.NewServer(r, w, cfg.HttpSrv.Http)
	httpsrv.Builder.Build(s.http, cfg.HttpSrv.Http)

	if cfg.HttpSrv.ProtoInterceptor != "" {
		interceptor, err := s.buildProtoInterceptor(cfg.HttpSrv.ProtoInterceptor)
		if err != nil {
			return err
		}

		s.http.AddGlobalInterceptor(interceptor)
	}

	s.http.SetDebugMode(cfg.IsDebugMode)
	return nil
}

func (s *BaseServer) buildHeaderFactory(cfg *SrvCfg) error {
	obj, err := s.objFactory.CreateObject(cfg.P2pSrv.HeaderFactory)
	if err != nil {
		return err
	}

	headerFactory, ok := obj.(p2pnet.PackHeaderFactory)
	if !ok {
		return errors.New("HeaderFactory register type error")
	}

	s.headerFactory = headerFactory
	return nil
}

func (s *BaseServer) buildNetListener(cfg *SrvCfg) (p2pnet.P2pNetListener, error) {
	obj, err := s.objFactory.CreateObject(cfg.P2pSrv.NetListener)
	if err != nil {
		return nil, err
	}

	l, ok := obj.(p2pnet.P2pNetListener)
	if !ok {
		return nil, errors.New("p2pnet.P2pNetListener register type error")
	}

	return l, nil
}

func (s *BaseServer) buildSrvNet(cfg *SrvCfg) (P2pServerNet, error) {
	obj, err := s.objFactory.CreateObject(cfg.P2pSrv.SrvNet)
	if err != nil {
		return nil, err
	}

	srvNet, ok := obj.(P2pServerNet)
	if !ok {
		return nil, errors.New("SrvNet register type error")
	}

	return srvNet, nil
}

func (s *BaseServer) buildHttpReader(cfg *SrvCfg) (httpsrv.Reader, error) {
	obj, err := s.objFactory.CreateObject(cfg.HttpSrv.Reader)
	if err != nil {
		return nil, err
	}

	reader, ok := obj.(httpsrv.Reader)
	if !ok {
		return nil, errors.New("httpsrv.Reader register type error")
	}

	return reader, nil
}

func (s *BaseServer) buildHttpWriter(cfg *SrvCfg) (httpsrv.Writer, error) {
	obj, err := s.objFactory.CreateObject(cfg.HttpSrv.Writer)
	if err != nil {
		return nil, err
	}

	writer, ok := obj.(httpsrv.Writer)
	if !ok {
		return nil, errors.New("httpsrv.Writer register type error")
	}

	return writer, nil
}

func (s *BaseServer) buildProtoInterceptor(name string) (server.Interceptor, error) {
	obj, err := s.objFactory.CreateObject(name)
	if err != nil {
		return nil, err
	}

	interceptor, ok := obj.(server.Interceptor)
	if !ok {
		return nil, errors.New("server.Interceptor register type error")
	}

	return interceptor, nil
}
