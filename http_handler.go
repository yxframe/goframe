package goframe

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/yxlib/httpsrv"
	"github.com/yxlib/server"
	"github.com/yxlib/yx"
)

var (
	ErrNotSupportOpr = errors.New("not support this operation")
)

type HttpHandler interface {
	http.Handler
	SetBasePattern(pattern string)
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
	srv         *server.BaseServer
	basePattern string
	cfg         *httpsrv.Config
	tkDecoder   TokenDecoder
	logger      *yx.Logger
	ec          *yx.ErrCatcher
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

func (l *DefaultHttpHandler) SetBasePattern(pattern string) {
	l.basePattern = pattern
}

func (l *DefaultHttpHandler) SetConfig(cfg *httpsrv.Config) {
	l.cfg = cfg
}

func (l *DefaultHttpHandler) SetServer(srv *server.BaseServer) {
	l.srv = srv
}

func (l *DefaultHttpHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var err error = nil
	// defer l.ec.DeferThrow("ServeHTTP", &err)

	respObj := &httpsrv.Response{}
	defer func() {
		writeErr := httpsrv.DefaultWrite(w, l.cfg, respObj, err)
		l.ec.Catch("OnHttpReadPack", &writeErr)
	}()

	// read
	reqObj, err := httpsrv.DefaultRead(req, l.cfg)
	if err != nil {
		respObj.Code = server.RESP_CODE_UNMARSHAL_REQ_FAILED
		return
	}

	respObj.Opr = reqObj.Opr
	respObj.SerialNo = reqObj.SerialNo

	// token
	var connId uint64 = 0
	if l.tkDecoder != nil {
		connId, err = l.tkDecoder.DecodeToken(reqObj.Pattern, reqObj.Opr, reqObj.Token)
		if err != nil {
			respObj.Code = server.RESP_CODE_UNMARSHAL_REQ_FAILED
			return
		}
	}

	log := fmt.Sprint("[C] Connect ID: ", connId, "\n")
	l.logger.Detail(yx.LOG_LV_DEBUG, []string{log})

	// proto No.
	procMapper := l.srv.GetProcMapper()
	pattern := reqObj.Pattern

	baseLen := len(l.basePattern)
	if baseLen > 0 && len(pattern) >= baseLen && strings.Index(pattern, l.basePattern) == 0 {
		pattern = pattern[baseLen:]
	}
	procName := fmt.Sprintf("%s.%s", pattern, reqObj.Opr)
	protoNo, ok := procMapper[procName]
	if !ok {
		respObj.Code = server.RESP_CODE_SYS_UNKNOWN_CMD
		err = ErrNotSupportOpr
		return
	}

	// request
	request := server.NewRequest(connId)
	request.Mod = server.GetMod(protoNo)
	request.Cmd = server.GetCmd(protoNo)
	request.Payload = []byte(reqObj.Params)
	request.SerialNo = reqObj.SerialNo

	// l.logger.I("Module: ", request.Mod)
	// l.logger.I("Command: ", request.Cmd)

	// handle
	response := server.NewResponse(request)
	err = l.srv.HandleRequest(request, response)
	if err == nil {
		respObj.Result = string(response.Payload)
	}

	respObj.Code = response.Code
}
