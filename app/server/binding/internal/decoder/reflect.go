package decoder

import "reflect"

// ReferenceValue 将 T 转为 *T，ptrDepth 为 '*' 的个数。
func ReferenceValue(v reflect.Value, ptrDepth int) reflect.Value {
	switch {
	case ptrDepth > 0:
		for ; ptrDepth > 0; ptrDepth-- {
			vv := reflect.New(v.Type())
			vv.Elem().Set(v)
			v = vv
		}
	case ptrDepth < 0:
		for ; ptrDepth < 0 && v.Kind() == reflect.Ptr; ptrDepth++ {
			v = v.Elem()
		}
	}
	return v
}

func GetNonNilReferenceValue(v reflect.Value) (reflect.Value, int) {
	var ptrDepth int
	t := v.Type()
	elemKind := t.Kind()
	for elemKind == reflect.Ptr {
		t = t.Elem()
		elemKind = t.Kind()
		ptrDepth++
	}
	val := reflect.New(t).Elem()
	return val, ptrDepth
}

func GetFieldValue(refValue reflect.Value, parentIndex []int) reflect.Value {
	// 为了防止 refValue -> (***bar)(nil) 为空指针，需要给一个默认值
	if refValue.Kind() == reflect.Ptr && refValue.IsNil() {
		nonNilVal, ptrDepth := GetNonNilReferenceValue(refValue)
		refValue = ReferenceValue(nonNilVal, ptrDepth)
	}
	for _, idx := range parentIndex {
		if refValue.Kind() == reflect.Ptr && refValue.IsNil() {
			nonNilValue, ptrDepth := GetNonNilReferenceValue(refValue)
			refValue.Set(ReferenceValue(nonNilValue, ptrDepth))
		}
		for refValue.Kind() == reflect.Ptr {
			refValue = refValue.Elem()
		}
		refValue = refValue.Field(idx)
	}

	// 为了防止父结构体也是空指针，需要创建一个非空 reflect.Value
	for refValue.Kind() == reflect.Ptr {
		if refValue.IsNil() {
			nonNilvalue, ptrDepth := GetNonNilReferenceValue(refValue)
			refValue.Set(ReferenceValue(nonNilvalue, ptrDepth))
		}
		refValue = refValue.Elem()
	}

	return refValue
}

func getElemType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	return t
}
