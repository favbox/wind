// Package decoder 包含了内置解码器。
package decoder

import (
	"fmt"
	"mime/multipart"
	"reflect"

	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/route/param"
)

// 定义了字段解码器需要实现的解码函数签名。
type fieldDecoder interface {
	Decode(req *protocol.Request, params param.Params, refValue reflect.Value) error
}

// Decoder 是请求的解码器。
type Decoder func(req *protocol.Request, params param.Params, rv reflect.Value) error

// DecodeConfig 是请求的解码配置项。
type DecodeConfig struct {
	LooseZeroMode                      bool                                  // 不用松散的零值
	DisableDefaultTag                  bool                                  // 禁止生成默认标签
	DisableStructFieldResolve          bool                                  // 禁用结构体字段解析
	EnableDecoderUseNumber             bool                                  // 将 float64 转为 Number
	EnableDecoderDisallowUnknownFields bool                                  // 有未知不匹配字段则报错
	ValidateTag                        string                                // 验证标签
	TypeUnmarshalFuncs                 map[reflect.Type]CustomizedDecodeFunc // 自定义类型解码函数
}

// GetReqDecoder 获取请求的解码器。
func GetReqDecoder(rt reflect.Type, byTag string, config *DecodeConfig) (Decoder, bool, error) {
	var decoders []fieldDecoder
	var needValidate bool

	el := rt.Elem()
	if el.Kind() != reflect.Struct {
		return nil, false, fmt.Errorf("不支持 %s 类型绑定", rt.String())
	}

	for i := 0; i < el.NumField(); i++ {
		if el.Field(i).PkgPath != "" && !el.Field(i).Anonymous {
			// 忽略未导出字段
			continue
		}

		// dec, needValidate2, err := getFieldDecoder(el.Field(i), i, []int{}, "", byTag, config)
		dec, needValidate2, err := getFieldDecoder(parentInfos{[]reflect.Type{el}, []int{}, ""}, el.Field(i), i, byTag, config)

		if err != nil {
			return nil, false, err
		}
		needValidate = needValidate || needValidate2

		if dec != nil {
			decoders = append(decoders, dec...)
		}
	}

	return func(req *protocol.Request, params param.Params, rv reflect.Value) error {
		for _, decoder := range decoders {
			err := decoder.Decode(req, params, rv)
			if err != nil {
				return err
			}
		}

		return nil
	}, needValidate, nil
}

type parentInfos struct {
	Types    []reflect.Type
	Indexes  []int
	JSONName string
}

// func getFieldDecoder(field reflect.StructField, index int, parentIdx []int, parentJSONName, byTag string, config *DecodeConfig) ([]fieldDecoder, bool, error) {
func getFieldDecoder(pInfo parentInfos, field reflect.StructField, index int, byTag string, config *DecodeConfig) ([]fieldDecoder, bool, error) {
	for field.Type.Kind() == reflect.Ptr {
		field.Type = field.Type.Elem()
	}
	// 跳过匿名定义，如：
	//type A struct {
	//	string
	//}
	if field.Type.Kind() != reflect.Struct && field.Anonymous {
		return nil, false, nil
	}

	// 形如 'a.b.c' 的 JSONName 用于必填验证。
	fieldTagInfos, newParentJSONName, needValidate := lookupFieldTags(field, pInfo.JSONName, config)
	if len(fieldTagInfos) == 0 && !config.DisableDefaultTag {
		fieldTagInfos = getDefaultFieldTags(field)
	}
	if len(byTag) != 0 {
		fieldTagInfos = getFieldTagInfoByTag(field, byTag)
	}

	// 自定义类型解码器拥有最高优先级
	if customizedFunc, exists := config.TypeUnmarshalFuncs[field.Type]; exists {
		dec, err := getCustomizedFieldDecoder(field, index, fieldTagInfos, pInfo.Indexes, customizedFunc, config)
		return dec, needValidate, err
	}

	// 切片、数组字段解码器
	if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array {
		dec, err := getSliceFieldDecoder(field, index, fieldTagInfos, pInfo.Indexes, config)
		return dec, needValidate, err
	}

	// 映射字段解码器
	if field.Type.Kind() == reflect.Map {
		dec, err := getMapFieldDecoder(field, index, fieldTagInfos, pInfo.Indexes, config)
		return dec, needValidate, err
	}

	// 结构体字段将被递归解析
	if field.Type.Kind() == reflect.Struct {
		var decoders []fieldDecoder
		el := field.Type
		// todo 更多内置常用结构体绑定，如时间...
		switch el {
		case reflect.TypeOf(multipart.FileHeader{}):
			dec, err := getMultipartFileDecoder(field, index, fieldTagInfos, pInfo.Indexes, config)
			return dec, needValidate, err
		}
		if !config.DisableStructFieldResolve { // 单独解码结构体类型
			structFieldDecoder, err := getStructTypeFieldDecoder(field, index, fieldTagInfos, pInfo.Indexes, config)
			if err != nil {
				return nil, needValidate, err
			}
			if structFieldDecoder != nil {
				decoders = append(decoders, structFieldDecoder...)
			}
		}

		// 防止无限递归：结构体的字段类型与父结构体相同时会产生。
		if hasSameType(pInfo.Types, el) {
			return decoders, needValidate, nil
		}

		pIdx := pInfo.Indexes
		for i := 0; i < el.NumField(); i++ {
			if el.Field(i).PkgPath != "" && !el.Field(i).Anonymous {
				// 忽略未导出字段
				continue
			}
			var indices []int
			if len(pInfo.Indexes) > 0 {
				indices = append(indices, pIdx...)
			}
			indices = append(indices, index)
			pInfo.Indexes = indices
			pInfo.Types = append(pInfo.Types, el)
			pInfo.JSONName = newParentJSONName
			dec, needValidate2, err := getFieldDecoder(pInfo, el.Field(i), i, byTag, config)
			needValidate = needValidate || needValidate2
			if err != nil {
				return nil, false, err
			}
			if dec != nil {
				decoders = append(decoders, dec...)
			}
		}
		return decoders, needValidate, nil
	}

	// 基本类型解码器
	dec, err := getBaseTypeTextDecoder(field, index, fieldTagInfos, pInfo.Indexes, config)
	return dec, needValidate, err
}

// hasSameType 确定父子关系中是否存在相同类型
func hasSameType(pts []reflect.Type, ft reflect.Type) bool {
	for _, pt := range pts {
		if reflect.DeepEqual(getElemType(pt), getElemType(ft)) {
			return true
		}
	}
	return false
}
