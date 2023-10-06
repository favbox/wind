package decoder

import (
	"fmt"
	"reflect"

	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/route/param"
)

type fileTypeDecoder struct {
	fieldInfo
	isRepeated bool
}

func (d *fileTypeDecoder) Decode(req *protocol.Request, params param.Params, refValue reflect.Value) error {
	fieldValue := GetFieldValue(refValue, d.parentIndex)
	field := fieldValue.Field(d.index)

	if d.isRepeated {
		return d.fileSliceDecode(req, params, refValue)
	}

	var fileName string
	// file_name > form > fieldName
	for _, tagInfo := range d.tagInfos {
		if tagInfo.Key == fileNameTag {
			fileName = tagInfo.Value
			break
		}
		if tagInfo.Key == formTag {
			fileName = tagInfo.Value
		}
	}
	if len(fileName) == 0 {
		fileName = d.fieldName
	}
	file, err := req.FormFile(fileName)
	if err != nil {
		return fmt.Errorf("无法获取文件 '%s'，错误：%v", fileName, err)
	}
	if field.Kind() == reflect.Ptr {
		t := field.Type()
		var ptrDepth int
		for t.Kind() == reflect.Ptr {
			t = t.Elem()
			ptrDepth++
		}
		v := reflect.New(t).Elem()
		v.Set(reflect.ValueOf(*file))
		field.Set(ReferenceValue(v, ptrDepth))
		return nil
	}

	// 非指针元素切片
	field.Set(reflect.ValueOf(*file))

	return nil
}

func (d *fileTypeDecoder) fileSliceDecode(req *protocol.Request, _ param.Params, refValue reflect.Value) error {
	fieldValue := GetFieldValue(refValue, d.parentIndex)
	field := fieldValue.Field(d.index)
	// 为了防止空指针，需要为其创建一个非空值
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

	var fileName string
	// file_name > form > fieldName
	for _, tagInfo := range d.tagInfos {
		if tagInfo.Key == fileNameTag {
			fileName = tagInfo.Value
			break
		}
		if tagInfo.Key == formTag {
			fileName = tagInfo.Value
		}
	}
	if len(fileName) == 0 {
		fileName = d.fieldName
	}
	multipartForm, err := req.MultipartForm()
	if err != nil {
		return fmt.Errorf("无法获取多部分表单信息，错误：%v", err)
	}
	files, exists := multipartForm.File[fileName]
	if !exists {
		return fmt.Errorf("文件 '%s' 不存在", fileName)
	}

	if field.Kind() == reflect.Array {
		if len(files) != field.Len() {
			return fmt.Errorf("文件 '%s' 的个数(%d) 与 %s 的长度(%d)不匹配", fileName, len(files), field.Type().String(), field.Len())
		}
	} else {
		// 切片需要足够的容量
		field = reflect.MakeSlice(field.Type(), len(files), len(files))
	}

	// 处理多个*的指针
	var ptrDepth int
	t := d.fieldType.Elem()
	elemKind := t.Kind()
	for elemKind == reflect.Ptr {
		t = t.Elem()
		elemKind = t.Kind()
		ptrDepth++
	}

	for idx, file := range files {
		v := reflect.New(t).Elem()
		v.Set(reflect.ValueOf(*file))
		field.Index(idx).Set(ReferenceValue(v, ptrDepth))
	}
	fieldValue.Field(d.index).Set(ReferenceValue(field, parentPtrDepth))

	return nil
}

func getMultipartFileDecoder(field reflect.StructField, index int, tagInfos []TagInfo, parentIdx []int, config *DecodeConfig) ([]fieldDecoder, error) {
	fieldType := field.Type
	for field.Type.Kind() == reflect.Ptr {
		fieldType = field.Type.Elem()
	}
	isRepeated := false
	if fieldType.Kind() == reflect.Array || fieldType.Kind() == reflect.Slice {
		isRepeated = true
	}

	return []fieldDecoder{&fileTypeDecoder{
		fieldInfo: fieldInfo{
			index:       index,
			parentIndex: parentIdx,
			fieldName:   field.Name,
			tagInfos:    tagInfos,
			fieldType:   fieldType,
			config:      config,
		},
		isRepeated: isRepeated,
	}}, nil
}
