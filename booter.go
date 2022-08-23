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

func (b *Booter) Boot(srv Server, cfg SrvCfg, bootCfg *BootCfg, buildSuccCb func() error, registerSuccCb func() error) error {
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

	buildCfg := cfg.GetSrvBuildCfg()
	yx.ConfigLogger(buildCfg.Log)
	yx.StartLogger()
	defer yx.StopLogger()

	defer b.ec.Catch("Boot", &err)

	// load config
	err = b.loadCfg(buildCfg, bootCfg)
	if err != nil {
		return err
	}

	// start
	err = b.start(srv, buildCfg, buildSuccCb, registerSuccCb)
	return err
}

func (b *Booter) loadCfg(srvCfg *SrvBuildCfg, bootCfg *BootCfg) error {
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

func (b *Booter) start(srv Server, cfg *SrvBuildCfg, buildSuccCb func() error, registerSuccCb func() error) error {
	var err error = nil
	defer b.ec.DeferThrow("start", &err)

	b.logger.I("Server Init...")

	// build
	err = srv.Build(cfg)
	if err != nil {
		return err
	}

	if buildSuccCb != nil {
		err = buildSuccCb()
		if err != nil {
			return err
		}
	}

	// start
	srv.Start()
	defer srv.Stop()

	// register
	err = srv.Register()
	if err != nil {
		return err
	}

	if registerSuccCb != nil {
		err = registerSuccCb()
		if err != nil {
			return err
		}
	}

	// listen
	name := srv.GetName()
	b.logger.I("###########################################################")
	b.logger.I("#                " + name + " Server Start")
	b.logger.I("###########################################################")

	err = srv.Listen()

	b.logger.I("===========================================================")
	b.logger.I("=                " + name + " Server Stop")
	b.logger.I("===========================================================")

	return err
}
