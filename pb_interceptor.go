// Copyright 2022 Guan Jianchang. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goframe

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
	// request
	if len(req.Payload) > 0 {
		reqData, err := server.ProtoBinder.GetRequest(req.Mod, req.Cmd)
		if err != nil {
			return server.RESP_CODE_NOT_SUPPORT_PROTO, err
		}

		protoObj, ok := reqData.(protoreflect.ProtoMessage)
		if !ok {
			return server.RESP_CODE_NOT_SUPPORT_PROTO, ErrNotProtoType
		}

		// always new an object, because it do not reset after reuse.
		msg := protoObj.ProtoReflect().New()
		protoObj = msg.Interface()

		err = proto.Unmarshal(req.Payload, protoObj)
		if err != nil {
			return server.RESP_CODE_UNMARSHAL_REQ_FAILED, err
		}

		req.ExtData = protoObj
	}

	// response
	respData, err := server.ProtoBinder.GetResponse(resp.Mod, resp.Cmd)
	if err == nil {
		protoObj, ok := respData.(protoreflect.ProtoMessage)
		if !ok {
			return server.RESP_CODE_NOT_SUPPORT_PROTO, ErrNotProtoType
		}

		// always new an object, because it do not reset after reuse.
		msg := protoObj.ProtoReflect().New()
		resp.ExtData = msg.Interface()
		// return server.RESP_CODE_NOT_SUPPORT_PROTO, err
	}

	return 0, nil
}

func (i *PbInterceptor) OnHandleCompletion(req *server.Request, resp *server.Response) (int32, error) {
	if resp.ExtData == nil {
		resp.Payload = make([]byte, 0)
		return 0, nil
	}

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
	// if req != nil {
	// 	server.ProtoBinder.ReuseRequest(req.ExtData, req.Mod, req.Cmd)
	// }

	// if resp != nil {
	// 	server.ProtoBinder.ReuseResponse(resp.ExtData, resp.Mod, resp.Cmd)
	// }

	return nil
}
