package binding

import (
	"fmt"
	"reflect"
	"unsafe"
)

type emptyInterface struct {
	typeID  uintptr
	dataPtr unsafe.Pointer
}

// 返回 v 的反射值及 typeID
func valueAndTypeID(v any) (reflect.Value, uintptr) {
	header := (*emptyInterface)(unsafe.Pointer(&v))
	rv := reflect.ValueOf(v)
	return rv, header.typeID
}

// 确保 rv 是非空指针，否则报错
func checkPointer(rv reflect.Value) error {
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("接收器必须为非空指针")
	}
	return nil
}

// 将 rv 解指针
func dereferPointer(rv reflect.Value) reflect.Type {
	rt := rv.Type()
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	return rt
}
