package binding

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type foo struct {
	f1 string
}

func TestReflect_TypeID(t *testing.T) {
	_, intType := valueAndTypeID(int(1))
	_, uintType := valueAndTypeID(uint(1))
	_, shouldBeIntType := valueAndTypeID(int(1))
	assert.Equal(t, intType, shouldBeIntType)
	assert.NotEqual(t, intType, uintType)

	foo1 := foo{f1: "1"}
	foo2 := foo{f1: "2"}
	_, foo1Type := valueAndTypeID(foo1)
	_, foo2Type := valueAndTypeID(foo2)
	_, foo1PointerType := valueAndTypeID(&foo1)
	_, foo2PointerType := valueAndTypeID(&foo2)

	assert.Equal(t, foo1Type, foo2Type)
	assert.NotEqual(t, foo1Type, foo1PointerType)
	assert.Equal(t, foo1PointerType, foo2PointerType)
}

func TestReflect_CheckPointer(t *testing.T) {
	foo1 := foo{}
	foo1Val := reflect.ValueOf(foo1)
	err := checkPointer(foo1Val)
	assert.NotNil(t, err)

	foo2 := &foo{}
	foo2Val := reflect.ValueOf(foo2)
	err = checkPointer(foo2Val)
	assert.Nil(t, err)

	foo3 := (*foo)(nil)
	foo3Val := reflect.ValueOf(foo3)
	err = checkPointer(foo3Val)
	assert.NotNil(t, err)
}

func TestReflect_DereferPointer(t *testing.T) {
	var foo1 ****foo
	foo1Val := reflect.ValueOf(foo1)
	rt := dereferPointer(foo1Val)
	assert.NotEqual(t, rt.Kind(), reflect.Ptr)

	var foo2 foo
	foo2Val := reflect.ValueOf(foo2)
	rt = dereferPointer(foo2Val)
	assert.NotEqual(t, rt.Kind(), reflect.Ptr)
	assert.Equal(t, "foo", rt.Name())
}
