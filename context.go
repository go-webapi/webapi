package webapi

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
)

type (
	//Middleware Middleware
	Middleware interface {
		Invoke(ctx *Context, next HTTPHandler)
	}

	//HTTPHandler Public HTTP Handler
	HTTPHandler func(*Context)

	httpHandler func(*Context, ...string)

	//Context HTTP Request Context
	Context struct {
		statuscode   int
		w            http.ResponseWriter
		r            *http.Request
		body         []byte
		predecessors []Middleware

		Crypto         CryptoService
		Deserializer   Serializer
		Serializer     Serializer
		errorCollector func(error) interface{}
	}
)

//Reply Reply to client with any data which can be marshaled into bytes if not bytes or string
func (ctx *Context) Reply(httpstatus int, obj ...interface{}) (err error) {
	var data []byte
	if len(obj) > 0 && obj[0] != nil {
		entity := reflect.ValueOf(obj[0])
	begin:
		value := entity.Interface()
		if kind := entity.Kind(); kind == reflect.Struct {
			//serializer is using for reply now.
			//use deserializer to handle body data instead.
			if ctx.Serializer == nil {
				//default is json.
				ctx.Serializer = Serializers["application/json"]
			}
			data, err = ctx.Serializer.Marshal(value)
		} else if _, iserr := value.(error); !iserr && kind == reflect.Ptr {
			entity = entity.Elem()
			goto begin
		} else {
			switch value.(type) {
			case []byte:
				data = value.([]byte)
				break
			case error:
				data = []byte(value.(error).Error())
				break
			default:
				data = []byte(fmt.Sprintf("%v", value))
			}
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

//Write Write to response(only for once)
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

//Redirect Jump to antoher url
func (ctx *Context) Redirect(addr string, httpstatus ...int) {
	if len(httpstatus) == 0 || !(httpstatus[0] > 299 && httpstatus[0] < 400) {
		httpstatus = []int{http.StatusTemporaryRedirect}
	}
	ctx.statuscode = httpstatus[0]
	http.Redirect(ctx.w, ctx.r, addr, httpstatus[0])
}

//SetCookies Set cookies
func (ctx *Context) SetCookies(cookies ...*http.Cookie) {
	for _, cookie := range cookies {
		http.SetCookie(ctx.w, cookie)
	}
}

//ResponseHeader Response Header
func (ctx *Context) ResponseHeader() http.Header {
	return ctx.w.Header()
}

//Context Get Context
func (ctx *Context) Context() *Context {
	return ctx
}

//GetRequest Get Request from Context
func (ctx *Context) GetRequest() *http.Request {
	return ctx.r
}

//Body The Body Bytes from Context
func (ctx *Context) Body() []byte {
	if ctx.r.Body != nil && ctx.body == nil {
		ctx.body, _ = ioutil.ReadAll(ctx.r.Body)
		if ctx.body == nil {
			ctx.body = []byte{}
		}
	}
	return ctx.body
}

//StatusCode Context Status Code
func (ctx *Context) StatusCode() int {
	return ctx.statuscode
}
