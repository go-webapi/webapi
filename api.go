/*Package webapi WebAPI 路由主机
 */package webapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"strconv"
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
		FromBody, FromQuery, Method string
	}{
		"fromBody", "fromQuery", "method",
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
	//endpoint 注册节点
	endpoint struct {
		Params   *reflect.Type //来自查询的字段
		Entity   *reflect.Type //来自实体的字段
		Context  reflect.Type  //上下文
		Function reflect.Value //实现函数
	}

	//Config 配置
	Config struct {
		//UserLowerLetter 使用小写 Path
		UserLowerLetter bool
		//AutoReport 自动报告
		DisableAutoReport bool
	}

	//Host 服务主机
	Host struct {
		//方法 - 地址 - HTTP句柄
		handlers map[string]map[string]HTTPHandler
		conf     Config

		//堆栈数据
		basepath string
		pstack   []Predecessor
	}
)

//NewHost 创建新的服务主机
func NewHost(conf Config, predecessors ...Predecessor) (host *Host) {
	host = &Host{
		handlers: map[string]map[string]HTTPHandler{},
		conf:     conf,

		basepath: "",
		pstack:   predecessors,
	}
	return
}

//Use 使用中间件
func (host *Host) Use(predecessors ...Predecessor) *Host {
	if host.pstack == nil {
		host.pstack = predecessors
	} else {
		host.pstack = append(host.pstack, predecessors...)
	}
	return host
}

//Group 组操作
func (host *Host) Group(basepath string, register func(), predecessors ...Predecessor) {
	{
		if host.pstack == nil {
			host.pstack = make([]Predecessor, 0)
		}
		orginalBasepath, orginalStack := host.basepath, host.pstack
		defer func() {
			//还原栈
			host.pstack, host.basepath = orginalStack, orginalBasepath
		}()
	}
	//处理基地址问题
	if len(basepath) == 0 || basepath[0] != '/' {
		basepath = "/" + basepath
	}
	host.pstack = append(host.pstack, predecessors...)
	host.basepath = basepath
	register()
}

//Register 向主机注册控制器
func (host *Host) Register(basePath string, controller Controller, predecessors ...Predecessor) (err error) {
	{
		if host.handlers == nil {
			host.handlers = map[string]map[string]HTTPHandler{}
		}
		//处理基地址问题
		if len(basePath) == 0 || basePath[0] != '/' {
			basePath = "/" + basePath
		}
		if basePath[len(basePath)-1] != '/' {
			basePath += "/"
		}
		basePath = host.basepath + basePath
	}
	val := reflect.ValueOf(controller)
	typ := val.Type()
	asideDict := internalControllerMethods
	{
		if alias, isAlias := interface{}(controller).(aliasController); isAlias {
			asideDict = internalAliasControllerMethods
			basePath += strings.Replace(alias.RouteAlias(), "/", "", -1)
		} else {
			temp := typ
			for temp.Kind() == reflect.Ptr {
				temp = temp.Elem()
			}
			basePath += temp.Name()
		}
	}
	for index := 0; index < typ.NumMethod(); index++ {
		methods := make(map[string]bool, 0)
		method := typ.Method(index)
		inputArgsCount := method.Type.NumIn()
		if inputArgsCount > 3 {
			err = errors.New("ignored action '" + method.Name + "' which have " + strconv.Itoa(inputArgsCount) + "args")
			return
		}
		if asideDict[method.Name] {
			//如果访问到了 Controller 约定方法
			continue
		}
		path := basePath + "/" + method.Name
		if host.conf.UserLowerLetter {
			path = strings.ToLower(path)
		}
		if len(host.pstack) > 0 {
			predecessors = append(host.pstack, predecessors...)
		}
		ep := endpoint{
			Function: method.Func,
		}
		ep.Context = method.Type.In(0)
		for argindex := 1; argindex < inputArgsCount; argindex++ {
			arg := method.Type.In(argindex)
			isBody := inputArgsCount == 3 && argindex == 1 //一定是正文
			{
				var temp = arg
				for temp.Kind() == reflect.Ptr {
					temp = temp.Elem()
				}
				temp.FieldByNameFunc(func(name string) bool {
					if name == reservedNamedStructure.FromBody {
						isBody = true
					}
					if len(name) > len(reservedNamedStructure.Method)+2 {
						switch httpmethod := strings.ToUpper(strings.Replace(name, reservedNamedStructure.Method, "", 1)); httpmethod {
						case http.MethodConnect, http.MethodDelete, http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPatch, http.MethodPost, http.MethodPut, http.MethodTrace:
							methods[httpmethod] = true
							break
						default:
							err = errors.New("cannot accept '" + httpmethod + "' method at endpoint '" + path + "'")
							return true
						}
					}
					return false
				})
			}
			if isBody && argindex != 2 {
				ep.Entity = &arg
			} else {
				if ep.Params != nil {
					return errors.New("cannot assign 2 sets from query")
				}
				ep.Params = &arg
			}
		}
		if len(methods) == 0 {
			if ep.Entity != nil {
				methods[http.MethodPost] = true
			} else {
				methods[http.MethodGet] = true
			}
		}
		handler := func(ctx *Context) {
			var reply = runEndpoint(&ep, ctx)
			if ctx.statuscode == 0 && len(reply) > 0 {
				ctx.Reply(http.StatusOK, reply[0])
			}
		}
		for httpmethod := range methods {
			if _, existed := host.handlers[httpmethod]; !existed {
				host.handlers[httpmethod] = map[string]HTTPHandler{}
			}
			_, existed := host.handlers[httpmethod][path]
			if existed {
				return errors.New("method '" + httpmethod + "' for '" + path + "' is already existed")
			}
			host.handlers[httpmethod][path] = pipeline(handler, predecessors...)
			if !host.conf.DisableAutoReport {
				os.Stdout.WriteString(fmt.Sprintf("[%s]\t%s\r\n", httpmethod, path))
			}
		}
	}
	return
}

