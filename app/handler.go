package app

import (
	"context"
	"reflect"
)

// HandlerFunc 是请求处理器函数。
type HandlerFunc func(ctx context.Context, c *RequestContext)

// HandlersChain 是一组请求处理器函数。
type HandlersChain []HandlerFunc

// Last 获取处理链的最后一个处理器函数（主函数）。
func (c HandlersChain) Last() HandlerFunc {
	if length := len(c); length > 0 {
		return c[length-1]
	}
	return nil
}

var handlerNames = make(map[uintptr]string)

// SetHandlerName 设置处理器的名称。
func SetHandlerName(handler HandlerFunc, name string) {
	handlerNames[getFuncAddr(handler)] = name
}

// GetHandlerName 获取处理器的名称。
func GetHandlerName(handler HandlerFunc) string {
	return handlerNames[getFuncAddr(handler)]
}

func getFuncAddr(v any) uintptr {
	return reflect.ValueOf(reflect.ValueOf(v)).Field(1).Pointer()
}
