package webapi

import (
	"errors"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

type (
	function struct {
		Args        []*param       //参数
		ContextArgs []reflect.Type //构造参数
		Context     reflect.Type   //上下文
		Function    reflect.Value  //实现函数
	}

	param struct {
		reflect.Type
		isBody  bool
		isQuery bool
	}
)

//Load 从数据源载入对象
func (p *param) Load(obj interface{}, serializer ...Serializer) (*reflect.Value, error) {
	if b, isBytes := obj.([]byte); isBytes {
		return p.loadFromBytes(b, serializer...)
	} else if values, isValues := obj.(url.Values); isValues {
		return p.loadFromValues(values)
	}
	return nil, errors.New("cannot accept input type " + reflect.TypeOf(obj).Name())
}

//loadFromBytes 从字节数组中载入值
func (p *param) loadFromBytes(body []byte, serializer ...Serializer) (*reflect.Value, error) {
	if len(serializer) == 0 {
		serializer = []Serializer{JSONSerializer}
	}
	obj, callback := createObj(p.Type)
	if len(body) > 0 {
		entityObj := obj.Addr().Interface()
		serializer[0].Unmarshal(body, entityObj)
		if checkObj, checkable := entityObj.(Validator); checkable {
			if err := checkObj.Check(); err != nil {
				return nil, err
			}
		}
		obj = callback(reflect.ValueOf(entityObj))
	} else {
		obj = reflect.Zero(p.Type)
	}
	return &obj, nil
}

//loadFromValues 从值对中载入值
func (p *param) loadFromValues(queries url.Values) (*reflect.Value, error) {
	obj, callback := createObj(p.Type)
	objType := obj.Type()
	for fieldIndex := 0; fieldIndex < objType.NumField(); fieldIndex++ {
		field := obj.Field(fieldIndex)
		if field.CanSet() {
			ftyp := objType.Field(fieldIndex)
			name := ftyp.Tag.Get("json")
			if len(name) == 0 {
				name = ftyp.Name
			}
			if len(name) > 0 && name != "-" {
				setValue(field, queries.Get(name))
			}
		}
	}
	{
		objInstance := obj.Addr().Interface()
		if checkObj, checkable := objInstance.(Validator); checkable {
			if err := checkObj.Check(); err != nil {
				return nil, err
			}
		}
		obj = reflect.ValueOf(objInstance)
	}
	obj = callback(obj)
	return &obj, nil
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

//setValue 为 reflect.Value 设置值
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
	}
	return nil
}

//setArray 为 reflect.Value 设置数组
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
