package webapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

type (
	function struct {
		Args        []*param       //Parameters
		ContextArgs []reflect.Type //Construct Parameters for Context
		Context     reflect.Type   //Context
		Function    reflect.Value  //Actual Function
	}

	param struct {
		reflect.Type
		isBody  bool
		isQuery bool
	}
)

var types = struct {
	Error reflect.Type
}{
	reflect.TypeOf((*error)(nil)).Elem(),
}

func (method *function) Run(ctx *Context, arguments ...string) (objs []interface{}) {
	args := make([]reflect.Value, 0)
	if method.Context != nil {
		obj, callback := createObj(method.Context)
		if setController(obj, reflect.ValueOf(interface{}(ctx).(Controller))) {
			var err error
			//init controller
			arguments, err = initController(obj, method, arguments...)
			if err != nil {
				if ctx.statuscode == 0 {
					ctx.Reply(http.StatusBadRequest, err.Error())
				}
				return
			}
		} else {
			ctx.Reply(http.StatusNotFound)
			return
		}
		args = append(args, callback(obj))
	}
	//analyse the params with context instance
	paramArgs, err := ctx.analyseParams(method.Args, arguments...)
	if err != nil {
		if ctx.statuscode == 0 {
			ctx.Reply(http.StatusBadRequest, err.Error())
		}
		return
	}
	//call the function
	result := method.Function.Call(append(args, paramArgs...))
	objs = make([]interface{}, len(result))
	for index, res := range result {
		objs[index] = res.Interface()
	}
	return
}

//Load Load object from data source
func (p *param) Load(obj interface{}, serializer Serializer) (*reflect.Value, error) {
	if b, isBytes := obj.([]byte); isBytes {
		return p.loadFromBytes(b, serializer)
	} else if values, isValues := obj.(url.Values); isValues {
		return p.loadFromValues(values)
	}
	return nil, errors.New("cannot accept input type " + reflect.TypeOf(obj).Name())
}

func (p *param) New() reflect.Value {
	val, function := createObj(p.Type)
	return function(val)
}

//loadFromBytes Load object from bytes
func (p *param) loadFromBytes(body []byte, serializer Serializer) (*reflect.Value, error) {
	var err error
	obj, callback := createObj(p.Type)
	if len(body) > 0 {
		entityObj := obj.Addr().Interface()
		err = serializer.Unmarshal(body, entityObj)
		obj = callback(reflect.ValueOf(entityObj))
	} else {
		obj = callback(obj)
	}
	return &obj, err
}

//loadFromValues Load object from url.Values
func (p *param) loadFromValues(queries url.Values) (*reflect.Value, error) {
	obj, callback := createObj(p.Type)
	if len(queries) > 0 {
		setObj(obj, queries)
		obj = callback(obj)
	} else {
		obj = callback(obj)
	}
	return &obj, nil
}

func setObj(value reflect.Value, queries url.Values) {
	t := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		if field.Kind() == reflect.Struct {
			setObj(field, queries)
			continue
		}
		if field.CanSet() {
			ftyp := t.Field(i)
			name := ftyp.Tag.Get("json")
			if len(name) == 0 {
				name = ftyp.Name
			}
		detect:
			if len(name) > 0 && name != "-" {
				if _, existed := (map[string][]string)(queries)[name]; existed {
					setValue(field, queries.Get(name))
				} else if lower := strings.ToLower(name); lower != name {
					name = lower
					goto detect
				}
			}
		}
	}
}

//createObj Create writable object and return a function which can set back to actual type
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

//setValue Set value to reflect.Value
func setValue(value reflect.Value, data string) (err error) {
	switch value.Type().Kind() {
	case reflect.String:
		value.SetString(data)
		break
	case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64:
		val, _ := strconv.ParseInt(data, 10, 64)
		value.SetInt(val)
		break
	case reflect.Uint, reflect.Uint32, reflect.Uint64, reflect.Uint8, reflect.Uint16:
		val, _ := strconv.ParseUint(data, 10, 64)
		value.SetUint(val)
		break
	case reflect.Float32, reflect.Float64:
		val, _ := strconv.ParseFloat(data, 64)
		value.SetFloat(val)
		break
	case reflect.Bool:
		value.SetBool(strings.ToLower(data) == "true")
		break
	case reflect.Array:
		return setArray(value, strings.Split(data, ","))
	case reflect.Slice:
		if array := strings.Split(data, ","); len(array[0]) > 0 {
			value.Set(reflect.MakeSlice(reflect.SliceOf(value.Type().Elem()), len(array), len(array)))
			return setArray(value, array)
		}
		break
	case reflect.Ptr:
		value.Set(reflect.New(value.Type().Elem()))
		err = setValue(value.Elem(), data)
		break
	}
	return
}

