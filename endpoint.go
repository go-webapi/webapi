package webapi

import (
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

type (
	//endpoint 注册节点
	endpoint struct {
		Params   *param        //来自查询的字段
		Entity   *param        //来自实体的字段
		Context  reflect.Type  //上下文
		Function reflect.Value //实现函数
	}

	param struct {
		reflect.Type
	}
)

func (paramTyp *param) loadFromBytes(body []byte) (*reflect.Value, error) {
	obj, callback := createObj(paramTyp.Type)
	if len(body) > 0 {
		entityObj := obj.Addr().Interface()
		json.Unmarshal(body, entityObj)
		if checkObj, checkable := entityObj.(Checkable); checkable {
			if err := checkObj.Check(); err != nil {
				return nil, err
			}
		}
		obj = callback(reflect.ValueOf(entityObj))
	} else {
		obj = reflect.Zero(paramTyp.Type)
	}
	return &obj, nil
}

func (paramTyp *param) loadFromValues(queries url.Values) (*reflect.Value, error) {
	obj, callback := createObj(paramTyp.Type)
	typ := obj.Type()
	for fieldIndex := 0; fieldIndex < typ.NumField(); fieldIndex++ {
		field := obj.Field(fieldIndex)
		if field.CanSet() {
			ftyp := typ.Field(fieldIndex)
			name := ftyp.Tag.Get("json")
			if len(name) == 0 {
				name = ftyp.Name
			}
			if len(name) > 0 && name != "-" {
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
	{
		objInstance := obj.Addr().Interface()
		if checkObj, checkable := objInstance.(Checkable); checkable {
			if err := checkObj.Check(); err != nil {
				return nil, err
			}
		}
		obj = reflect.ValueOf(objInstance)
	}
	obj = callback(obj)
	return &obj, nil
}

func (typ *param) Load(obj interface{}) (*reflect.Value, error) {
	if b, isBytes := obj.([]byte); isBytes {
		return typ.loadFromBytes(b)
	} else if values, isValues := obj.(url.Values); isValues {
		return typ.loadFromValues(values)
	}
	return nil, errors.New("cannot accept input type " + reflect.TypeOf(obj).Name())
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
