// Copyright 2022 Guan Jianchang. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package basics

import (
	"errors"

	"github.com/yxlib/server"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	ErrNotProtoType = errors.New("not protoreflect.ProtoMessage type")
)

type PbInterceptor struct {
}

func (i *PbInterceptor) OnPreHandle(req *server.Request, resp *server.Response) (int32, error) {
	if req.Payload != nil {
		reqData, err := server.ProtoBinder.GetRequest(req.Mod, req.Cmd)
		if err != nil {
			return server.RESP_CODE_NOT_SUPPORT_PROTO, err
		}

		protoObj, ok := reqData.(protoreflect.ProtoMessage)
		if !ok {
			return server.RESP_CODE_NOT_SUPPORT_PROTO, ErrNotProtoType
		}

		err = proto.Unmarshal(req.Payload, protoObj)
		if err != nil {
			return server.RESP_CODE_UNMARSHAL_REQ_FAILED, err
		}

		req.ExtData = protoObj
	}

	respData, err := server.ProtoBinder.GetResponse(resp.Mod, resp.Cmd)
	if err != nil {
		return server.RESP_CODE_NOT_SUPPORT_PROTO, err
	}

	resp.ExtData = respData
	return 0, nil
}

func (i *PbInterceptor) OnHandleCompletion(req *server.Request, resp *server.Response) (int32, error) {
	protoObj, ok := resp.ExtData.(protoreflect.ProtoMessage)
	if !ok {
		return server.RESP_CODE_NOT_SUPPORT_PROTO, ErrNotProtoType
	}

	respPayload, err := proto.Marshal(protoObj)
	if err != nil {
		return server.RESP_CODE_MARSHAL_RESP_FAILED, err
	}

	resp.Payload = respPayload
	return 0, nil
}

func (i *PbInterceptor) OnResponseCompletion(req *server.Request, resp *server.Response) error {
	if req != nil {
		server.ProtoBinder.ReuseRequest(req.ExtData, req.Mod, req.Cmd)
	}

	if resp != nil {
		server.ProtoBinder.ReuseResponse(resp.ExtData, resp.Mod, resp.Cmd)
	}

	return nil
}
