package webapi

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
)

var (
	//these types can be marshaled to []byte directlty
	marshalableKinds = map[reflect.Kind]bool{
		reflect.Struct: true,
		reflect.Map:    true,
		reflect.Slice:  true,
		reflect.Array:  true,
	}
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

		Deserializer Serializer
		Serializer   Serializer

		BeforeReading func([]byte) []byte
		BeforeWriting func(int, []byte) []byte
	}
)

//Reply Reply to client with any data which can be marshaled into bytes if not bytes or string
func (ctx *Context) Reply(httpstatus int, obj ...interface{}) (err error) {
	var data []byte
	if len(obj) > 0 && obj[0] != nil {
		if _, isErr := obj[0].(error); isErr {
			data = []byte(obj[0].(error).Error())
		} else if entity := reflect.Indirect(reflect.ValueOf(obj[0])); entity.IsValid() {
			value := entity.Interface()
			_, isByte := value.([]byte)
			_, isRune := value.([]rune)
			if kind := entity.Kind(); !isByte && !isRune && marshalableKinds[kind] {
				//serializer is using for reply now.
				//use deserializer to handle body data instead.
				if ctx.Serializer == nil {
					//default is json.
					ctx.Serializer = Serializers["application/json"]
				}
				data, err = ctx.Serializer.Marshal(value)
			} else {
				switch value.(type) {
				case []byte:
					data = value.([]byte)
					break
				case []rune:
					data = []byte(string(value.([]rune)))
					break
				default:
					data = []byte(fmt.Sprintf("%v", value))
				}
			}
		}
		if err != nil {
			return
		}
	}
	return ctx.Write(httpstatus, data)
}

//Write Write to response(only for once)
func (ctx *Context) Write(httpstatus int, data []byte) (err error) {
	if ctx.statuscode == 0 {
		ctx.statuscode = httpstatus
		ctx.w.WriteHeader(httpstatus)
		if ctx.BeforeWriting != nil && len(data) > 0 {
			data = ctx.BeforeWriting(ctx.statuscode, data)
		}
		if len(data) > 0 {
			_, err = ctx.w.Write(data)
		}
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
	ctx.w.Header().Set("Location", addr)
	ctx.w.WriteHeader(ctx.statuscode)
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

//GetResponseWriter Get ResponseWriter as io.Writer to support stream write
func (ctx *Context) GetResponseWriter() io.Writer {
	return ctx.w
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
