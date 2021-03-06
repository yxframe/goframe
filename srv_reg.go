// Copyright 2022 Guan Jianchang. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goframe

import (
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/yxlib/p2pnet"
	"github.com/yxlib/reg"
	"github.com/yxlib/yx"
)

var (
	ErrNoDataProcessor = errors.New("no data processor")
)

const (
	CONN_REG_SRV_DELAY = 5 * time.Second
	RESTART_REG_DELAY  = 1 * time.Second
)

//========================================
//                RegInfo
//========================================
type RegInfo interface {
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
}

type RegInfoBase struct {
}

func (i *RegInfoBase) Marshal(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	return data, err
}

func (i *RegInfoBase) Unmarshal(data []byte, v interface{}) error {
	err := json.Unmarshal(data, v)
	return err
}

type RegSrvInfo struct {
	SrvType uint32
	SrvNo   uint32
	Data    RegInfo
}

var EmptyRegSrvInfo = &RegSrvInfo{
	SrvType: 0,
	SrvNo:   0,
	Data:    nil,
}

type RegGlobalData struct {
	Key  string
	Data RegInfo
}

var EmptyRegGlobalData = &RegGlobalData{
	Key:  "",
	Data: nil,
}

//========================
//     SrvRegImpl
//========================
type SrvRegImpl interface {
	SetWatchSrvAndData(watchedSrvTypes []uint32, watchedGDataKeys []string)
	GetWatchSrvTypes() []uint32
	GetWatchGDataKeys() []string
	SetNets(regNet *RpcNetListener, observerNet *RegPushNetListener)
	ConnRegSrv(regCfg *RegCfg) error
	Init(regCfg *RegCfg) error
	Stop()
	Reset() error
	Register() error
	Watch() error
	FetchInfos() error
	GetSrvInfo(peerType uint32, peerNo uint32) (*RegSrvInfo, bool)
	GetSrvInfosByType(peerType uint32) []*RegSrvInfo
	GetGlobalData(key string) (*RegGlobalData, bool)
}

type RegDataProcessor interface {
	ProcessRegGlobalData(key string, data []byte) (*RegGlobalData, error)
	ProcessRegSrvInfo(srvInfo *reg.SrvInfo) (*RegSrvInfo, error)
}

type RegPushListener interface {
	OnGlobalDataRemovePush(key string)
	OnGlobalDataUpdatePush(key string)
	OnSrvInfoRemovePush(peerType uint32, peerNo uint32)
	OnSrvInfoUpdatePush(peerType uint32, peerNo uint32)
}

//========================
//     BaseSrvRegImpl
//========================
type BaseSrvRegImpl struct {
	watchedSrvTypes   []uint32
	mapPeerId2SrvInfo map[uint32]*RegSrvInfo
	lckSrvInfo        *sync.RWMutex

	watchedGDataKeys  []string
	mapKey2GlobalData map[string]*RegGlobalData
	lckGlobalData     *sync.RWMutex

	p2pCli        *p2pnet.SimpleClient
	regNet        *RpcNetListener
	observerNet   *RegPushNetListener
	regCli        *reg.Client
	dataProcessor RegDataProcessor
	pushListener  RegPushListener
	logger        *yx.Logger
	ec            *yx.ErrCatcher
}

func NewBaseSrvRegImpl(p2pCli *p2pnet.SimpleClient, p RegDataProcessor) *BaseSrvRegImpl {
	return &BaseSrvRegImpl{
		watchedSrvTypes:   nil,
		mapPeerId2SrvInfo: make(map[uint32]*RegSrvInfo),
		lckSrvInfo:        &sync.RWMutex{},
		watchedGDataKeys:  nil,
		mapKey2GlobalData: make(map[string]*RegGlobalData),
		lckGlobalData:     &sync.RWMutex{},
		p2pCli:            p2pCli,
		regCli:            nil,
		dataProcessor:     p,
		pushListener:      nil,
		logger:            yx.NewLogger("BaseSrvRegImpl"),
		ec:                yx.NewErrCatcher("BaseSrvRegImpl"),
	}
}

func (r *BaseSrvRegImpl) SetWatchSrvAndData(watchedSrvTypes []uint32, watchedGDataKeys []string) {
	r.watchedSrvTypes = watchedSrvTypes
	r.watchedGDataKeys = watchedGDataKeys
}

func (r *BaseSrvRegImpl) GetWatchSrvTypes() []uint32 {
	return r.watchedSrvTypes
}

