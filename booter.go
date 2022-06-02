// Copyright 2022 Guan Jianchang. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package basics

import (
	"fmt"

	"github.com/yxlib/odb"
	"github.com/yxlib/yx"
)

type BootCfg struct {
	BaseCfgPath    string
	P2psrvCfgPath  string
	HttpsrvCfgPath string
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

func (b *Booter) Start(srv Server, cfg *SrvCfg, bootCfg *BootCfg, buildCompleteCb func() error) {
	if srv == nil {
		fmt.Println("server is nil")
		return
	}

	ServInst = srv

	// log
	err := yx.LoadJsonConf(cfg, bootCfg.BaseCfgPath, bootCfg.CfgDecodeCb)
	if err != nil {
		fmt.Println("load log config err: ", err)
		return
	}

	yx.ConfigLogger(cfg.Log)
	yx.StartLogger()
	defer yx.StopLogger()

	b.boot(srv, cfg, bootCfg, buildCompleteCb)
}

func (b *Booter) boot(srv Server, cfg *SrvCfg, bootCfg *BootCfg, buildCompleteCb func() error) {
	var err error = nil
	defer b.ec.Catch("boot", &err)

	// build server
	err = b.buildSrv(srv, cfg, bootCfg)
	if err != nil {
		return
	}

	if buildCompleteCb != nil {
		err = buildCompleteCb()
		if err != nil {
			return
		}
	}

	defer srv.Close()

	// data center
	if cfg.Db != nil {
		srv.StartDataCenter(cfg.Db)
	}

	// http
	if cfg.HttpSrv != nil {
		if cfg.P2pSrv != nil {
			b.logger.I("################ " + cfg.Name + " Server Start ################")
			srv.ListenHttp(cfg.HttpSrv)
			b.logger.I("################ " + cfg.Name + " Server Stop ################")
			return
		}

		go srv.ListenHttp(cfg.HttpSrv)
	}

	// socket and websocket
	b.logger.I("################ " + cfg.Name + " Server Start ################")
	srv.ListenConn(cfg.P2pSrv)
	b.logger.I("################ " + cfg.Name + " Server Stop ################")
}

func (b *Booter) buildSrv(srv Server, cfg *SrvCfg, bootCfg *BootCfg) error {
	var err error = nil
	defer b.ec.DeferThrow("buildSrv", &err)

	if bootCfg.P2psrvCfgPath != "" {
		cfg.P2pSrv = &P2pSrvCfg{}
		err = yx.LoadJsonConf(cfg.P2pSrv, bootCfg.P2psrvCfgPath, bootCfg.CfgDecodeCb)
		if err != nil {
			return err
		}
	}

	if bootCfg.HttpsrvCfgPath != "" {
		cfg.HttpSrv = &HttpSrvCfg{}
		err = yx.LoadJsonConf(cfg.HttpSrv, bootCfg.HttpsrvCfgPath, bootCfg.CfgDecodeCb)
		if err != nil {
			return err
		}
	}

	if bootCfg.DbCfgPath != "" {
		cfg.Db = &odb.Config{}
		err = yx.LoadJsonConf(cfg.Db, bootCfg.DbCfgPath, bootCfg.CfgDecodeCb)
		if err != nil {
			return err
		}
	}

	err = ServInst.Build(cfg)
	return nil
}
