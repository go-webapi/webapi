package webapi

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strings"
)

var (
	//internalControllerMethods 一个内部使用方法字段的方便字典
	internalControllerMethods      = map[string]bool{}
	internalAliasControllerMethods = map[string]bool{}

	/*
		reservedNamedStructure 保留字段用于确认请求形式
		1. fromBody 字段用于标记该数据将从正文载入；
		2. fromQuery 字段用于标记带数据将从查询载入；
		3. method[*] 字段用于明示此函数将使用怎样的方式被访问（允许多个）。

		混杂模式：
		method[*] 显式指定为最高优先级，其次为 fromBody，最末为 fromQuery。但是，如果不指定，则视为 fromQuery（缺省）。
		注意，为了便于阅读和调用，fromQuery 永远在最后（顺序反了会出现报错）。
		除去 method[*] 显式指定以外，但凡接口包含 fromBody，即为 POST 注册点。
		如果需要多个请求形式兼容，使用多个 method[*] 显式指定，例如：methodGET、methodPost（大小写不区分，但是如果不匹配 RESTful 的谓词将会视为不兼容）
	*/
	reservedNamedStructure = struct {
		Method string
	}{
		"method",
	}
)

func init() {
	t := reflect.TypeOf(&struct {
		Controller
	}{})
	for index := 0; index < t.NumMethod(); index++ {
		internalControllerMethods[t.Method(index).Name] = true
	}
	t = reflect.TypeOf(&struct {
		aliasController
	}{})
	for index := 0; index < t.NumMethod(); index++ {
		internalAliasControllerMethods[t.Method(index).Name] = true
	}
}

type (
	//Host 服务
	Host struct {
		handlers map[string]*endpoint
		conf     Config

		//堆栈数据
		basepath     string
		mstack       []Middleware
		ErrorHandler func(error) interface{}
	}

	//Config 配置
	Config struct {
		//UserLowerLetter 使用小写 Path
		UserLowerLetter bool

		//AutoReport 自动报告
		DisableAutoReport bool
		// //OpenAPIEndpoint Open API 3.0 的报告地址
		// OpenAPIEndpoint string
	}
)

//NewHost 创建新的服务主机
func NewHost(conf Config, middlewares ...Middleware) (host *Host) {
	host = &Host{
		handlers: map[string]*endpoint{},
		conf:     conf,

		basepath: "",
		mstack:   middlewares,
	}
	if !conf.DisableAutoReport {
		os.Stdout.WriteString("Registration Info:\r\n")
	}
	return
}

//ServeHTTP 启动 HTTP 服务
func (host *Host) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if host.ErrorHandler == nil {
		host.ErrorHandler = func(err error) interface{} {
			return err.Error()
		}
	}
	if r.Body != nil {
		defer r.Body.Close()
	}
	collection := host.handlers[strings.ToUpper(r.Method)]
	if collection == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(http.StatusText(http.StatusNotFound)))
		return
	}
	path := r.URL.Path
	if host.conf.UserLowerLetter {
		path = strings.ToLower(path)
	}
	handler, args := collection.Find(r.URL.Path)
	if handler == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(http.StatusText(http.StatusNotFound)))
		return
	}
	ctx := &Context{
		w:          w,
		r:          r,
		Serializer: JSONSerializer,
	}
	handler(ctx, args...)
}

//Use 使用中间件
func (host *Host) Use(middlewares ...Middleware) *Host {
	if host.mstack == nil {
		host.mstack = middlewares
	} else {
		host.mstack = append(host.mstack, middlewares...)
	}
	return host
}

//Group 组操作
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

