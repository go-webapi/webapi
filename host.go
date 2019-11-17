package webapi

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

var (
	//internalControllerMethods A convenient dictionary of internal usage method fields
	internalControllerMethods      = map[string]bool{}
	internalAliasControllerMethods = map[string]bool{}

	/*
		reservedNamedStructure is used to indicate request forms
		method[*] field is used to indicate how this function will be accessed (allowing multiple).

		Mixed Mode:
		method[*] explicitly specified as the highest priority, and
		you can set a structure reference to set as body and then implicitly declared as POST.
		If you would like to accpet multiple methods, you need use some method[*] to indicate forms.
		E.g.: methodGET, methodPut (case insensitive, but you will get an error if you set a method which is not included in RESTful)
	*/
	reservedNamedStructure = struct {
		Method string
	}{
		"method",
	}

	//supported http request methods dictionary
	supportedMthods = map[string]bool{
		http.MethodConnect: true,
		http.MethodDelete:  true,
		http.MethodGet:     true,
		http.MethodHead:    true,
		http.MethodOptions: true,
		http.MethodPatch:   true,
		http.MethodPost:    true,
		http.MethodPut:     true,
		http.MethodTrace:   true,
	}
)

func init() {
	//generate method keyword dictionary from Controller
	t := reflect.TypeOf(&struct {
		Controller
	}{})
	for index := 0; index < t.NumMethod(); index++ {
		internalControllerMethods[t.Method(index).Name] = true
	}
	//generate method keyword dictionary from aliasController
	t = reflect.TypeOf(&struct {
		aliasController
	}{})
	for index := 0; index < t.NumMethod(); index++ {
		internalAliasControllerMethods[t.Method(index).Name] = true
	}
}

type (
	//Host Service for HTTP
	Host struct {
		handlers map[string]*endpoint
		conf     Config
		errList  []error

		//Stack data
		basepath string
		global   httpHandler
		mstack   []Middleware
	}

	//Config Configuration
	Config struct {
		//UserLowerLetter Use lower letter in path
		UserLowerLetter bool

		//EnableCommentWithDoubleUnderscores This option will ignore the path after the last underscore
		EnableCommentWithUnderscore bool

		//EnablePathSeparationWithUnderscore This option will translate _ to / in the path
		EnablePathSeparationWithUnderscore bool

		//AutoReport This option will display route table after successful registration
		DisableAutoReport bool
	}
)

//NewHost Create a new service host
func NewHost(conf Config, middlewares ...Middleware) (host *Host) {
	host = &Host{
		handlers: map[string]*endpoint{},
		conf:     conf,

		basepath: "",
		global:   pipeline(nil, middlewares...),
		mstack:   middlewares,
	}
	if !conf.DisableAutoReport {
		os.Stdout.WriteString("Registration Info:\r\n")
	}
	host.initCheck()
	return
}

//ServeHTTP service http request
func (host *Host) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		defer r.Body.Close()
	}
	ctx := &Context{
		w:            w,
		r:            r,
		Deserializer: Serializers[strings.Split(r.Header.Get("Content-Type"), ";")[0]],
	}
	collection := host.handlers[strings.ToUpper(r.Method)]
	var run, args = host.global, []string{}
	if collection != nil {
		var path = r.URL.Path
		if host.conf.UserLowerLetter {
			path = strings.ToLower(path)
		}
		handler, arguments := collection.Search(path)
		if handler != nil {
			run = handler.(httpHandler)
			args = arguments
		}
	}
	if run != nil {
		run(ctx, args...)
	}
	if ctx.statuscode == 0 {
		ctx.Reply(http.StatusNotFound, http.StatusText(http.StatusNotFound))
	}
}

//Use Add middlewares into host
func (host *Host) Use(middlewares ...Middleware) *Host {
	if host.mstack == nil {
		host.mstack = middlewares
	} else {
		host.mstack = append(host.mstack, middlewares...)
	}
	host.global = pipeline(host.global, middlewares...)
	return host
}

//Group Set prefix to endpoints
func (host *Host) Group(basepath string, register func(), middlewares ...Middleware) {
	{
		if host.mstack == nil {
			host.mstack = make([]Middleware, 0)
		}
		orginalBasepath, orginalStack := host.basepath, host.mstack
		defer func() {
			//还原栈
			host.mstack, host.basepath = orginalStack, orginalBasepath
		}()
	}
	//处理基地址问题
	host.mstack = append(host.mstack, middlewares...)
	host.basepath = solveBasePath(basepath)
	register()
}