func (r *BaseSrvRegImpl) GetWatchGDataKeys() []string {
	return r.watchedGDataKeys
}

func (r *BaseSrvRegImpl) SetPushListener(l RegPushListener) {
	r.pushListener = l
}

func (r *BaseSrvRegImpl) SetNets(regNet *RpcNetListener, observerNet *RegPushNetListener) {
	r.regNet = regNet
	r.observerNet = observerNet
}

func (r *BaseSrvRegImpl) ConnRegSrv(regCfg *RegCfg) error {
	addr := regCfg.Address + ":" + strconv.FormatUint(uint64(regCfg.Port), 10)
	err := r.p2pCli.OpenConn(regCfg.PeerType, regCfg.PeerNo, regCfg.Network, addr, time.Duration(regCfg.Timeout)*time.Second, true)
	return r.ec.Throw("ConnRegSrv", err)
}

func (r *BaseSrvRegImpl) Init(regCfg *RegCfg) error {
	var err error = nil

	// connect sock
	r.logger.I("connect to register server...")
	err = r.ConnRegSrv(regCfg)
	if err != nil {
		return r.ec.Throw("Init", err)
	}

	// start regCli
	r.logger.I("fetch function list...")
	r.regCli = reg.NewClient(r.regNet, r.observerNet, regCfg.PeerType, regCfg.PeerNo)
	r.regCli.Start()
	go r.regCli.ListenDataOprPush(r.handleRegPush)

	err = r.regCli.FetchFuncList()
	return r.ec.Throw("Init", err)
}

func (r *BaseSrvRegImpl) Stop() {
	if r.regCli != nil {
		r.regCli.Stop()
	}
}

func (r *BaseSrvRegImpl) Reset() error {
	err := r.regCli.FetchFuncList()
	if err != nil {
		return r.ec.Throw("Reset", err)
	}

	r.clearSrvInfos()
	r.clearGlobalDatas()
	return nil
}

func (r *BaseSrvRegImpl) Register() error {
	return nil
}

