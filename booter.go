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
	HttpCfgPath    string
	HttpSrvCfgPath string
	RpcSrvCfgPath  string
	DbCfgPath      string
	CfgDecodeCb    func(data []byte) ([]byte, error)
	BuildSuccCb    func() error
	StartCb        func()
	StopCb         func()
	RegisterSuccCb func() error
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

func (b *Booter) Boot(srv Server, cfg SrvCfg, bootCfg *BootCfg, logPrintFunc func(lv int, logStr string)) error {
	var err error = nil

	// check params
	if srv == nil || cfg == nil || bootCfg == nil || len(bootCfg.BaseCfgPath) == 0 {
		err = errors.New("Booter start params error")
		fmt.Println(err)
		return err
	}

	SrvInst = srv

	// load base config
	err = yx.LoadJsonConf(cfg, bootCfg.BaseCfgPath, bootCfg.CfgDecodeCb)
	if err != nil {
		fmt.Println("load log config err: ", err)
		return err
	}

	// init logger
	buildCfg := cfg.GetSrvBuildCfg()
	yx.ConfigLogger(buildCfg.Log, logPrintFunc)
	yx.StartLogger()
	defer yx.StopLogger()

	defer b.ec.Catch("Boot", &err)

	// load config
	b.logger.I("=====>  Load Config...")

	err = b.loadCfg(buildCfg, bootCfg)
	if err != nil {
		return err
	}

	b.logger.I("=====>  Load Config Success!!")

	// build
	b.logger.I("=====>  Build Server...")

	err = srv.Build(buildCfg)
	if err != nil {
		return err
	}

	if bootCfg.BuildSuccCb != nil {
		err = bootCfg.BuildSuccCb()
		if err != nil {
			return err
		}
	}

	b.logger.I("=====>  Build Server Success!!")

	// start
	bNeedReg := (buildCfg.Reg != nil)
	err = b.start(srv, bNeedReg, bootCfg)
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

	if len(bootCfg.HttpCfgPath) > 0 {
		cfg := &HttpCfg{}
		err = yx.LoadJsonConf(cfg, bootCfg.HttpCfgPath, bootCfg.CfgDecodeCb)
		if err != nil {
			return err
		}

		srvCfg.Http = cfg
	}

	if len(bootCfg.RpcSrvCfgPath) > 0 {
		cfg := &ServerCfg{}
		err = yx.LoadJsonConf(cfg, bootCfg.RpcSrvCfgPath, bootCfg.CfgDecodeCb)
		if err != nil {
			return err
		}

		srvCfg.RpcSrv = cfg
	}

	if len(bootCfg.P2pSrvCfgPath) > 0 {
		cfg := &ServerCfg{}
		err = yx.LoadJsonConf(cfg, bootCfg.P2pSrvCfgPath, bootCfg.CfgDecodeCb)
		if err != nil {
			return err
		}

		srvCfg.P2pSrv = cfg
	}

	if len(bootCfg.HttpSrvCfgPath) > 0 {
		cfg := &ServerCfg{}
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

func (b *Booter) start(srv Server, bNeedReg bool, bootCfg *BootCfg) error {
	var err error = nil
	defer b.ec.DeferThrow("start", &err)

	// start
	b.logger.I("=====>  Start Server...")

	srv.Start()
	if bootCfg.StartCb != nil {
		bootCfg.StartCb()
	}

	defer func() {
		if bootCfg.StopCb != nil {
			bootCfg.StopCb()
		}
	}()

	defer srv.Stop()

	name := srv.GetName()
	b.logger.I("###########################################################")
	b.logger.I("#                " + name + " Server Start")
	b.logger.I("###########################################################")

	// register
	if bNeedReg {
		go b.register(srv, bootCfg.RegisterSuccCb)
	}

	// listen
	err = srv.Listen()

	// stop
	b.logger.I("===========================================================")
	b.logger.I("=                " + name + " Server Stop")
	b.logger.I("===========================================================")

	return err
}

func (b *Booter) register(srv Server, registerSuccCb func() error) {
	var err error = nil
	defer b.ec.Catch("register", &err)

	b.logger.I("=====>  Register Server...")

	err = srv.Register()
	if err != nil {
		srv.Close()
		return
	}

	b.logger.I("=====>  Register Server Success!!")

	if registerSuccCb != nil {
		err = registerSuccCb()
		if err != nil {
			srv.Close()
			return
		}
	}
}
