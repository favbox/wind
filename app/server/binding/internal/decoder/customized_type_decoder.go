package decoder

import (
	"reflect"

	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/route/param"
)

// CustomizedDecodeFunc 是自定义的 *protocol.Request 解码函数
type CustomizedDecodeFunc func(req *protocol.Request, params param.Params, text string) (reflect.Value, error)

// 自定义的字段文本解码器
type customizedFieldTextDecoder struct {
	fieldInfo
	decodeFunc CustomizedDecodeFunc
}

func (d *customizedFieldTextDecoder) Decode(req *protocol.Request, params param.Params, refValue reflect.Value) error {
	var text string
	var exists bool
	var defaultValue string
	for _, tagInfo := range d.tagInfos {
		if tagInfo.Skip || tagInfo.Key == jsonTag || tagInfo.Key == fileNameTag {
			defaultValue = tagInfo.Default
			continue
		}
		text, exists = tagInfo.Getter(req, params, tagInfo.Value)
		defaultValue = tagInfo.Default
		if exists {
			break
		}
	}
	if !exists {
		return nil
	}
	if len(text) == 0 && len(defaultValue) != 0 {
		text = defaultValue
	}

	v, err := d.decodeFunc(req, params, text)
	if err != nil {
		return err
	}
	if !v.IsValid() {
		return nil
	}

	refValue = GetFieldValue(refValue, d.parentIndex)
	field := refValue.Field(d.index)
	if field.Kind() == reflect.Ptr {
		t := field.Type()
		var ptrDepth int
		for t.Kind() == reflect.Ptr {
			t = t.Elem()
			ptrDepth++
		}
		field.Set(ReferenceValue(v, ptrDepth))
		return nil
	}

	field.Set(v)
	return nil
}

// 获取自定义的字段解码器
func getCustomizedFieldDecoder(field reflect.StructField, index int, tagInfos []TagInfo, parentIdx []int, decodeFunc CustomizedDecodeFunc, config *DecodeConfig) ([]fieldDecoder, error) {
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

	return []fieldDecoder{&customizedFieldTextDecoder{
		fieldInfo: fieldInfo{
			index:       index,
			parentIndex: parentIdx,
			fieldName:   field.Name,
			tagInfos:    tagInfos,
			fieldType:   fieldType,
			config:      config,
		},
		decodeFunc: decodeFunc,
	}}, nil
}
