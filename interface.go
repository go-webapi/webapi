package webapi

import (
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

	//Validator Validator for body and query structures
	Validator interface {
		Check() error
	}

	//ResponseWriter is a alternative for ResponseWriter Request
	ResponseWriter interface {
		Write(p []byte) (int, error)
		Header() http.Header
		WriteHeader(statusCode int)
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
