package webapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type (
	//Predecessor 前置件
	Predecessor interface {
		Invoke(ctx *Context, next HTTPHandler)
	}

	//HTTPHandler 控制器执行
	HTTPHandler func(*Context)

	//Context 请求上下文
	Context struct {
		statuscode   int
		w            http.ResponseWriter
		r            *http.Request
		predecessors []Predecessor

		Crypto         CryptoService
		errorCollector func(*Context, interface{}) interface{}
	}
)

//Reply 回复
/*
	回复支持错误收集功能
	当 httpstatus 代码不为 299 以下，则会允许在第[3]个参数提供是否提交给用户的选项。
	默认不提供错误详情，如果错误收集器被指派工作，那么这个时候将会把数据传递给收集器做记载处理。
	如果收集器认为有必要给出额外信息（例如：错误代码）那么返回一个回复实体以明文的形式提交给客户端。
*/
func (ctx *Context) Reply(httpstatus int, obj ...interface{}) (err error) {
	var data = []byte(http.StatusText(ctx.statuscode))
	notify2User := ctx.statuscode < 300
	if len(obj) > 1 {
		if val, isBool := obj[1].(bool); isBool && val {
			notify2User = true
		} else if ctx.errorCollector != nil {
			result := ctx.errorCollector(ctx, obj[0])
			if result != nil {
				//错误记录处理后返回处理的信息
				notify2User = true
				obj[0] = result
			}
		}
	}
	if notify2User {
		if str, isStr := obj[0].(string); isStr {
			data = []byte(str)
		} else if bytearr, isBytearr := obj[0].([]byte); isBytearr {
			data = bytearr
		} else {
			data, err = json.Marshal(obj[0])
			if err != nil {
				return err
			}
		}
		if httpstatus < 300 && ctx.Crypto != nil {
			//加密
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

//ResponseHeader 头部
func (ctx *Context) ResponseHeader() http.Header {
	return ctx.w.Header()
}

//GetContext 获取上下文信息
func (ctx *Context) GetContext() *Context {
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
