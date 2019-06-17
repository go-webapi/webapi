package middlewares

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"

	"github.com/johnwiichang/webapi"
)

var (
	recoverSpecialCharacters = struct {
		Dunno, CentreDot, Dot, Slash []byte
	}{
		[]byte("???"), []byte("·"), []byte("."), []byte("/"),
	}

	defaultRecoveryCollector = func(err string, stack string) string {
		return fmt.Sprintf("500 Internal Server Error: %s\r\nStack Trace:\r\n%s", err, stack)
	}
)

//Recovery panic错误捕获中间件
type Recovery struct {
	//RecoveryDataCollector 用于处理收集到的运行时错误报告
	//第一个参数：运行时错误
	//第二个参数：栈跟踪信息
	//返回值：提交给客户端的错误信息
	recoveryCollector func(string, string) string
}

//SetupRecoveryHandler 设置重启中间件的自定义错误处理函数，handler函数不能再次出现未处理的panic，否则服务将中断退出
func SetupRecoveryHandler(handler ...func(string, string) string) (r *Recovery) {
	if len(handler) == 0 {
		handler = []func(string, string) string{
			defaultRecoveryCollector,
		}
	}
	r = &Recovery{
		recoveryCollector: handler[0],
	}
	return
}

//Invoke 前置件调用约定
func (r *Recovery) Invoke(ctx *webapi.Context, next webapi.HTTPHandler) {
	if r.recoveryCollector == nil {
		r.recoveryCollector = defaultRecoveryCollector
	}
	defer func() {
		if err := recover(); err != nil {
			panicInfo := fmt.Sprintf("%v", err)
			stack := string(r.stack(3))
			if r.recoveryCollector == nil {
				return
			}
			if replyMsg := r.recoveryCollector(panicInfo, stack); len(replyMsg) > 0 {
				ctx.Reply(http.StatusInternalServerError, replyMsg, false)
				return
			}
		}
	}()
	next(ctx)
}

// stack returns a nicely formatted stack frame, skipping skip frames.
func (r *Recovery) stack(skip int) []byte {
	buf := new(bytes.Buffer) // the returned data
	// As we loop, we open files and read them. These variables record the currently
	// loaded file.
	var lines [][]byte
	var lastFile string
	for i := skip; ; i++ { // Skip the expected number of frames
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		// Print this much at least.  If we can't find the source, it won't show.
		fmt.Fprintf(buf, "%s:%d (0x%x)\n", file, line, pc)
		if file != lastFile {
			data, err := ioutil.ReadFile(file)
			if err != nil {
				continue
			}
			lines = bytes.Split(data, []byte{'\n'})
			lastFile = file
		}
		fmt.Fprintf(buf, "\t%s: %s\n", r.function(pc), r.source(lines, line))
	}
	return buf.Bytes()
}

// source returns a space-trimmed slice of the n'th line.
func (r *Recovery) source(lines [][]byte, n int) []byte {
	n-- // in stack trace, lines are 1-indexed but our array is 0-indexed
	if n < 0 || n >= len(lines) {
		return recoverSpecialCharacters.Dunno
	}
	return bytes.TrimSpace(lines[n])
}

// function returns, if possible, the name of the function containing the PC.
func (r *Recovery) function(pc uintptr) []byte {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return recoverSpecialCharacters.Dunno
	}
	name := []byte(fn.Name())
	// The name includes the path name to the package, which is unnecessary
	// since the file name is already included.  Plus, it has center dots.
	// That is, we see
	//	runtime/debug.*T·ptrmethod
	// and want
	//	*T.ptrmethod
	// Also the package path might contains dot (e.g. code.google.com/...),
	// so first eliminate the path prefix
	if lastslash := bytes.LastIndex(name, recoverSpecialCharacters.Slash); lastslash >= 0 {
		name = name[lastslash+1:]
	}
	if period := bytes.Index(name, recoverSpecialCharacters.Dot); period >= 0 {
		name = name[period+1:]
	}
	name = bytes.Replace(name, recoverSpecialCharacters.CentreDot, recoverSpecialCharacters.Dot, -1)
	return name
}
