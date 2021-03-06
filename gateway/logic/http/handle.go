// Copyright 2018 github.com/xiaoenai. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/henrylee2cn/goutil"
	"github.com/henrylee2cn/erpc/v6"
	"github.com/henrylee2cn/erpc/v6/codec"
	"github.com/henrylee2cn/erpc/v6/plugin/proxy"
	"github.com/valyala/fasthttp"
	"github.com/xiaoenai/tp-micro/v6/gateway/logic"
	"github.com/xiaoenai/tp-micro/v6/gateway/logic/hosts"
)

const (
	// SEQ the packet sequence query parameter specified by the client
	SEQ = "seq_"
)

var (
	allowCross bool
	gwHostsUri string
)

func handler(ctx *fasthttp.RequestCtx) {
	(&requestHandler{ctx: ctx}).handle()
}

type requestHandler struct {
	ctx    *fasthttp.RequestCtx
	errMsg []byte
}

var statInternalServerError = erpc.NewStatus(erpc.CodeInternalServerError, erpc.CodeText(erpc.CodeInternalServerError), "")

func (r *requestHandler) handle() {
	var (
		ctx             = r.ctx
		serviceMethod   = goutil.BytesToString(ctx.Path())
		h               = r.Header()
		contentType     = goutil.BytesToString(h.ContentType())
		bodyCodec       = GetBodyCodec(contentType, codec.ID_PLAIN)
		acceptBodyCodec = GetBodyCodec(goutil.BytesToString(h.Peek("Accept")), bodyCodec)
		query           = r.ctx.QueryArgs()
		bodyBytes       = ctx.Request.Body()
		reply           []byte
		label           proxy.Label
	)
	label.ServiceMethod = serviceMethod

	// set real ip
	if xRealIp := h.Peek("X-Real-IP"); len(xRealIp) > 0 {
		label.RealIP = string(xRealIp)
	} else if xForwardedFor := h.Peek("X-Forwarded-For"); len(xForwardedFor) > 0 {
		label.RealIP = string(bytes.Split(xForwardedFor, []byte{','})[0])
	}
	if len(label.RealIP) == 0 {
		label.RealIP = ctx.RemoteAddr().String()
	}

	start := time.Now()
	defer func() {
		if p := recover(); p != nil {
			r.replyError(statInternalServerError.SetCause(fmt.Sprint(p)))
		}
		r.runlog(start, &label, goutil.BytesToString(query.Peek(SEQ)), bodyBytes, &reply)
	}()

	// cross
	if allowCross && r.crossDomainFilter() {
		return
	}

	// gw hosts
	if label.ServiceMethod == gwHostsUri {
		switch acceptBodyCodec {
		case codec.ID_PROTOBUF:
			b, _ := codec.ProtoMarshal(hosts.GwHosts())
			r.ctx.Success("application/x-protobuf", b)
		default:
			b, _ := json.Marshal(hosts.GwHosts())
			r.ctx.Success("application/json", b)
		}
		return
	}

	// verify access token
	accessToken, settings, stat := logic.HttpHooks().OnRequest(r, bodyBytes, logic.AuthFunc())
	if stat != nil {
		r.replyError(stat)
		return
	}

	// set header
	h.VisitAll(func(key, value []byte) {
		settings = append(settings, erpc.WithAddMeta(string(key), string(value)))
	})

	// set body codec
	settings = append(settings, erpc.WithBodyCodec(bodyCodec))

	// set accept body codec
	if acceptBodyCodec != bodyCodec {
		settings = append(settings, erpc.WithAcceptBodyCodec(acceptBodyCodec))
	}

	// set session id
	if accessToken == nil {
		label.SessionID = ctx.RemoteAddr().String()
	} else {
		label.SessionID = accessToken.SessionId()
		if info := accessToken.AddedQuery(); info != nil {
			info.VisitAll(func(key, value []byte) {
				erpc.Warnf("key-> %s", string(key))
				settings = append(settings, erpc.WithAddMeta(string(key), string(value)))
			})
		}
	}

	settings = append(settings, erpc.WithAddMeta(erpc.MetaRealIP, label.RealIP))

	// set query
	if query.Len() > 0 {
		query.VisitAll(func(key, value []byte) {
			settings = append(settings, erpc.WithAddMeta(string(key), string(value)))
		})
	}
	callcmd := logic.
		ProxySelector(&label).
		Call(label.ServiceMethod, bodyBytes, &reply, settings...)

	// fail
	if stat := callcmd.Status(); stat != nil {
		callcmd.InputMeta().VisitAll(func(key, value []byte) {
			k := goutil.BytesToString(key)
			v := goutil.BytesToString(value)
			ctx.Response.Header.Add(k, v)
		})
		r.replyError(stat)
		return
	}

	// succ
	var hasRespContentType bool
	callcmd.InputMeta().VisitAll(func(key, value []byte) {
		k := goutil.BytesToString(key)
		v := goutil.BytesToString(value)
		if k == "Content-Type" {
			hasRespContentType = true
			ctx.Response.Header.Set(k, v)
		} else {
			ctx.Response.Header.Add(k, v)
		}
	})
	if !hasRespContentType {
		ctx.Response.Header.Add(
			"Content-Type",
			GetContentType(callcmd.InputBodyCodec(), contentType),
		)
	}
	ctx.SetBody(reply)
}

var optionsBytes = []byte("OPTIONS")

func (r *requestHandler) crossDomainFilter() bool {
	r.ctx.Response.Header.Set("Access-Control-Allow-Origin", string(r.ctx.Request.Header.Peek("Origin")))
	r.ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
	r.ctx.Response.Header.Set("Access-Control-Allow-Methods", string(r.ctx.Request.Header.Peek("Access-Control-Request-Method")))
	// r.ctx.Response.Header.Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	r.ctx.Response.Header.Set("Access-Control-Allow-Headers", string(r.ctx.Request.Header.Peek("Access-Control-Request-Headers")))
	// r.ctx.Response.Header.Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	r.ctx.Response.Header.Set("Access-Control-Max-Age", "-1")
	// r.ctx.Response.Header.Set("Access-Control-Max-Age", "172800")
	if !bytes.Equal(r.ctx.Method(), optionsBytes) {
		return false
	}
	r.ctx.SetStatusCode(204)
	return true
}

func (r *requestHandler) replyError(stat *erpc.Status) {
	var statusCode int
	if stat.Code() < 200 {
		// Internal communication error
		statusCode = 500
	} else if stat.Code() < 600 {
		// Custom HTTP error
		statusCode = int(stat.Code())
	} else {
		// Business error
		statusCode = 299
	}
	r.errMsg, _ = stat.MarshalJSON()
	r.ctx.SetStatusCode(statusCode)
	r.ctx.SetContentType("application/json")
	r.ctx.SetBody(r.errMsg)
}

// QueryArgs returns the query arguments object of request.
func (r *requestHandler) QueryArgs() *fasthttp.Args {
	return r.ctx.QueryArgs()
}

// Header returns the header object of request.
func (r *requestHandler) Header() *fasthttp.RequestHeader {
	return &r.ctx.Request.Header
}