func (r *BaseSrvRegImpl) Watch() error {
	r.logger.I("watch...")

	var err error = nil
	defer r.ec.DeferThrow("Watch", &err)

	regCli := r.GetRegCli()
	if len(r.watchedSrvTypes) > 0 {
		for _, srvType := range r.watchedSrvTypes {
			err = regCli.WatchSrvsByType(srvType)
			if err != nil {
				return err
			}
		}
	}

	if len(r.watchedGDataKeys) > 0 {
		for _, gDataKey := range r.watchedGDataKeys {
			err = regCli.WatchGlobalData(gDataKey)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *BaseSrvRegImpl) FetchInfos() error {
	r.logger.I("get info...")

	var err error = nil
	defer r.ec.DeferThrow("FetchInfos", &err)

	if len(r.watchedSrvTypes) > 0 {
		for _, srvType := range r.watchedSrvTypes {
			_, err = r.FetchSrvInfos(srvType)
			if err != nil {
				return err
			}
		}
	}

	if len(r.watchedGDataKeys) > 0 {
		for _, gDataKey := range r.watchedGDataKeys {
			_, err = r.FetchGlobalData(gDataKey)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *BaseSrvRegImpl) GetRegCli() *reg.Client {
	return r.regCli
}

func (r *BaseSrvRegImpl) FetchSrvInfos(srvType uint32) ([]*RegSrvInfo, error) {
	r.logger.I("fetch server infos...")
	r.logger.I("srvType = ", srvType)

	srvInfos, err := r.regCli.GetSrvsByType(srvType)
	if err != nil {
		if err == reg.ErrRegCallFailed {
			return []*RegSrvInfo{}, nil
		}

		return nil, r.ec.Throw("FetchSrvInfos", err)
	}

	regInfos := r.addSrvInfos(srvInfos)
	if r.pushListener != nil {
		for _, regInfo := range regInfos {
			r.pushListener.OnSrvInfoUpdatePush(regInfo.SrvType, regInfo.SrvNo)
		}
	}

	return regInfos, nil
}

func (r *BaseSrvRegImpl) FetchSrvInfo(srvType uint32, srvNo uint32) (*RegSrvInfo, error) {
	r.logger.I("fetch server info...")
	r.logger.I("srvType = ", srvType)

	srvInfo, err := r.regCli.GetSrv(srvType, srvNo)
	if err != nil {
		if err == reg.ErrRegCallFailed {
			return EmptyRegSrvInfo, nil
		}

		return nil, r.ec.Throw("FetchSrvInfo", err)
	}

	regInfo, err := r.addSrvInfo(srvInfo)
	if err != nil {
		return nil, r.ec.Throw("FetchSrvInfo", err)
	}

	return regInfo, nil
}

func (r *BaseSrvRegImpl) FetchGlobalData(key string) (*RegGlobalData, error) {
	r.logger.I("fetch global data info...")
	r.logger.I("key = ", key)

	info, err := r.regCli.GetGlobalData(key)
	if err != nil {
		if err == reg.ErrRegCallFailed {
			return EmptyRegGlobalData, nil
		}

		return nil, r.ec.Throw("FetchGlobalData", err)
	}

	regInfo := r.setGlobalData(key, info)
	return regInfo, nil
}

func (r *BaseSrvRegImpl) GetSrvInfo(peerType uint32, peerNo uint32) (*RegSrvInfo, bool) {
	r.lckSrvInfo.RLock()
	defer r.lckSrvInfo.RUnlock()

	peerId := r.getPeerId(peerType, peerNo)
	info, ok := r.mapPeerId2SrvInfo[peerId]
	return info, ok
}

func (r *BaseSrvRegImpl) GetSrvInfosByType(peerType uint32) []*RegSrvInfo {
	r.lckSrvInfo.RLock()
	defer r.lckSrvInfo.RUnlock()

	infoArr := make([]*RegSrvInfo, 0)
	for peerId, info := range r.mapPeerId2SrvInfo {
		existPeerType := peerId >> 16
		if existPeerType == peerType {
			infoArr = append(infoArr, info)
		}
	}

	return infoArr
}

func (r *BaseSrvRegImpl) GetGlobalData(key string) (*RegGlobalData, bool) {
	r.lckGlobalData.RLock()
	defer r.lckGlobalData.RUnlock()

	info, ok := r.mapKey2GlobalData[key]
	return info, ok
}

func (r *BaseSrvRegImpl) handleRegPush(keyType int, key string, operate int) {
	if keyType == reg.KEY_TYPE_GLOBAL_DATA {
		if operate == reg.DATA_OPR_TYPE_REMOVE {
			r.removeGlobalData(key)
			if r.pushListener != nil {
				r.pushListener.OnGlobalDataRemovePush(key)
			}

		} else {
			data, err := r.regCli.GetGlobalData(key)
			if err != nil {
				return
			}

			r.setGlobalData(key, data)
			if r.pushListener != nil {
				r.pushListener.OnGlobalDataUpdatePush(key)
			}
		}
	} else if keyType == reg.KEY_TYPE_SRV_INFO {
		peerType, peerNo := reg.GetSrvTypeAndNo(key)

		if operate == reg.DATA_OPR_TYPE_REMOVE {
			r.removeSrvInfo(peerType, peerNo)
			if r.pushListener != nil {
				r.pushListener.OnSrvInfoRemovePush(peerType, peerNo)
			}

		} else {
			info, err := r.regCli.GetSrvByKey(key)
			if err != nil {
				return
			}

			r.addSrvInfo(info)
			if r.pushListener != nil {
				r.pushListener.OnSrvInfoUpdatePush(peerType, peerNo)
			}
		}
	}
}

func (r *BaseSrvRegImpl) getPeerId(peerType uint32, peerNo uint32) uint32 {
	return (peerType << 16) | peerNo
}

func (r *BaseSrvRegImpl) addSrvInfos(srvInfos []*reg.SrvInfo) []*RegSrvInfo {
	regInfos := make([]*RegSrvInfo, 0)

	if r.dataProcessor == nil {
		return regInfos
	}

	r.lckSrvInfo.Lock()
	defer r.lckSrvInfo.Unlock()

	for _, info := range srvInfos {
		regInfo, err := r.dataProcessor.ProcessRegSrvInfo(info)
		if err != nil {
			continue
		}

		peerId := r.getPeerId(info.SrvType, info.SrvNo)
		r.mapPeerId2SrvInfo[peerId] = regInfo
		regInfos = append(regInfos, regInfo)
	}

	return regInfos
}

func (r *BaseSrvRegImpl) addSrvInfo(info *reg.SrvInfo) (*RegSrvInfo, error) {
	if r.dataProcessor == nil {
		return nil, ErrNoDataProcessor
	}

	regInfo, err := r.dataProcessor.ProcessRegSrvInfo(info)
	if err != nil {
		return nil, err
	}

	r.lckSrvInfo.Lock()
	defer r.lckSrvInfo.Unlock()

	peerId := r.getPeerId(info.SrvType, info.SrvNo)
	r.mapPeerId2SrvInfo[peerId] = regInfo
	return regInfo, nil
}

func (r *BaseSrvRegImpl) removeSrvInfo(peerType uint32, peerNo uint32) {
	r.lckSrvInfo.Lock()
	defer r.lckSrvInfo.Unlock()

	peerId := r.getPeerId(peerType, peerNo)
	_, ok := r.mapPeerId2SrvInfo[peerId]
	if ok {
		delete(r.mapPeerId2SrvInfo, peerId)
	}
}

func (r *BaseSrvRegImpl) clearSrvInfos() {
	r.lckSrvInfo.Lock()
	defer r.lckSrvInfo.Unlock()
	r.mapPeerId2SrvInfo = make(map[uint32]*RegSrvInfo)
}

func (r *BaseSrvRegImpl) setGlobalData(key string, data []byte) *RegGlobalData {
	if r.dataProcessor == nil {
		return nil
	}

	regData, err := r.dataProcessor.ProcessRegGlobalData(key, data)
	if err != nil {
		return nil
	}

	r.lckGlobalData.Lock()
	defer r.lckGlobalData.Unlock()

	r.mapKey2GlobalData[key] = regData
	return regData
}

func (r *BaseSrvRegImpl) removeGlobalData(key string) {
	r.lckGlobalData.Lock()
	defer r.lckGlobalData.Unlock()

	_, ok := r.mapKey2GlobalData[key]
	if ok {
		delete(r.mapKey2GlobalData, key)
	}
}

func (r *BaseSrvRegImpl) clearGlobalDatas() {
	r.lckGlobalData.Lock()
	defer r.lckGlobalData.Unlock()

	r.mapKey2GlobalData = make(map[string]*RegGlobalData)
}

//========================
//     SrvReg
//========================
type SrvReg struct {
	impl   SrvRegImpl
	logger *yx.Logger
	ec     *yx.ErrCatcher
}

func NewSrvReg(impl SrvRegImpl) *SrvReg {
	return &SrvReg{
		impl:   impl,
		logger: yx.NewLogger("SrvReg"),
		ec:     yx.NewErrCatcher("SrvReg"),
	}
}

func (r *SrvReg) SetNets(regNet *RpcNetListener, observerNet *RegPushNetListener) {
	r.impl.SetNets(regNet, observerNet)
}

func (r *SrvReg) Init(regCfg *RegCfg) error {
	err := r.impl.Init(regCfg)
	return r.ec.Throw("Init", err)
}

func (r *SrvReg) Start() error {
	var err error = nil
	defer r.ec.DeferThrow("Start", &err)

	// Register
	err = r.impl.Register()
	if err != nil {
		return err
	}

	// Watch
	err = r.impl.Watch()
	if err != nil {
		return err
	}

	// GetInfo
	err = r.impl.FetchInfos()
	if err != nil {
		return err
	}

	return nil
}

func (r *SrvReg) Stop() {
	r.impl.Stop()
}

func (r *SrvReg) Register() error {
	err := r.impl.Register()
	return r.ec.Throw("Register", err)
}

func (r *SrvReg) ReconnRegSrv(regCfg *RegCfg) {
	<-time.After(CONN_REG_SRV_DELAY)

	r.logger.I("Reconnect register server...")

	err := r.impl.ConnRegSrv(regCfg)
	if err != nil {
		r.logger.E("Reconnect register server err: ", err)
		r.ec.Catch("ReconnRegSrv", &err)

		r.ReconnRegSrv(regCfg)
		return
	}

	r.logger.I("Reconnect register server success !!")

	<-time.After(RESTART_REG_DELAY)
	err = r.impl.Reset()
	if err != nil {
		r.logger.E("Reset err: ", err)
		r.ec.Catch("ReconnRegSrv", &err)
		return
	}

	err = r.Start()
	if err != nil {
		r.logger.E("Start err: ", err)
		r.ec.Catch("ReconnRegSrv", &err)
	}
}

func (r *SrvReg) GetSrvInfo(peerType uint32, peerNo uint32) (*RegSrvInfo, bool) {
	return r.impl.GetSrvInfo(peerType, peerNo)
}

func (r *SrvReg) GetSrvInfosByType(peerType uint32) []*RegSrvInfo {
	return r.impl.GetSrvInfosByType(peerType)
}

func (r *SrvReg) GetGlobalData(key string) (*RegGlobalData, bool) {
	return r.impl.GetGlobalData(key)
}
