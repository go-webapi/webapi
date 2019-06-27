package webapi

import (
	"container/list"
	"errors"
	"strconv"
	"strings"
)

type (
	//endpoint Endpoint and its sub-endpoints
	endpoint struct {
		prior    *endpoint
		val      interface{}
		nodes    map[string]*endpoint
		Fallback func(string, int) (string, error)
	}

	//search keyword
	keyword struct {
		text  string
		times int
	}

	//search stack (actual working object)
	stack struct {
		current  *keyword
		node     *endpoint
		history  *list.List //[]*keyword
		queue    *list.List //[]*keyword
		args     *list.List
		fallback func(string, int) (string, error)
	}
)

func (n *endpoint) setVal(value interface{}, path ...string) (err error) {
	if n.nodes == nil {
		n.nodes = map[string]*endpoint{}
	}
	if len(path) == 0 {
		if n.val == nil {
			n.val = value
			return
		} else {
			return errors.New("the endpoint is already existed")
		}
	}
	name := path[0]
	if _, existed := n.nodes[name]; !existed {
		n.nodes[name] = &endpoint{prior: n}
	}
	return n.nodes[name].setVal(value, path[1:]...)
}

//SetValue Add value to endpoint
func (n *endpoint) Add(path string, value interface{}) (err error) {
	if n.nodes == nil {
		n.nodes = map[string]*endpoint{}
	}
	if strings.Contains(path, "{digits}") || strings.Contains(path, "{float}") || strings.Contains(path, "{string}") || strings.Contains(path, "{bool}") {
		err = n.setVal(value, strings.Split(path, "/")[1:]...)
		if err != nil {
			err = errors.New("the endpoint " + path + " is already existed")
		}
	} else {
		_, existed := n.nodes[path]
		if existed {
			err = errors.New("the endpoint " + path + " is already existed")
		} else {
			n.nodes[path] = &endpoint{
				val: value,
			}
		}
	}
	return
}

//Search Get the endpoint value via keyword list
func (n endpoint) search(path ...string) (value interface{}, args []string) {
	if len(path) == 0 {
		path = []string{""}
	}
	var queue = list.New()
	for _, p := range path[1:] {
		queue.PushBack(&keyword{text: p})
	}
	return (&stack{
		current:  &keyword{text: path[0]},
		history:  list.New(),
		args:     list.New(),
		queue:    queue,
		node:     &n,
		fallback: n.Fallback,
	}).search()
}

func (n endpoint) Search(path string) (value interface{}, args []string) {
	if n.nodes == nil {
		return nil, nil
	}
	if obj, existed := n.nodes[path]; existed {
		return obj.val, nil
	}
	return n.search(strings.Split(path, "/")[1:]...)
}

func (stack *stack) search() (value interface{}, args []string) {
	if stack.fallback == nil {
		stack.fallback = defaultFallback
	}
	var key = stack.current.text
	var err error
	for stack.current.times++; stack.current.times > 1; stack.current.times++ {
		key, err = stack.fallback(stack.current.text, stack.current.times-1)
		if err != nil || len(key) > 0 {
			break
		}
	}
	if err != nil {
		if stack.history.Len() == 0 {
			return nil, nil
		}
		stack.back()
	}
	if node, existed := stack.node.nodes[key]; existed {
		if stack.queue.Len() == 0 {
			params := []string{}
			for stack.args.Front() != nil {
				if arg := stack.args.Remove(stack.args.Front()).(string); len(arg) > 0 {
					params = append(params, arg)
				}
			}
			if stack.current.times > 1 {
				params = append(params, stack.current.text)
			}
			return node.val, params
		}
		stack.next(node)
	}
	return stack.search()
}

func (stack *stack) next(node *endpoint) {
	stack.node = node
	stack.history.PushFront(stack.current)
	var arg string
	if stack.current.times > 1 {
		arg = stack.current.text
	}
	stack.args.PushBack(arg)
	stack.current = stack.queue.Remove(stack.queue.Front()).(*keyword)
}

func (stack *stack) back() {
	stack.node = stack.node.prior
	stack.current.times = 0
	stack.queue.PushFront(stack.current)
	stack.args.Remove(stack.args.Back())
	stack.current = stack.history.Remove(stack.history.Back()).(*keyword)
}

func defaultFallback(value string, times int) (string, error) {
	switch times {
	case 1:
		digit, isDigit := strconv.ParseInt(value, 10, 64)
		decimal, isDecimal := strconv.ParseFloat(value, 64)
		if isDigit == nil && float64(digit) == decimal {
			return "{digits}", nil
		}
		if isDecimal == nil {
			return `{float}`, nil
		}
		if strings.ToLower(value) == "true" || strings.ToLower(value) == "false" {
			return `{bool}`, nil
		}
		return "", nil
	case 2:
		return `{string}`, nil
	default:
		return "", errors.New("")
	}
}
