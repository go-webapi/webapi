package webapi

import (
	"errors"
	"net/http"
	"strconv"
)

type (
	//Middleware 前置件
	Middleware interface {
		Invoke(ctx *Context, next HTTPHandler)
	}

	//HTTPHandler 控制器执行
	HTTPHandler func(*Context)

	httpHandler func(*Context, ...string)

	//Context 请求上下文
	Context struct {
		statuscode   int
		w            http.ResponseWriter
		r            *http.Request
		predecessors []Middleware

		Crypto         CryptoService
		Serializer     Serializer
		errorCollector func(*Context, interface{}) interface{}
	}
)

//Reply 回复
func (ctx *Context) Reply(httpstatus int, obj ...interface{}) (err error) {
	var data []byte
	if len(obj) > 0 {
		switch obj[0].(type) {
		case string:
			data = []byte(obj[0].(string))
			break
		case []byte:
			data = obj[0].([]byte)
			break
		case error:
			data = []byte(obj[0].(error).Error())
			break
		default:
			data, err = ctx.Serializer.Marshal(obj)
		}
		if err != nil {
			return
		}
		if httpstatus < 300 && ctx.Crypto != nil {
			data = ctx.Crypto.Encrypt(data)
		}
	}
	return ctx.Write(httpstatus, data)
}

//Write 写入（只允许执行一次写入操作）
func (ctx *Context) Write(httpstatus int, data []byte) (err error) {
	if ctx.statuscode == 0 {
		ctx.statuscode = httpstatus
		ctx.w.WriteHeader(httpstatus)
		_, err = ctx.w.Write(data)
	} else {
		err = errors.New("the last written with " + strconv.Itoa(ctx.statuscode) + " has been submitted")
	}
	return
}

//Redirect 跳转
func (ctx *Context) Redirect(httpstatus int, addr string) {
	if !(httpstatus > 299 && httpstatus < 400) {
		httpstatus = http.StatusPermanentRedirect
	}
	ctx.statuscode = httpstatus
	http.Redirect(ctx.w, ctx.r, addr, httpstatus)
}

//SetCookies 设置小甜饼
func (ctx *Context) SetCookies(cookies ...*http.Cookie) {
	for _, cookie := range cookies {
		http.SetCookie(ctx.w, cookie)
	}
}

//ResponseHeader 应答头
func (ctx *Context) ResponseHeader() http.Header {
	return ctx.w.Header()
}

//Context 获取上下文信息
func (ctx *Context) Context() *Context {
	return ctx
}

//GetRequest 获取请求信息
func (ctx *Context) GetRequest() *http.Request {
	return ctx.r
}

//StatusCode 获取状态码
func (ctx *Context) StatusCode() int {
	return ctx.statuscode
}