//AddEndpoint 向主机注册端点
func (host *Host) AddEndpoint(method string, basePath string, handler HTTPHandler, predecessors ...Predecessor) (err error) {
	{
		if host.handlers == nil {
			host.handlers = map[string]map[string]HTTPHandler{}
		}
		//处理基地址问题
		if len(basePath) == 0 || basePath[0] != '/' {
			basePath = "/" + basePath
		}
		if basePath[len(basePath)-1] != '/' {
			basePath += "/"
		}
		basePath = host.basepath + basePath
	}
	if _, existed := host.handlers[method]; !existed {
		host.handlers[method] = map[string]HTTPHandler{}
	}
	if _, existed := host.handlers[method][basePath]; existed {
		err = errors.New("endpoint '" + basePath + "' is already existed")
	} else {
		if len(host.pstack) > 0 {
			predecessors = append(host.pstack, predecessors...)
		}
		host.handlers[method][basePath] = pipeline(handler, predecessors...)
		if !host.conf.DisableAutoReport {
			os.Stdout.WriteString(fmt.Sprintf("[%s]\t%s\r\n", method, basePath))
		}
	}
	return
}

//ServeHTTP 启动 HTTP 服务
func (host *Host) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	handler := collection[r.URL.Path]
	if handler == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(http.StatusText(http.StatusNotFound)))
		return
	}
	ctx := &Context{
		w: w,
		r: r,
	}
	handler(ctx)
}

//Report 报告路由表信息
func (host *Host) Report() (router map[string][]string) {
	router = map[string][]string{}
	for method, handler := range host.handlers {
		var list = make([]string, len(handler))
		var index = 0
		for path := range handler {
			list[index] = path
			index++
		}
		router[method] = list
	}
	return
}

//runEndpoint 执行端点
func runEndpoint(point *endpoint, ctx *Context) (objs []interface{}) {
	args := make([]reflect.Value, 0)
	if point.Context != nil {
		obj, callback := createObj(point.Context)
		obj.FieldByName("Controller").Set(reflect.ValueOf(interface{}(ctx).(Controller)))
		args = append(args, callback(obj))
	}
	if point.Entity != nil {
		obj, callback := createObj(*point.Entity)
		body, _ := ioutil.ReadAll(ctx.r.Body)
		if len(body) > 0 {
			if ctx.Crypto != nil {
				body, _ = ctx.Crypto.Decrypt(body)
			}
		}
		if len(body) > 0 {
			entityObj := obj.Addr().Interface()
			json.Unmarshal(body, entityObj)
			obj = callback(reflect.ValueOf(entityObj))
		} else {
			obj = reflect.Zero(*point.Entity)
		}
		args = append(args, obj)
	}
	if point.Params != nil {
		obj, callback := createObj(*point.Params)
		queries := ctx.r.URL.Query()
		typ := obj.Type()
		for fieldIndex := 0; fieldIndex < typ.NumField(); fieldIndex++ {
			field := obj.Field(fieldIndex)
			if field.CanSet() {
				ftyp := typ.Field(fieldIndex)
				if name := ftyp.Tag.Get("json"); len(name) > 0 && name != "-" {
					value := queries.Get(name)
					switch field.Type().Kind() {
					case reflect.String:
						field.SetString(value)
						break
					case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64:
						val, _ := strconv.ParseInt(value, 10, 64)
						field.SetInt(val)
						break
					case reflect.Uint, reflect.Uint32, reflect.Uint64, reflect.Uint8, reflect.Uint16:
						val, _ := strconv.ParseUint(value, 10, 64)
						field.SetUint(val)
						break
					case reflect.Float32, reflect.Float64:
						val, _ := strconv.ParseFloat(value, 64)
						field.SetFloat(val)
						break
					case reflect.Bool:
						field.SetBool(strings.ToLower(value) == "true")
						break
					}
				}
			}
		}
		args = append(args, callback(obj))
	}
	result := point.Function.Call(args)
	objs = make([]interface{}, len(result))
	for index, res := range result {
		objs[index] = res.Interface()
	}
	return
}

//createObj 创建可写对象，并返回一个转化它为设定值的函数
func createObj(typ reflect.Type) (reflect.Value, func(reflect.Value) reflect.Value) {
	level := 0
	for typ.Kind() == reflect.Ptr {
		level++
		typ = typ.Elem()
	}
	obj := reflect.New(typ).Elem()
	return obj, func(v reflect.Value) reflect.Value {
		for ; level > 0; level-- {
			obj = obj.Addr()
		}
		return obj
	}
}

//pipeline 工作管线
func pipeline(handler HTTPHandler, predecessors ...Predecessor) HTTPHandler {
	if len(predecessors) == 0 {
		return handler
	}
	predecessor := predecessors[len(predecessors)-1]
	predecessors = predecessors[:len(predecessors)-1]
	complexHandler := func(ctx *Context) {
		//使用中间件创建复合管线
		predecessor.Invoke(ctx, handler)
	}
	//返回复合管线
	return pipeline(complexHandler, predecessors...)
}
