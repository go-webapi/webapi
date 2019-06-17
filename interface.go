package webapi

import (
	"encoding/json"
	"net/http"
)

type (
	//Controller 控制器声明
	Controller interface {
		Redirect(int, string)
		SetCookies(...*http.Cookie)
		Reply(int, ...interface{}) error
		Write(int, []byte) error
		ResponseHeader() http.Header
		Context() *Context
	}

	//aliasController 别名控制器
	aliasController interface {
		Controller
		RouteAlias() string
	}
)

type (
	//CryptoService 密码学服务
	CryptoService interface {
		Encrypt([]byte) []byte
		Decrypt([]byte) ([]byte, error)
	}

	//Validator 验证器
	Validator interface {
		Check() error
	}

	//Serializer 序列器
	Serializer interface {
		Marshal(interface{}) ([]byte, error)
		Unmarshal([]byte, interface{}) error
	}

	//LogService 日志服务
	LogService interface {
		//Log 带有 [datetime] 前缀的日志
		Log(tpl string, args ...interface{})

		//Write 仅文字
		Write(tpl string, args ...interface{})

		Stop()
	}
)

//JSONSerializer JSON 序列器
var JSONSerializer Serializer = &jsonSerializer{}

type jsonSerializer struct{}

func (*jsonSerializer) Marshal(obj interface{}) ([]byte, error) {
	return json.Marshal(obj)
}

func (*jsonSerializer) Unmarshal(src []byte, obj interface{}) error {
	return json.Unmarshal(src, obj)
}
