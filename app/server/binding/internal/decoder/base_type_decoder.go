package decoder

import (
	"fmt"
	"reflect"

	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/route/param"
)

// 字段信息
type fieldInfo struct {
	index       int           // 字段索引
	parentIndex []int         // 父级索引切片
	fieldName   string        // 字段名称
	tagInfos    []TagInfo     // 标签切片
	fieldType   reflect.Type  // 字段的反射类型
	config      *DecodeConfig // 解码配置
}

type baseTypeFieldTextDecoder struct {
	fieldInfo
	decoder TextDecoder
}

func (d *baseTypeFieldTextDecoder) Decode(req *protocol.Request, params param.Params, refValue reflect.Value) error {
	var err error
	var text string
	var exists bool
	var defaultValue string
	for _, tagInfo := range d.tagInfos {
		if tagInfo.Skip || tagInfo.Key == jsonTag || tagInfo.Key == fileNameTag {
			defaultValue = tagInfo.Default
			if tagInfo.Key == jsonTag {
				found := checkRequiredJSON(req, tagInfo)
				if found {
					err = nil
				} else {
					err = fmt.Errorf("'%s' 字段必填，但请求体无此参数 '%s'", d.fieldName, tagInfo.JSONName)
				}
			}
			continue
		}
		text, exists = tagInfo.Getter(req, params, tagInfo.Value)
		defaultValue = tagInfo.Default
		if exists {
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
	if len(text) == 0 && len(defaultValue) != 0 {
		text = defaultValue
	}
	if !exists && len(text) == 0 {
		return nil
	}

	// 获取父字段的非空值
	refValue = GetFieldValue(refValue, d.parentIndex)
	field := refValue.Field(d.index)
	if field.Kind() == reflect.Ptr {
		t := field.Type()
		var ptrDepth int
		for t.Kind() == reflect.Ptr {
			t = t.Elem()
			ptrDepth++
		}
		var vv reflect.Value
		vv, err := stringToValue(t, text, req, params, d.config)
		if err != nil {
			return err
		}
		field.Set(ReferenceValue(vv, ptrDepth))
		return nil
	}

	// 非指针元素
	err = d.decoder.UnmarshalString(text, field, d.config.LooseZeroMode)
	if err != nil {
		return fmt.Errorf("无法解码 '%s' 为 %s: %w", text, d.fieldType.Name(), err)
	}

	return nil
}

func getBaseTypeTextDecoder(field reflect.StructField, index int, tagInfos []TagInfo, parentIdx []int, config *DecodeConfig) ([]fieldDecoder, error) {
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

	textDecoder, err := SelectTextDecoder(fieldType)
	if err != nil {
		return nil, err
	}

	return []fieldDecoder{&baseTypeFieldTextDecoder{
		fieldInfo: fieldInfo{
			index:       index,
			parentIndex: parentIdx,
			fieldName:   field.Name,
			tagInfos:    tagInfos,
			fieldType:   fieldType,
			config:      config,
		},
		decoder: textDecoder,
	}}, nil
}
