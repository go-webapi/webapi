package webapi

import (
	"encoding/json"
	"net/http"
)

type (
	//Controller Controller statement
	Controller interface {
		Redirect(string, ...int)
		SetCookies(...*http.Cookie)
		Reply(int, ...interface{}) error
		Write(int, []byte) error
		ResponseHeader() http.Header
		Context() *Context
	}

	//aliasController Alias Controller statement
	aliasController interface {
		Controller
		RouteAlias() string
	}
)

type (
	//Replyable Replyable for request reply
	Replyable interface {
		StatusCode() int
		Data() interface{}
	}

	//CryptoService Cryptography service
	CryptoService interface {
		Encrypt([]byte) []byte
		Decrypt([]byte) ([]byte, error)
	}

	//Validator Validator for body and query structures
	Validator interface {
		Check() error
	}

	//Serializer Serializer
	Serializer interface {
		Marshal(interface{}) ([]byte, error)
		Unmarshal([]byte, interface{}) error
	}

	//LogService Log service
	LogService interface {
		//Log with [datetime] prefix
		Log(tpl string, args ...interface{})

		//Write only text
		Write(tpl string, args ...interface{})

		//Stop exit
		Stop()
	}
)

//JSONSerializer JSON Serializer
var JSONSerializer Serializer = &jsonSerializer{}

type jsonSerializer struct{}

func (*jsonSerializer) Marshal(obj interface{}) ([]byte, error) {
	return json.Marshal(obj)
}

func (*jsonSerializer) Unmarshal(src []byte, obj interface{}) error {
	return json.Unmarshal(src, obj)
}

//Reply Default implementation of Response
type Reply struct {
	Status int
	Body   interface{}
}

//StatusCode HTTP Status Code
func (reply Reply) StatusCode() int {
	return reply.Status
}

//Data Body
func (reply Reply) Data() interface{} {
	return reply.Body
}
