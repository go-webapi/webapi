package webapi

import (
	"errors"
	"strconv"
	"strings"
)

type (
	endpoint struct {
		handler   httpHandler
		endpoints map[string]*endpoint
	}
)

//Add 添加 HTTP 端点
func (point *endpoint) Add(path string, handler httpHandler) error {
	if point.endpoints == nil {
		point.endpoints = map[string]*endpoint{}
	}
	var current = point
	if !strings.Contains(path, "{string}") && !strings.Contains(path, "{digits}") && !strings.Contains(path, "{float}") {
		current.endpoints[path] = &endpoint{handler: handler} //便捷注册法
	} else {
		for _, address := range strings.Split(path, "/")[1:] {
			if _, existed := current.endpoints[address]; !existed {
				current.endpoints[address] = &endpoint{endpoints: map[string]*endpoint{}}
			}
			current = current.endpoints[address]
		}
		if current.handler != nil {
			return errors.New("endpoint already existed")
		}
		current.handler = handler
	}
	return nil
}

//Find 查找 HTTP 端点
func (point *endpoint) Find(path string) (httpHandler, []string) {
	if point.endpoints == nil {
		point.endpoints = map[string]*endpoint{}
	}
	if handler, existed := point.endpoints[path]; existed {
		return handler.handler, nil
	}
	var current = point
	args := []string{}
	var paths = strings.Split(path, "/")[1:]
	if paths[len(paths)-1] == "index" || paths[len(paths)-1] == "Index" {
		paths = paths[:len(paths)-1]
	}
	for _, path := range paths {
		if len(current.endpoints) == 0 {
			break
		}
		obj, existed := current.endpoints[path]
		if !existed {
			if len(path) == 0 {
				continue
			}
			digit, isDigit := strconv.ParseInt(path, 10, 64)
			decimal, isDecimal := strconv.ParseFloat(path, 64)
			if isDigit == nil && float64(digit) == decimal {
				obj, existed = current.endpoints[`{digits}`]
				if existed {
					current = obj
					args = append(args, path)
					continue
				}
			}
			if isDecimal == nil {
				obj, existed = current.endpoints[`{float}`]
				if existed {
					current = obj
					args = append(args, path)
					continue
				}
			}
			if obj, existed = current.endpoints[`{string}`]; existed {
				current = obj
			} else {
				break
			}
			args = append(args, path)
		} else {
			current = obj
		}
	}
	return current.handler, args
}
