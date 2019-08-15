package middlewares

import (
	"net/http"
	"strings"

	"github.com/go-webapi/webapi"
)

type (
	//StaticFileHandler 静态文件
	StaticFileHandler struct {
		address string
		folder  string
		server  http.Handler
	}
)

//SetupStaticFileSupport 静态文件支持
func SetupStaticFileSupport(address string, folder string) webapi.Middleware {
	if len(folder) > 0 && folder[len(folder)-1] != '/' {
		folder += "/"
	}
	if len(address) > 0 && address[len(address)-1] != '/' {
		address += "/"
	}
	if len(address) > 0 && address[0] != '/' {
		address = "/" + address
	}
	return &StaticFileHandler{
		address: address,
		folder:  folder,
		server:  http.FileServer(http.Dir(folder)),
	}
}

func (handler *StaticFileHandler) Invoke(ctx *webapi.Context, next webapi.HTTPHandler) {
	next(ctx)
	if ctx.StatusCode() == 0 {
		path := ctx.GetRequest().URL.Path
		if strings.Index(path, handler.address) == 0 {
			ctx.GetRequest().URL.Path = strings.Replace(path, handler.address, "", 1)
			handler.server.ServeHTTP(&respWriter{ctx: ctx}, ctx.GetRequest())
		}
	}
}

type respWriter struct {
	ctx *webapi.Context
}

func (w *respWriter) Write(p []byte) (int, error) {
	return w.ctx.GetResponseWriter().Write(p)
}

func (w *respWriter) WriteHeader(statusCode int) {
	w.ctx.Reply(statusCode)
}

func (w *respWriter) Header() http.Header {
	return w.ctx.ResponseHeader()
}
