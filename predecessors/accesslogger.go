package predecessors

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"
	"tools/webapi"
)

type (
	//AccessLogger 访问记录器
	AccessLogger struct {
		accesslogger webapi.LogService
	}
)

//SetupAccessLogger 设置访问日志
func SetupAccessLogger(logger ...webapi.LogService) (accesslogger *AccessLogger) {
	if len(logger) == 0 {
		logger = []webapi.LogService{
			&stdLogger{},
		}
	}
	accesslogger = &AccessLogger{
		accesslogger: logger[0],
	}
	return
}

//Invoke 记录访问日志
func (logger *AccessLogger) Invoke(ctx *webapi.Context, next webapi.HTTPHandler) {
	start := time.Now() // Start
	path := ctx.GetRequest().URL.Path
	next(ctx) // Process request

	latency := time.Since(start)
	clientIP, _, _ := net.SplitHostPort(strings.TrimSpace(ctx.GetRequest().RemoteAddr))
	method := ctx.GetRequest().Method
	//采用自定义写文件方式
	logger.accesslogger.Write("[%s]\t%d\t%s\t%s -> %s\t%s", start.Format("2006-01-02 15:04:05"), ctx.StatusCode(), method, clientIP, path, latency)
}

type (
	stdLogger struct{}
)

func (l *stdLogger) Log(tpl string, args ...interface{}) {
	l.Write(time.Now().Format("[2006-01-02 15:04:05] ")+tpl, args...)
}

func (l *stdLogger) Write(tpl string, args ...interface{}) {
	os.Stdout.WriteString(fmt.Sprintf(tpl, args...) + "\r\n")
}

func (l *stdLogger) Stop() {}
