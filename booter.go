// Copyright 2022 Guan Jianchang. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goframe

import (
	"errors"
	"fmt"

	"github.com/yxlib/odb"
	"github.com/yxlib/yx"
)

type BootCfg struct {
	BaseCfgPath    string
	P2pConnCfgPath string
	P2pSrvCfgPath  string
	HttpSrvCfgPath string
	RpcSrvCfgPath  string
	DbCfgPath      string
	CfgDecodeCb    func(data []byte) ([]byte, error)
}

type Booter struct {
	logger *yx.Logger
	ec     *yx.ErrCatcher
}

func NewBooter() *Booter {
	return &Booter{
		logger: yx.NewLogger("Booter"),
		ec:     yx.NewErrCatcher("Booter"),
	}
}

func (b *Booter) Boot(srv Server, cfg *SrvCfg, bootCfg *BootCfg) error {
	var err error = nil

	// check params
	if srv == nil || cfg == nil || bootCfg == nil || len(bootCfg.BaseCfgPath) == 0 {
		err = errors.New("Booter start params error")
		fmt.Println(err)
		return err
	}

	SrvInst = srv

	// init logger
	err = yx.LoadJsonConf(cfg, bootCfg.BaseCfgPath, bootCfg.CfgDecodeCb)
	if err != nil {
		fmt.Println("load log config err: ", err)
		return err
	}

	yx.ConfigLogger(cfg.Log)
	yx.StartLogger()
	defer yx.StopLogger()

	defer b.ec.Catch("Boot", &err)

	// load config
	err = b.loadCfg(cfg, bootCfg)
	if err != nil {
		return err
	}

	// start
	err = b.start(srv, cfg)
	return err
}

func (b *Booter) loadCfg(srvCfg *SrvCfg, bootCfg *BootCfg) error {
	var err error = nil
	defer b.ec.DeferThrow("loadCfg", &err)

	if len(bootCfg.P2pConnCfgPath) > 0 {
		cfg := &P2pConnSrvCfg{}
		err = yx.LoadJsonConf(cfg, bootCfg.P2pConnCfgPath, bootCfg.CfgDecodeCb)
		if err != nil {
			return err
		}

		srvCfg.P2pConnSrv = cfg
	}

	if len(bootCfg.RpcSrvCfgPath) > 0 {
		cfg := &RpcSrvCfg{}
		err = yx.LoadJsonConf(cfg, bootCfg.RpcSrvCfgPath, bootCfg.CfgDecodeCb)
		if err != nil {
			return err
		}

		srvCfg.RpcSrv = cfg
	}

	if len(bootCfg.P2pSrvCfgPath) > 0 {
		cfg := &P2pSrvCfg{}
		err = yx.LoadJsonConf(cfg, bootCfg.P2pSrvCfgPath, bootCfg.CfgDecodeCb)
		if err != nil {
			return err
		}

		srvCfg.P2pSrv = cfg
	}

	if len(bootCfg.HttpSrvCfgPath) > 0 {
		cfg := &HttpSrvCfg{}
		err = yx.LoadJsonConf(cfg, bootCfg.HttpSrvCfgPath, bootCfg.CfgDecodeCb)
		if err != nil {
			return err
		}

		srvCfg.HttpSrv = cfg
	}

	if len(bootCfg.DbCfgPath) > 0 {
		cfg := &odb.Config{}
		err = yx.LoadJsonConf(cfg, bootCfg.DbCfgPath, bootCfg.CfgDecodeCb)
		if err != nil {
			return err
		}

		srvCfg.Db = cfg
	}

	return nil
}

func (b *Booter) start(srv Server, cfg *SrvCfg) error {
	var err error = nil
	defer b.ec.DeferThrow("start", &err)

	b.logger.I("Server Init...")

	// build
	err = srv.Build(cfg)
	if err != nil {
		return err
	}

	// start
	srv.Start()
	defer srv.Stop()

	// register
	err = srv.Register()
	if err != nil {
		return err
	}

	// listen
	name := srv.GetName()
	b.logger.I("################ " + name + " Server Start ################")

	err = srv.Listen()

	b.logger.I("################ " + name + " Server Stop ################")

	return err
}

// func (b *Booter) Start(srv Server, cfg *SrvCfg, bootCfg *BootCfg, buildCompleteCb func() error) {
// 	if srv == nil {
// 		fmt.Println("server is nil")
// 		return
// 	}

// 	ServInst = srv

// 	// log
// 	err := yx.LoadJsonConf(cfg, bootCfg.BaseCfgPath, bootCfg.CfgDecodeCb)
// 	if err != nil {
// 		fmt.Println("load log config err: ", err)
// 		return
// 	}

// 	yx.ConfigLogger(cfg.Log)
// 	yx.StartLogger()
// 	defer yx.StopLogger()

// 	b.boot(srv, cfg, bootCfg, buildCompleteCb)
// }

// func (b *Booter) boot(srv Server, cfg *SrvCfg, bootCfg *BootCfg, buildCompleteCb func() error) {
// 	var err error = nil
// 	defer b.ec.Catch("boot", &err)

// 	// build server
// 	err = b.buildSrv(srv, cfg, bootCfg)
// 	if err != nil {
// 		return
// 	}

// 	if buildCompleteCb != nil {
// 		err = buildCompleteCb()
// 		if err != nil {
// 			return
// 		}
// 	}

// 	defer srv.Close()

// 	// data center
// 	if cfg.Db != nil {
// 		srv.StartDataCenter(cfg.Db)
// 	}

// 	// http
// 	if cfg.HttpSrv != nil {
// 		if cfg.P2pSrv != nil {
// 			b.logger.I("################ " + cfg.Name + " Server Start ################")
// 			srv.ListenHttp(cfg.HttpSrv)
// 			b.logger.I("################ " + cfg.Name + " Server Stop ################")
// 			return
// 		}

// 		go srv.ListenHttp(cfg.HttpSrv)
// 	}

// 	// socket and websocket
// 	b.logger.I("################ " + cfg.Name + " Server Start ################")
// 	srv.ListenConn(cfg.P2pSrv)
// 	b.logger.I("################ " + cfg.Name + " Server Stop ################")
// }

// func (b *Booter) buildSrv(srv Server, cfg *SrvCfg, bootCfg *BootCfg) error {
// 	var err error = nil
// 	defer b.ec.DeferThrow("buildSrv", &err)

// 	if bootCfg.P2psrvCfgPath != "" {
// 		cfg.P2pSrv = &P2pConnCfg{}
// 		err = yx.LoadJsonConf(cfg.P2pSrv, bootCfg.P2psrvCfgPath, bootCfg.CfgDecodeCb)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	if bootCfg.HttpsrvCfgPath != "" {
// 		cfg.HttpSrv = &HttpSrvCfg{}
// 		err = yx.LoadJsonConf(cfg.HttpSrv, bootCfg.HttpsrvCfgPath, bootCfg.CfgDecodeCb)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	if bootCfg.DbCfgPath != "" {
// 		cfg.Db = &odb.Config{}
// 		err = yx.LoadJsonConf(cfg.Db, bootCfg.DbCfgPath, bootCfg.CfgDecodeCb)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	err = ServInst.Build(cfg)
// 	return nil
// }