//Register 向主机注册控制器
func (host *Host) Register(basePath string, controller Controller, middlewares ...Middleware) (err error) {
	{
		host.initCheck()
		basePath = host.basepath + solveBasePath(basePath)
	}
	val := reflect.ValueOf(controller)
	typ := val.Type()
	asideDict := internalControllerMethods
	{
		if alias, isAlias := interface{}(controller).(aliasController); isAlias {
			asideDict = internalAliasControllerMethods
			aliasName := solveBasePath(alias.RouteAlias())[1:]
			if len(aliasName) == 0 {
				return errors.New("cannot set empty alias")
			}
			basePath += "/" + aliasName
		} else {
			temp := typ
			for temp.Kind() == reflect.Ptr {
				temp = temp.Elem()
			}
			basePath += "/" + temp.Name()
		}
	}
	initFunc, existed := typ.MethodByName("Init")
	var contextArgs []reflect.Type
	if existed && (initFunc.Type.NumOut() == 1 && initFunc.Type.Out(0) == reflect.TypeOf((*error)(nil)).Elem()) {
		contextArgs = []reflect.Type{}
		for index := 1; index < initFunc.Type.NumIn(); index++ {
			arg := initFunc.Type.In(index)
			name, err := getReplacer(arg)
			if err != nil {
				return err
			}
			basePath += ("/" + name)
			contextArgs = append(contextArgs, arg)
		}
	}
	for index := 0; index < typ.NumMethod(); index++ {
		var hasBody, hasQuery bool
		methods, method, path := make(map[string]bool, 0), typ.Method(index), basePath
		inputArgsCount := method.Type.NumIn()
		if asideDict[method.Name] || (method.Name == "Init" && contextArgs != nil) {
			//如果访问到了 Controller 约定方法
			continue
		}
		if len(host.mstack) > 0 {
			middlewares = append(host.mstack, middlewares...)
		}
		ep := function{
			Function:    method.Func,
			ContextArgs: contextArgs,
			Context:     method.Type.In(0),
			Args:        make([]*param, 0),
		}
		if method.Name != "Index" {
			path += ("/" + method.Name)
		}
		for argindex := 1; argindex < inputArgsCount; argindex++ {
			// var methodflag = true
			arg := method.Type.In(argindex)
			isBody := arg.Kind() == reflect.Ptr
			if isBody || arg.Kind() == reflect.Struct {
				var temp = arg
				for temp.Kind() == reflect.Ptr {
					temp = temp.Elem()
				}
				if _, errorOccurred := temp.FieldByNameFunc(func(name string) bool {
					if strings.Index(name, reservedNamedStructure.Method) == 0 {
						switch httpmethod := strings.ToUpper(strings.Replace(name, reservedNamedStructure.Method, "", 1)); httpmethod {
						case http.MethodConnect, http.MethodDelete, http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPatch, http.MethodPost, http.MethodPut, http.MethodTrace:
							methods[httpmethod] = true
							break
						default:
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
				path += ("/" + name)
			}
		}
		if host.conf.UserLowerLetter {
			path = strings.ToLower(path)
		}
		if len(methods) == 0 {
			if hasBody {
				methods[http.MethodPost] = true
			} else {
				methods[http.MethodGet] = true
			}
		}
		handler := func(ctx *Context, args ...string) {
			var reply = host.runEndpoint(&ep, ctx, args...)
			if ctx.statuscode == 0 && len(reply) > 0 {
				ctx.Reply(http.StatusOK, reply[0])
			}
		}
		for httpmethod := range methods {
			if _, existed := host.handlers[httpmethod]; !existed {
				host.handlers[httpmethod] = &endpoint{}
			}
			if err := host.handlers[httpmethod].Add(path, pipeline(handler, middlewares...)); err != nil {
				return err
			}
			if !host.conf.DisableAutoReport {
				os.Stdout.WriteString(fmt.Sprintf("[%s]\t%s\r\n", httpmethod, path))
			}
		}
	}
	return
}

//AddEndpoint 向主机注册端点
func (host *Host) AddEndpoint(method string, path string, handler HTTPHandler, middlewares ...Middleware) (err error) {
	{
		host.initCheck()
		path = host.basepath + solveBasePath(path)
	}
	if _, existed := host.handlers[method]; !existed {
		host.handlers[method] = &endpoint{}
	}
	if len(host.mstack) > 0 {
		middlewares = append(host.mstack, middlewares...)
	}
	host.handlers[method].Add(path, pipeline(func(context *Context, _ ...string) {
		handler(context)
	}, middlewares...))
	if !host.conf.DisableAutoReport {
		os.Stdout.WriteString(fmt.Sprintf("[%s]\t%s\r\n", method, path))
	}
	return
}

//runEndpoint 执行端点
func (host *Host) runEndpoint(method *function, ctx *Context, arguments ...string) (objs []interface{}) {
	args := make([]reflect.Value, 0)
	if method.Context != nil {
		obj, callback := createObj(method.Context)
		obj.FieldByName("Controller").Set(reflect.ValueOf(interface{}(ctx).(Controller)))
		preArgs := []reflect.Value{}
		if len(method.ContextArgs) > 0 {
			for index, arg := range method.ContextArgs {
				val := reflect.New(arg).Elem()
				if err := setValue(val, arguments[index]); err != nil {
					ctx.Reply(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
					return
				}
				preArgs = append(preArgs, val)
			}
			arguments = arguments[len(method.ContextArgs):]
			if err := obj.Addr().MethodByName("Init").Call(preArgs)[0]; err.Interface() != nil {
				ctx.Reply(http.StatusBadRequest, err.Interface().(error))
				return
			}
		}
		args = append(args, callback(obj))
	}
	var index = 0
	for _, arg := range method.Args {
		var val reflect.Value
		if !arg.isBody && !arg.isQuery {
			val = reflect.New(arg.Type).Elem()
			if err := setValue(val, arguments[index]); err != nil {
				ctx.Reply(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
				return
			}
			index++
		} else if arg.isBody {
			body := ctx.Body()
			if len(body) > 0 {
				if ctx.Crypto != nil {
					body, _ = ctx.Crypto.Decrypt(body)
				}
			}
			obj, err := arg.Load(body, ctx.Serializer)
			if obj == nil {
				if err != nil {
					ctx.Reply(http.StatusBadRequest, host.ErrorHandler(err))
				} else {
					ctx.Reply(http.StatusBadRequest)
				}
				return
			}
			val = *obj
		} else if arg.isQuery {
			obj, err := arg.Load(ctx.r.URL.Query())
			if obj == nil {
				if err != nil {
					ctx.Reply(http.StatusBadRequest, host.ErrorHandler(err))
				} else {
					ctx.Reply(http.StatusBadRequest)
				}
				return
			}
			val = *obj
		}
		args = append(args, val)
	}
	result := method.Function.Call(args)
	objs = make([]interface{}, len(result))
	for index, res := range result {
		objs[index] = res.Interface()
	}
	return
}

func (host *Host) initCheck() {
	if host.handlers == nil {
		host.handlers = map[string]*endpoint{}
	}
}

//pipeline 工作管线
func pipeline(handler httpHandler, middlewares ...Middleware) httpHandler {
	if len(middlewares) == 0 {
		return handler
	}
	middleware := middlewares[len(middlewares)-1]
	middlewares = middlewares[:len(middlewares)-1]
	complexHandler := func(ctx *Context, args ...string) {
		//使用中间件创建复合管线
		middleware.Invoke(ctx, func(arguments ...string) HTTPHandler {
			return func(context *Context) {
				handler(context, arguments...)
			}
		}(args...))
	}
	//返回复合管线
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
