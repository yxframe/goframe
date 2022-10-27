// Copyright 2022 Guan Jianchang. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goframe

import (
	"encoding/base64"
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
	// ErrNoDataProcessor = errors.New("no data processor")
	ErrStructNotRegister = errors.New("struct not register")
)

const (
	CONN_REG_SRV_DELAY = 5 * time.Second
	RESTART_REG_DELAY  = 1 * time.Second
)

type RegSrvInfo struct {
	SrvType uint32
	SrvNo   uint32
	Data    interface{}
}

var EmptyRegSrvInfo = &RegSrvInfo{
	SrvType: 0,
	SrvNo:   0,
	Data:    nil,
}

type RegGlobalData struct {
	Key  string
	Data interface{}
}

var EmptyRegGlobalData = &RegGlobalData{
	Key:  "",
	Data: nil,
}

//========================
//     RegCenterImpl
//========================
type RegCenterImpl interface {
	// SetWatchSrvAndData(watchedSrvTypes []uint32, watchedGDataKeys []string)
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

//========================
//     RegCenter
//========================
type RegCenter struct {
	impl   RegCenterImpl
	logger *yx.Logger
	ec     *yx.ErrCatcher
}

func NewRegCenter(impl RegCenterImpl, regNet *RpcNetListener, observerNet *RegPushNetListener) *RegCenter {
	r := &RegCenter{
		impl:   impl,
		logger: yx.NewLogger("RegCenter"),
		ec:     yx.NewErrCatcher("RegCenter"),
	}

	r.impl.SetNets(regNet, observerNet)
	return r
}

// func (r *RegCenter) SetNets(regNet *RpcNetListener, observerNet *RegPushNetListener) {
// 	r.impl.SetNets(regNet, observerNet)
// }

func (r *RegCenter) Init(regCfg *RegCfg) error {
	err := r.impl.Init(regCfg)
	return r.ec.Throw("Init", err)
}

func (r *RegCenter) Start() error {
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

func (r *RegCenter) Stop() {
	r.impl.Stop()
}

func (r *RegCenter) Register() error {
	err := r.impl.Register()
	return r.ec.Throw("Register", err)
}

func (r *RegCenter) ReconnRegSrv(regCfg *RegCfg) {
	var err error = nil

	for {
		err = r.reconnRegSrvImpl(regCfg)
		if err != nil {
			r.ec.Catch("ReconnRegSrv", &err)
			continue
		}

		break
	}
}

func (r *RegCenter) reconnRegSrvImpl(regCfg *RegCfg) error {
	<-time.After(CONN_REG_SRV_DELAY)

	r.logger.I("Reconnect register server...")

	err := r.impl.ConnRegSrv(regCfg)
	if err != nil {
		r.logger.E("Reconnect register server err: ", err)
		return err
	}

	r.logger.I("Reconnect register server success !!")

	<-time.After(RESTART_REG_DELAY)
	err = r.impl.Reset()
	if err != nil {
		r.logger.E("Reset err: ", err)
		return err
	}

	err = r.Start()
	if err != nil {
		r.logger.E("Start err: ", err)
		return err
	}

	return nil
}

func (r *RegCenter) GetSrvInfo(peerType uint32, peerNo uint32) (*RegSrvInfo, bool) {
	return r.impl.GetSrvInfo(peerType, peerNo)
}

func (r *RegCenter) GetSrvInfosByType(peerType uint32) []*RegSrvInfo {
	return r.impl.GetSrvInfosByType(peerType)
}

func (r *RegCenter) GetGlobalData(key string) (*RegGlobalData, bool) {
	return r.impl.GetGlobalData(key)
}

//========================
//     RegPushListener
//========================
type RegPushListener interface {
	OnGlobalDataRemovePush(key string)
	OnGlobalDataUpdatePush(key string)
	OnSrvInfoRemovePush(peerType uint32, peerNo uint32)
	OnSrvInfoUpdatePush(peerType uint32, peerNo uint32)
}

//========================
//     BaseRegCenterImpl
//========================
type BaseRegCenterImpl struct {
	// watchedSrvTypes   []uint32
	mapSrvType2Struct map[uint32]string
	mapPeerId2SrvInfo map[uint32]*RegSrvInfo
	lckSrvInfo        *sync.RWMutex
	// watchedGDataKeys  []string
	mapDataKey2Struct map[string]string
	mapKey2GlobalData map[string]*RegGlobalData
	lckGlobalData     *sync.RWMutex

	p2pCli      *p2pnet.SimpleClient
	regNet      *RpcNetListener
	observerNet *RegPushNetListener
	regCli      *reg.Client
	// dataProcessor RegDataProcessor
	objFactory   *yx.ObjectFactory
	pushListener RegPushListener
	logger       *yx.Logger
	ec           *yx.ErrCatcher
}

func NewBaseRegCenterImpl(p2pCli *p2pnet.SimpleClient, objFactory *yx.ObjectFactory) *BaseRegCenterImpl {
	return &BaseRegCenterImpl{
		// watchedSrvTypes:   nil,
		mapSrvType2Struct: make(map[uint32]string),
		mapPeerId2SrvInfo: make(map[uint32]*RegSrvInfo),
		lckSrvInfo:        &sync.RWMutex{},
		// watchedGDataKeys:      nil,
		mapDataKey2Struct: make(map[string]string),
		mapKey2GlobalData: make(map[string]*RegGlobalData),
		lckGlobalData:     &sync.RWMutex{},
		p2pCli:            p2pCli,
		regCli:            nil,
		// dataProcessor:     p,
		objFactory:   objFactory,
		pushListener: nil,
		logger:       yx.NewLogger("BaseRegCenterImpl"),
		ec:           yx.NewErrCatcher("BaseRegCenterImpl"),
	}
}

// func (r *BaseSrvRegImpl) SetWatchSrvAndData(watchedSrvTypes []uint32, watchedGDataKeys []string) {
// 	r.watchedSrvTypes = watchedSrvTypes
// 	r.watchedGDataKeys = watchedGDataKeys
// }

func (r *BaseRegCenterImpl) GetWatchSrvTypes() []uint32 {
	types := make([]uint32, 0, len(r.mapSrvType2Struct))
	for key := range r.mapSrvType2Struct {
		types = append(types, key)
	}

	return types
}

func (r *BaseRegCenterImpl) GetWatchGDataKeys() []string {
	keys := make([]string, 0, len(r.mapDataKey2Struct))
	for key := range r.mapDataKey2Struct {
		keys = append(keys, key)
	}

	return keys
}

func (r *BaseRegCenterImpl) SetPushListener(l RegPushListener) {
	r.pushListener = l
}

func (r *BaseRegCenterImpl) SetNets(regNet *RpcNetListener, observerNet *RegPushNetListener) {
	r.regNet = regNet
	r.observerNet = observerNet
}

func (r *BaseRegCenterImpl) ConnRegSrv(regCfg *RegCfg) error {
	addr := regCfg.Address + ":" + strconv.FormatUint(uint64(regCfg.Port), 10)
	err := r.p2pCli.OpenConn(regCfg.PeerType, regCfg.PeerNo, regCfg.Network, addr, time.Duration(regCfg.Timeout)*time.Second, true)
	return r.ec.Throw("ConnRegSrv", err)
}

func (r *BaseRegCenterImpl) Init(regCfg *RegCfg) error {
	var err error = nil

	// connect sock
	r.logger.I("connect to register server...")
	err = r.ConnRegSrv(regCfg)
	if err != nil {
		return r.ec.Throw("Init", err)
	}

	// init watch info
	r.mapSrvType2Struct = make(map[uint32]string)
	for _, watchSrvCfg := range regCfg.WatchSrvTypes {
		r.mapSrvType2Struct[watchSrvCfg.SrvType] = watchSrvCfg.Data
	}

	r.mapDataKey2Struct = make(map[string]string)
	for _, watchDataCfg := range regCfg.WatchDataKeys {
		r.mapDataKey2Struct[watchDataCfg.Key] = watchDataCfg.Data
	}

	// start regCli
	r.logger.I("fetch function list...")
	r.regCli = reg.NewClient(r.regNet, r.observerNet, regCfg.PeerType, regCfg.PeerNo)
	r.regCli.Start()
	r.regCli.ListenDataOprPush(r.handleRegPush)

	err = r.regCli.FetchFuncList()
	return r.ec.Throw("Init", err)
}

func (r *BaseRegCenterImpl) Stop() {
	if r.regCli != nil {
		r.regCli.Stop()
	}
}

func (r *BaseRegCenterImpl) Reset() error {
	err := r.regCli.FetchFuncList()
	if err != nil {
		return r.ec.Throw("Reset", err)
	}

	r.clearSrvInfos()
	r.clearGlobalDatas()
	return nil
}

func (r *BaseRegCenterImpl) Register() error {
	return nil
}

func (r *BaseRegCenterImpl) Watch() error {
	r.logger.I("watch...")

	var err error = nil
	defer r.ec.DeferThrow("Watch", &err)

	regCli := r.GetRegCli()
	if len(r.mapSrvType2Struct) > 0 {
		for srvType := range r.mapSrvType2Struct {
			err = regCli.WatchSrvsByType(srvType)
			if err != nil {
				return err
			}
		}
	}

	if len(r.mapDataKey2Struct) > 0 {
		for gDataKey := range r.mapDataKey2Struct {
			err = regCli.WatchGlobalData(gDataKey)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *BaseRegCenterImpl) FetchInfos() error {
	r.logger.I("get info...")

	var err error = nil
	defer r.ec.DeferThrow("FetchInfos", &err)

	if len(r.mapSrvType2Struct) > 0 {
		for srvType := range r.mapSrvType2Struct {
			_, err = r.FetchSrvInfos(srvType)
			if err != nil {
				// return err
				r.logger.E("FetchSrvInfos ", srvType, " err")
				r.ec.Catch("FetchInfos", &err)
			}
		}
	}

	if len(r.mapDataKey2Struct) > 0 {
		for gDataKey := range r.mapDataKey2Struct {
			_, err = r.FetchGlobalData(gDataKey)
			if err != nil {
				// return err
				r.logger.E("FetchGlobalData ", gDataKey, " err")
				r.ec.Catch("FetchInfos", &err)
			}
		}
	}

	return nil
}

func (r *BaseRegCenterImpl) GetRegCli() *reg.Client {
	return r.regCli
}

func (r *BaseRegCenterImpl) FetchSrvInfos(srvType uint32) ([]*RegSrvInfo, error) {
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

func (r *BaseRegCenterImpl) FetchSrvInfo(srvType uint32, srvNo uint32) (*RegSrvInfo, error) {
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

func (r *BaseRegCenterImpl) FetchGlobalData(key string) (*RegGlobalData, error) {
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

func (r *BaseRegCenterImpl) GetSrvInfo(peerType uint32, peerNo uint32) (*RegSrvInfo, bool) {
	r.lckSrvInfo.RLock()
	defer r.lckSrvInfo.RUnlock()

	peerId := r.getPeerId(peerType, peerNo)
	info, ok := r.mapPeerId2SrvInfo[peerId]
	return info, ok
}

func (r *BaseRegCenterImpl) GetSrvInfosByType(peerType uint32) []*RegSrvInfo {
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

func (r *BaseRegCenterImpl) GetGlobalData(key string) (*RegGlobalData, bool) {
	r.lckGlobalData.RLock()
	defer r.lckGlobalData.RUnlock()

	info, ok := r.mapKey2GlobalData[key]
	return info, ok
}

func (r *BaseRegCenterImpl) handleRegPush(keyType int, key string, operate int) {
	if keyType == reg.KEY_TYPE_GLOBAL_DATA {
		if operate == reg.DATA_OPR_TYPE_REMOVE {
			r.removeGlobalData(key)
			if r.pushListener != nil {
				r.pushListener.OnGlobalDataRemovePush(key)
			}

		} else {
			data, err := r.regCli.GetGlobalData(key)
			if err != nil {
				r.ec.Catch("handleRegPush", &err)
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
				r.ec.Catch("handleRegPush", &err)
				return
			}

			r.addSrvInfo(info)
			if r.pushListener != nil {
				r.pushListener.OnSrvInfoUpdatePush(peerType, peerNo)
			}
		}
	}
}

func (r *BaseRegCenterImpl) getPeerId(peerType uint32, peerNo uint32) uint32 {
	return (peerType << 16) | peerNo
}

func (r *BaseRegCenterImpl) addSrvInfos(srvInfos []*reg.SrvInfo) []*RegSrvInfo {
	regInfos := make([]*RegSrvInfo, 0)

	// if r.dataProcessor == nil {
	// 	return regInfos
	// }

	r.lckSrvInfo.Lock()
	defer r.lckSrvInfo.Unlock()

	for _, info := range srvInfos {
		regInfo, err := r.unmarshalSrvInfo(info)
		if err != nil {
			continue
		}

		peerId := r.getPeerId(info.SrvType, info.SrvNo)
		r.mapPeerId2SrvInfo[peerId] = regInfo
		regInfos = append(regInfos, regInfo)
	}

	return regInfos
}

func (r *BaseRegCenterImpl) addSrvInfo(info *reg.SrvInfo) (*RegSrvInfo, error) {
	// if r.dataProcessor == nil {
	// 	return nil, ErrNoDataProcessor
	// }

	regInfo, err := r.unmarshalSrvInfo(info)
	if err != nil {
		return nil, err
	}

	r.lckSrvInfo.Lock()
	defer r.lckSrvInfo.Unlock()

	peerId := r.getPeerId(info.SrvType, info.SrvNo)
	r.mapPeerId2SrvInfo[peerId] = regInfo
	return regInfo, nil
}

func (r *BaseRegCenterImpl) removeSrvInfo(peerType uint32, peerNo uint32) {
	r.lckSrvInfo.Lock()
	defer r.lckSrvInfo.Unlock()

	peerId := r.getPeerId(peerType, peerNo)
	_, ok := r.mapPeerId2SrvInfo[peerId]
	if ok {
		delete(r.mapPeerId2SrvInfo, peerId)
	}
}

func (r *BaseRegCenterImpl) clearSrvInfos() {
	r.lckSrvInfo.Lock()
	defer r.lckSrvInfo.Unlock()
	r.mapPeerId2SrvInfo = make(map[uint32]*RegSrvInfo)
}

func (r *BaseRegCenterImpl) setGlobalData(key string, data []byte) *RegGlobalData {
	// if r.dataProcessor == nil {
	// 	return nil
	// }

	regData, err := r.unmarshalGlobalData(key, data)
	if err != nil {
		return nil
	}

	r.lckGlobalData.Lock()
	defer r.lckGlobalData.Unlock()

	r.mapKey2GlobalData[key] = regData
	return regData
}

func (r *BaseRegCenterImpl) removeGlobalData(key string) {
	r.lckGlobalData.Lock()
	defer r.lckGlobalData.Unlock()

	_, ok := r.mapKey2GlobalData[key]
	if ok {
		delete(r.mapKey2GlobalData, key)
	}
}

func (r *BaseRegCenterImpl) clearGlobalDatas() {
	r.lckGlobalData.Lock()
	defer r.lckGlobalData.Unlock()

	r.mapKey2GlobalData = make(map[string]*RegGlobalData)
}

func (r *BaseRegCenterImpl) unmarshalSrvInfo(info *reg.SrvInfo) (*RegSrvInfo, error) {
	data, err := base64.StdEncoding.DecodeString(info.DataBase64)
	if err != nil {
		return nil, err
	}

	structName, ok := r.mapSrvType2Struct[info.SrvType]
	if !ok {
		return nil, ErrStructNotRegister
	}

	regInfo, err := r.objFactory.CreateObject(structName)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, regInfo)
	if err != nil {
		return nil, err
	}

	regData := &RegSrvInfo{
		SrvType: info.SrvType,
		SrvNo:   info.SrvNo,
		Data:    regInfo,
	}

	return regData, nil
}

func (r *BaseRegCenterImpl) unmarshalGlobalData(key string, data []byte) (*RegGlobalData, error) {
	structName, ok := r.mapDataKey2Struct[key]
	if !ok {
		return nil, ErrStructNotRegister
	}

	regInfo, err := r.objFactory.CreateObject(structName)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, regInfo)
	if err != nil {
		return nil, err
	}

	regData := &RegGlobalData{
		Key:  key,
		Data: regInfo,
	}

	return regData, nil
}