//Register Register the controller with the host
func (host *Host) Register(basePath string, controller Controller, middlewares ...Middleware) (err error) {
	{
		host.initCheck()
		basePath = host.basepath + solveBasePath(basePath)
		defer func() {
			if err != nil {
				host.errList = append(host.errList, err)
			}
		}()
		if len(host.mstack) > 0 {
			//stack data will used to set prior middlewares
			middlewares = append(host.mstack, middlewares...)
		}
	}
	val := reflect.ValueOf(controller)
	typ := val.Type()
	//select the default keyword dictionary
	asideDict := internalControllerMethods
	{
		if alias, isAlias := interface{}(controller).(aliasController); isAlias {
			//if controller can fit aliasController type, replace dictionary from basic to alias one
			asideDict = internalAliasControllerMethods
			basePath += solveBasePath(alias.RouteAlias())
		} else {
			//basic path should be type name
			temp := typ
			for temp.Kind() == reflect.Ptr {
				//need element not reference
				temp = temp.Elem()
			}
			name := temp.Name()
			if strings.ToLower(name) == "homecontroller" {
				name = ""
			} else {
				if index := strings.Index(strings.ToLower(name), "controller"); index > 0 && index+10 == len(name) {
					basePath += "/" + name[0:index]
				} else {
					basePath += "/" + name
				}
			}
		}
	}
	//check prefix request parameters
	initFunc, existed := typ.MethodByName("Init")
	var contextArgs []reflect.Type
	if existed && (initFunc.Type.NumOut() == 1 && initFunc.Type.Out(0) == types.Error) {
		contextArgs = []reflect.Type{}
		//find out all the initialization parameters and record them.
		for index := 1; index < initFunc.Type.NumIn(); index++ {
			arg := initFunc.Type.In(index)
			name, err := getReplacer(arg)
			if err != nil {
				return err
			}
			basePath = filepath.Join(basePath, name)
			contextArgs = append(contextArgs, arg)
		}
	}
	for index := 0; index < typ.NumMethod(); index++ {
		//register all open methods.
		var hasBody, hasQuery bool
		methods, method, path := make(map[string]bool, 0), typ.Method(index), basePath
		inputArgsCount := method.Type.NumIn()
		if asideDict[method.Name] || (method.Name == "Init" && contextArgs != nil) {
			//a special keyword flushed
			continue
		}
		ep := function{
			//created function entity to ready the endpoint
			Function:    method.Func,
			ContextArgs: contextArgs,
			Context:     method.Type.In(0),
			Args:        make([]*param, 0),
		}
		var name = method.Name
		if host.conf.EnableCommentWithUnderscore {
			if arr := strings.Split(name, "_"); len(arr) > 1 {
				name = strings.Join(arr[0:len(arr)-1], "_")
			}
		}
		if host.conf.EnablePathSeparationWithUnderscore {
			name = strings.Replace(name, "_", "/", -1)
		}
		paths := []string{path + "/" + name}
		if name == "Index" {
			//if the method is named of 'Index'
			//both "/Index" and "/" paths will assigned to this method
			paths = append(paths, path+"/")
		}
		for argindex := 1; argindex < inputArgsCount; argindex++ {
			arg := method.Type.In(argindex)
			//If a parameter is a reference, it should be treated as the body structure
			isBody := arg.Kind() == reflect.Ptr
			if isBody || arg.Kind() == reflect.Struct {
				//these logics are test the request forms, it might be existed in
				//both query and body structures
				var temp = arg
				for temp.Kind() == reflect.Ptr {
					//the flowing require element not reference
					temp = temp.Elem()
				}
				if _, errorOccurred := temp.FieldByNameFunc(func(name string) bool {
					if strings.Index(name, reservedNamedStructure.Method) == 0 {
						//only field start with 'method' will be processed
						if httpmethod := strings.ToUpper(strings.Replace(name, reservedNamedStructure.Method, "", 1)); supportedMthods[httpmethod] {
							methods[httpmethod] = true
						} else {
							err = errors.New("cannot accept '" + httpmethod + "' method at method '" + method.Name + "' of '" + basePath + "'")
							return true
						}
					}
					return false
				}); errorOccurred {
					return err
				}
			}
			if isBody {
				if hasBody {
					return errors.New("cannot assign 2 sets from body")
				}
				ep.Args = append(ep.Args, &param{
					Type:   arg,
					isBody: true,
				})
				hasBody = true
			} else if arg.Kind() == reflect.Struct {
				if hasQuery {
					return errors.New("cannot assign 2 sets from query")
				}
				ep.Args = append(ep.Args, &param{
					Type:    arg,
					isQuery: true,
				})
				hasQuery = true
			} else {
				name, err := getReplacer(arg)
				if err != nil {
					return err
				}
				ep.Args = append(ep.Args, &param{
					Type: arg,
				})
				for index := range paths {
					paths[index] = filepath.Join(paths[index], name)
				}
			}
		}
		if host.conf.UserLowerLetter {
			for index, p := range paths {
				paths[index] = strings.ToLower(p)
			}
		}
		if len(methods) == 0 {
			//If the method is not explicitly declared,
			//then fall back to the default rule to register the node.
			if hasBody {
				//body existed might be POST
				methods[http.MethodPost] = true
			} else {
				//no body might be GET
				methods[http.MethodGet] = true
			}
		}
		handler := func(ctx *Context, args ...string) {
			//endpoint is constructed and executable
			var reply = ep.Run(ctx, args...)
			if ctx.statuscode == 0 {
				//if status code is zero, means the reply didn't handle by method
				if len(reply) > 0 {
					//try to reply with the return value
					response, isResp := reply[0].(Replyable)
					if !isResp {
						response = &Reply{
							Status: http.StatusOK,
							Body:   reply[0],
						}
					}
					statusCode := response.StatusCode()
					if statusCode == 0 {
						statusCode = http.StatusOK
						if response.Data() == nil {
							statusCode = http.StatusNoContent
						}
					}
					ctx.Reply(statusCode, response.Data())
				} else {
					//no info can give back to client
					ctx.Reply(http.StatusNoContent)
				}
			}
		}
		for httpmethod := range methods {
			if _, existed := host.handlers[httpmethod]; !existed {
				host.handlers[httpmethod] = &endpoint{}
			}
			for index, p := range paths {
				if err := host.handlers[httpmethod].Add(p, pipeline(handler, middlewares...)); err != nil {
					if index > 0 {
						//if the alias is already existed,
						//jump it directly.
						continue
					}
					return err
				}
				if !host.conf.DisableAutoReport {
					//only 4 letters will be displayed if autoreport
					methodTag := fmt.Sprintf("[%4s]", smallerMethod(httpmethod))
					if index > 0 {
						//it is said that the method will serve as 2 or more endpoints
						methodTag = fmt.Sprintf("%6s", ` ↘`)
					}
					os.Stdout.WriteString(fmt.Sprintf("%s\t%s\r\n", methodTag, p))
				}
			}
		}
	}
	return
}

