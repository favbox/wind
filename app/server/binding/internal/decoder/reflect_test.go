package decoder

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type foo struct {
	f1 string
}

type foo2 struct {
	F1 string
}

func TestReferenceValue(t *testing.T) {
	foo1 := foo2{F1: "f1"}
	foo1Val := reflect.ValueOf(foo1)
	foo1PointerVal := ReferenceValue(foo1Val, 5)
	assert.Equal(t, "f1", foo1.F1)
	assert.Equal(t, "f1", foo1Val.Field(0).Interface().(string))
	assert.Equal(t, reflect.Ptr, foo1PointerVal.Kind())
	assert.Equal(t, "*****decoder.foo2", foo1PointerVal.Type().String())

	deFoo1PointerVal := ReferenceValue(foo1PointerVal, -5)
	assert.NotEqual(t, reflect.Ptr, deFoo1PointerVal.Kind())
	assert.Equal(t, "f1", deFoo1PointerVal.Field(0).Interface().(string))
}

func TestGetNonNilReferenceValue(t *testing.T) {
	foo1 := (****foo)(nil)
	foo1Val := reflect.ValueOf(foo1)
	foo1ValNonNil, ptrDepth := GetNonNilReferenceValue(foo1Val)
	assert.True(t, foo1ValNonNil.IsValid())
	assert.True(t, foo1ValNonNil.CanSet())
	foo1ReferPointer := ReferenceValue(foo1ValNonNil, ptrDepth)
	assert.Equal(t, reflect.Ptr, foo1ReferPointer.Kind())
}

func TestGetFieldValue(t *testing.T) {
	type fooq struct {
		F1 **string
	}

	type bar struct {
		Bar **fooq
	}

	bar1 := (***bar)(nil)
	parentIdx := []int{0}
	idx := 0

	bar1Val := reflect.ValueOf(bar1)
	parentFieldVal := GetFieldValue(bar1Val, parentIdx)
	assert.NotEqual(t, reflect.Ptr, parentFieldVal)
	assert.True(t, parentFieldVal.CanSet())
	fooFieldVal := parentFieldVal.Field(idx)
	assert.Equal(t, "**string", fooFieldVal.Type().String())
	assert.True(t, fooFieldVal.CanSet())
}
