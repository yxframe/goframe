package goframe

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/yxlib/httpsrv"
	"github.com/yxlib/server"
	"github.com/yxlib/yx"
)

var (
	ErrNotSupportOpr = errors.New("not support this operation")
)

type HttpHandler interface {
	http.Handler
	SetConfig(cfg *httpsrv.Config)
	SetServer(srv *server.BaseServer)
}

type TokenDecoder interface {
	// Decode the token.
	// @param pattern, the url pattern
	// @param opr, the operation
	// @param token, the token.
	// @return uint64, an id which can mark a client.
	// @return error, error.
	DecodeToken(pattern string, opr string, token string) (uint64, error)
}

type DefaultHttpHandler struct {
	srv       *server.BaseServer
	cfg       *httpsrv.Config
	tkDecoder TokenDecoder
	logger    *yx.Logger
	ec        *yx.ErrCatcher
}

func NewDefaultHttpHandler(tkDecoder TokenDecoder) *DefaultHttpHandler {
	return &DefaultHttpHandler{
		srv:       nil,
		cfg:       nil,
		tkDecoder: tkDecoder,
		logger:    yx.NewLogger("DefaultHttpHandler"),
		ec:        yx.NewErrCatcher("DefaultHttpHandler"),
	}
}

func (l *DefaultHttpHandler) SetConfig(cfg *httpsrv.Config) {
	l.cfg = cfg
}

func (l *DefaultHttpHandler) SetServer(srv *server.BaseServer) {
	l.srv = srv
}

func (l *DefaultHttpHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var err error = nil
	defer l.ec.DeferThrow("ServeHTTP", &err)

	var respObj *httpsrv.Response = nil
	defer func() {
		writeErr := httpsrv.DefaultWrite(w, l.cfg, respObj, err)
		l.ec.Catch("OnHttpReadPack", &writeErr)
	}()

	// read
	reqObj, err := httpsrv.DefaultRead(req, l.cfg)
	if err != nil {
		return
	}

	// token
	var connId uint64 = 0
	if l.tkDecoder != nil {
		connId, err = l.tkDecoder.DecodeToken(reqObj.Pattern, reqObj.Opr, reqObj.Token)
		if err != nil {
			return
		}
	}

	l.logger.D("Connect ID: ", connId)

	// proto No.
	procMapper := l.srv.GetProcMapper()
	procName := fmt.Sprintf("%s.%s", reqObj.Pattern, reqObj.Opr)
	protoNo, ok := procMapper[procName]
	if !ok {
		err = ErrNotSupportOpr
		return
	}

	// request
	request := server.NewRequest(connId)
	request.Mod = server.GetMod(protoNo)
	request.Cmd = server.GetCmd(protoNo)
	request.Payload = []byte(reqObj.Params)
	request.SerialNo = reqObj.SerialNo

	l.logger.I("Module: ", request.Mod)
	l.logger.I("Command: ", request.Cmd)

	// handle
	response := server.NewResponse(request)
	err = l.srv.HandleRequest(request, response)
	if err != nil {
		return
	}

	respObj = &httpsrv.Response{
		Opr:      reqObj.Opr,
		SerialNo: reqObj.SerialNo,
		Code:     response.Code,
		Result:   string(response.Payload),
	}
}
