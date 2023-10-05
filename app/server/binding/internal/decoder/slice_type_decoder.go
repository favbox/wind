package decoder

import (
	"fmt"
	"mime/multipart"
	"reflect"

	wjson "github.com/favbox/wind/common/json"
	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/route/param"
)

type sliceTypeFieldTextDecoder struct {
	fieldInfo
	isArray bool
}

func (d *sliceTypeFieldTextDecoder) Decode(req *protocol.Request, params param.Params, refValue reflect.Value) error {
	var err error
	var texts []string
	var defaultValue string
	var bindRawBody bool
	for _, tagInfo := range d.tagInfos {
		if tagInfo.Skip || tagInfo.Key == jsonTag || tagInfo.Key == fileNameTag {
			defaultValue = tagInfo.Default
			if tagInfo.Key == jsonTag {
				found := checkRequiredJSON(req, tagInfo)
				if found {
					err = nil
				} else {
					err = fmt.Errorf("'%s' 字段必填，但请求无此参数", d.fieldName)
				}
			}
			continue
		}
		if tagInfo.Key == rawBodyTag {
			bindRawBody = true
		}
		texts = tagInfo.SliceGetter(req, params, tagInfo.Value)
		defaultValue = tagInfo.Default
		if len(texts) != 0 {
			err = nil
			break
		}
		if tagInfo.Required {
			err = fmt.Errorf("'%s' 字段必填，但请求无此参数", d.fieldName)
		}
	}
	if err != nil {
		return err
	}
	if len(texts) == 0 && len(defaultValue) != 0 {
		texts = append(texts, defaultValue)
	}
	if len(texts) == 0 {
		return nil
	}

	refValue = GetFieldValue(refValue, d.parentIndex)
	field := refValue.Field(d.index)
	// **[]**int
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			nonNilVal, ptrDepth := GetNonNilReferenceValue(field)
			field.Set(ReferenceValue(nonNilVal, ptrDepth))
		}
	}
	var parentPtrDepth int
	for field.Kind() == reflect.Ptr {
		field = field.Elem()
		parentPtrDepth++
	}

	if d.isArray {
		if len(texts) != field.Len() {
			return fmt.Errorf("%q 对于 %s 不是有效的值。", texts, field.Type().String())
		}
	} else {
		// 切片需要足够的容量
		field = reflect.MakeSlice(field.Type(), len(texts), len(texts))
	}
	// raw_body && []byte 绑定
	if bindRawBody && field.Type().Elem().Kind() == reflect.Uint8 {
		refValue.Field(d.index).Set(reflect.ValueOf(req.Body()))
		return nil
	}

	// 处理内部多指针，[]**int
	var ptrDepth int
	t := d.fieldType.Elem() // d.fieldType 为字段的非指针类型
	elemKind := t.Kind()
	for elemKind == reflect.Ptr {
		t = t.Elem()
		elemKind = t.Kind()
		ptrDepth++
	}

	for idx, text := range texts {
		var vv reflect.Value
		vv, err = stringToValue(t, text, req, params, d.config)
		if err != nil {
			break
		}
		field.Index(idx).Set(ReferenceValue(vv, ptrDepth))
	}
	if err != nil {
		if !refValue.Field(d.index).CanAddr() {
			return err
		}
		// texts[0] 是可用于 []Type 的完整json内容。
		err = wjson.Unmarshal(bytesconv.S2b(texts[0]), refValue.Field(d.index).Addr().Interface())
		if err != nil {
			return fmt.Errorf("使用 '%s' 解码字段 '%s: %s' 失败，%v", texts[0], d.fieldName, d.fieldType.String(), err)
		}
	} else {
		refValue.Field(d.index).Set(ReferenceValue(field, parentPtrDepth))
	}

	return nil
}

// 将字符文本转为真实反射类型的值。
func stringToValue(elemType reflect.Type, text string, req *protocol.Request, params param.Params, config *DecodeConfig) (v reflect.Value, err error) {
	v = reflect.New(elemType).Elem()
	if customizedFunc, exists := config.TypeUnmarshalFuncs[elemType]; exists {
		if val, e := customizedFunc(req, params, text); e != nil {
			return reflect.Value{}, e
		} else {
			return val, nil
		}
	}

	switch elemType.Kind() {
	case reflect.Struct:
		err = wjson.Unmarshal(bytesconv.S2b(text), v.Addr().Interface())
	case reflect.Map:
		err = wjson.Unmarshal(bytesconv.S2b(text), v.Addr().Interface())
	case reflect.Array, reflect.Slice:
	// 啥也不用做
	default:
		decoder, err := SelectTextDecoder(elemType)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("切片/数组所不支持的类型 %s", elemType.String())
		}
		err = decoder.UnmarshalString(text, v, config.LooseZeroMode)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("无法解码 '%s' 为 %s: %w", text, elemType.String(), err)
		}
	}

	return v, err
}

func getSliceFieldDecoder(field reflect.StructField, index int, tagInfos []TagInfo, parentIdx []int, config *DecodeConfig) ([]fieldDecoder, error) {
	if !(field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array) {
		return nil, fmt.Errorf("不支持的类型 %s，期望切片或数组", field.Type.String())
	}

	isArray := false
	if field.Type.Kind() == reflect.Array {
		isArray = true
	}
	for idx, tagInfo := range tagInfos {
		switch tagInfo.Key {
		case pathTag:
			tagInfos[idx].SliceGetter = pathSlice
			tagInfos[idx].Getter = path
		case formTag:
			tagInfos[idx].SliceGetter = postFormSlice
			tagInfos[idx].Getter = postForm
		case queryTag:
			tagInfos[idx].SliceGetter = querySlice
			tagInfos[idx].Getter = query
		case cookieTag:
			tagInfos[idx].SliceGetter = cookieSlice
			tagInfos[idx].Getter = cookie
		case headerTag:
			tagInfos[idx].SliceGetter = headerSlice
			tagInfos[idx].Getter = header
		case jsonTag:
			// 啥也不用做
		case rawBodyTag:
			tagInfos[idx].SliceGetter = rawBodySlice
			tagInfos[idx].Getter = rawBody
		case fileNameTag:
			// 啥也不用做
		default:
			// 啥也不用做
		}
	}

	fieldType := field.Type
	for field.Type.Kind() == reflect.Ptr {
		fieldType = field.Type.Elem()
	}
	// fieldType.Element() 是数组/切片元素的类型
	t := getElemType(fieldType.Elem())
	if t == reflect.TypeOf(multipart.FileHeader{}) {
		return getMultipartFileDecoder(field, index, tagInfos, parentIdx, config)
	}

	return []fieldDecoder{&sliceTypeFieldTextDecoder{
		fieldInfo: fieldInfo{
			index:       index,
			parentIndex: parentIdx,
			fieldName:   field.Name,
			tagInfos:    tagInfos,
			fieldType:   fieldType,
			config:      config,
		},
		isArray: isArray,
	}}, nil
}
