package webapi

import "net/http"

type (
	//CryptoService 密码学服务
	CryptoService interface {
		Encrypt([]byte) []byte
		Decrypt([]byte) ([]byte, error)
	}

	//LogService 日志服务
	LogService interface {
		//Log 带有 [datetime] 前缀的日志
		Log(tpl string, args ...interface{})

		//Write 仅文字
		Write(tpl string, args ...interface{})

		Stop()
	}

	//CompleteRequired 需要预先装配的数据
	CompleteRequired interface {
		Complete() error
	}

	//Checkable 数据可检查性
	Checkable interface {
		Check() error
	}

	//Controller 控制器
	Controller interface {
		Redirect(int, string)
		SetCookies(...*http.Cookie)
		Reply(int, ...interface{}) error
		Write(int, []byte) error
		ResponseHeader() http.Header
		GetContext() *Context
	}

	//aliasController 别名控制器
	aliasController interface {
		Controller
		RouteAlias() string
	}
)
