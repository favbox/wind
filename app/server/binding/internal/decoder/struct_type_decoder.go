package decoder

import (
	"fmt"
	"reflect"

	wjson "github.com/favbox/wind/common/json"
	"github.com/favbox/wind/common/wlog"
	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/route/param"
)

type structTypeFieldTextDecoder struct {
	fieldInfo
}

func (d *structTypeFieldTextDecoder) Decode(req *protocol.Request, params param.Params, refValue reflect.Value) error {
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
					err = fmt.Errorf("'%s' 字段必填，但请求无此参数", d.fieldName)
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
			wlog.Infof("无法解码 '%s' 为 %s：%v，但可能不影响正确性，故跳过", text, d.fieldType.Name(), err)
			return nil
		}
		field.Set(ReferenceValue(vv, ptrDepth))
		return nil
	}

	err = wjson.Unmarshal(bytesconv.S2b(text), field.Addr().Interface())
	if err != nil {
		wlog.Infof("无法解码 '%s' 为 %s：%v，但可能不影响正确性，故跳过", text, d.fieldType.Name(), err)
	}

	return nil
}

func getStructTypeFieldDecoder(field reflect.StructField, index int, tagInfos []TagInfo, parentIdx []int, config *DecodeConfig) ([]fieldDecoder, error) {
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
			// 啥也不用干
		case rawBodyTag:
			tagInfos[idx].SliceGetter = rawBodySlice
			tagInfos[idx].Getter = rawBody
		case fileNameTag:
			// 啥也不用干
		default:
			// 啥也不用干
		}
	}

	fieldType := field.Type
	for field.Type.Kind() == reflect.Ptr {
		fieldType = field.Type.Elem()
	}

	return []fieldDecoder{&structTypeFieldTextDecoder{
		fieldInfo: fieldInfo{
			index:       index,
			parentIndex: parentIdx,
			fieldName:   field.Name,
			tagInfos:    tagInfos,
			fieldType:   fieldType,
			config:      config,
		},
	}}, nil
}