//setArray Set array value to reflect.Value
func setArray(value reflect.Value, data []string) (err error) {
	cap := value.Len()
	if cap > len(data) {
		cap = len(data)
	}
	for index := 0; index < cap; index++ {
		if err = setValue(value.Index(index), data[index]); err != nil {
			return err
		}
	}
	return nil
}

//setController assign to actual field if there is a embedded controller
func setController(value reflect.Value, controller reflect.Value) bool {
	for index := 0; index < value.NumField(); index++ {
		field := value.Field(index)
		if name := value.Type().Field(index).Name; len(name) > 0 && strings.ToLower(name[:1]) == name[:1] {
			continue
		}
		if field.Kind() == reflect.Interface {
			field.Set(controller)
			return true
		} else if field.Kind() == reflect.Ptr {
			//create entity for each field
			field, callback := createObj(field.Type())
			if setController(field, controller) {
				//set to controller
				value.Field(index).Set(callback(field))
				return true
			}
		}
	}
	return false
}

//initController run init function
func initController(obj reflect.Value, method *function, arguments ...string) ([]string, error) {
	preArgs := []reflect.Value{}
	if method.ContextArgs != nil {
		//means preconditions required or ctx parameter existed
		for index, arg := range method.ContextArgs {
			val := reflect.New(arg).Elem()
			if err := setValue(val, arguments[index]); err != nil {
				return nil, errors.New(http.StatusText(http.StatusBadRequest))
			}
			preArgs = append(preArgs, val)
		}
		arguments = arguments[len(method.ContextArgs):]
		//call init function with parameters which are provided by path(query is excluded)
		if err := obj.Addr().MethodByName("Init").Call(preArgs)[0]; err.Interface() != nil {
			return nil, err.Interface().(error)
		}
	}
	return arguments, nil
}

//analyseParams assign value to params
func (ctx *Context) analyseParams(params []*param, arguments ...string) ([]reflect.Value, error) {
	var index = 0
	var args = []reflect.Value{}
	for _, arg := range params {
		var val reflect.Value
		if arg.isBody {
			//load body structure from body with serializer(default will be JSON)
			if ctx.Deserializer != nil {
				var body = ctx.Body()
				if ctx.BeforeReading != nil {
					body = ctx.BeforeReading(body)
				}
				obj, err := arg.Load(body, ctx.Deserializer)
				if err != nil {
					return nil, err
				}
				val = *obj
			} else {
				//if cannot found any suitable serializer,
				//the brand new value will take to method to avoid nil ptr panic.
				//body val won't read in this situation.
				val = arg.New()
			}
		} else if arg.isQuery {
			obj, err := arg.Load(ctx.r.URL.Query(), nil)
			if obj == nil {
				return nil, fmt.Errorf("%v", err)
			}
			val = (*obj).Addr()
		} else {
			//it's a simple param from path(not query)
			val = reflect.New(arg.Type).Elem()
			if err := setValue(val, arguments[index]); err != nil {
				return nil, err
			}
			index++
		}
		//run checker
		if err := runChecker(val); err != nil {
			return nil, err
		} else if arg.isQuery {
			val = val.Elem()
		}
		args = append(args, val)
	}
	return args, nil
}

//runChecker invoke Check function to validate transferring entity
func runChecker(val reflect.Value, checkername ...string) (err error) {
	if len(checkername) == 0 {
		checkername = []string{"Check"}
	}
	if checker := val.MethodByName(checkername[0]); checker.IsValid() && checker.Type().NumIn() == 0 && checker.Type().NumOut() == 1 && checker.Type().Out(0) == types.Error {
		if err := checker.Call(make([]reflect.Value, 0))[0].Interface(); err != nil {
			return err.(error)
		}
	}
	return nil
}