//AddEndpoint Register the endpoint with the host
func (host *Host) AddEndpoint(method string, path string, handler HTTPHandler, middlewares ...Middleware) (err error) {
	{
		host.initCheck()
		path = host.basepath + solveBasePath(path)
		defer func() {
			if err != nil {
				host.errList = append(host.errList, err)
			}
		}()
	}
	if _, existed := host.handlers[method]; !existed {
		host.handlers[method] = &endpoint{}
	}
	if len(host.mstack) > 0 {
		middlewares = append(host.mstack, middlewares...)
	}
	err = host.handlers[method].Add(path, pipeline(func(context *Context, _ ...string) {
		handler(context)
	}, middlewares...))
	if !host.conf.DisableAutoReport {
		if len(path) == 0 {
			path = "/"
		}
		os.Stdout.WriteString(fmt.Sprintf("[%4s]\t%s\r\n", method, path))
	}
	return
}

//Errors Return server build time error
func (host *Host) Errors() []error {
	return host.errList
}

func (host *Host) initCheck() {
	if host.handlers == nil {
		host.handlers = map[string]*endpoint{}
		host.errList = make([]error, 0)
	}
}

//pipeline create httpHandler with handler and middlewares (Recursive)
func pipeline(handler httpHandler, middlewares ...Middleware) httpHandler {
	if len(middlewares) == 0 {
		return handler
	}
	middleware := middlewares[len(middlewares)-1]
	middlewares = middlewares[:len(middlewares)-1]
	complexHandler := func(ctx *Context, args ...string) {
		//create a composite pipeline using middleware
		middleware.Invoke(ctx, func(arguments ...string) HTTPHandler {
			if handler == nil {
				return func(*Context) {}
			}
			return func(context *Context) {
				handler(context, arguments...)
			}
		}(args...))
	}
	return pipeline(complexHandler, middlewares...)
}

func getReplacer(typ reflect.Type) (string, error) {
	var name string
	switch typ.Kind() {
	case reflect.Int, reflect.Int32, reflect.Int64, reflect.Int16, reflect.Int8, reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint8:
		name = "{digits}"
		break
	case reflect.Float32, reflect.Float64:
		name = "{float}"
		break
	case reflect.Bool:
		name = "{bool}"
		break
	case reflect.String:
		name = "{string}"
		break
	}
	if len(name) == 0 {
		return "", errors.New("cannot accpet type '" + typ.Kind().String() + "'")
	}
	return name, nil
}

func solveBasePath(path string) string {
	if len(path) == 0 || path[0] != '/' {
		path = "/" + path
	}
	if len(path) > 0 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	return path
}

func smallerMethod(method string) string {
	if len(method) > 4 {
		method = method[:4]
	}
	return method
}
