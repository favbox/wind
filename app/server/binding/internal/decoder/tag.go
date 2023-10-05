package decoder

import (
	"reflect"
	"strings"
)

const (
	pathTag     = "path"
	formTag     = "form"
	queryTag    = "query"
	cookieTag   = "cookie"
	headerTag   = "header"
	jsonTag     = "json"
	rawBodyTag  = "raw_body"
	fileNameTag = "file_name"
)

const (
	defaultTag = "default" // 默认值标签
)

const (
	requiredTagOpt = "required" // 必填标签操作符
)

type TagInfo struct {
	Key         string
	Value       string
	JSONName    string
	Required    bool
	Skip        bool
	Default     string
	Options     []string
	Getter      getter
	SliceGetter sliceGetter
}

// 返回将 str 按指定 sep 分割后的头部和尾部。
func head(str, sep string) (head, tail string) {
	idx := strings.Index(str, sep)
	if idx < 0 {
		return str, ""
	}
	return str[:idx], str[idx+len(sep):]
}

// 查找并返回指定字段的所有标签信息、新的父级json名称路径及是否需要验证。
func lookupFieldTags(field reflect.StructField, parentJSONName string, config *DecodeConfig) ([]TagInfo, string, bool) {
	var ret []string
	var needValidate bool
	if _, ok := field.Tag.Lookup(config.ValidateTag); ok {
		needValidate = true
	}
	tags := []string{pathTag, formTag, queryTag, cookieTag, headerTag, jsonTag, rawBodyTag, fileNameTag}
	for _, tag := range tags {
		if _, ok := field.Tag.Lookup(tag); ok {
			ret = append(ret, tag)
		}
	}

	defaultValue := ""
	if val, ok := field.Tag.Lookup(defaultTag); ok {
		defaultValue = val
	}

	var tagInfos []TagInfo
	var newParentJSONName string
	for _, tag := range ret {
		tagContent := field.Tag.Get(tag)
		tagValue, opts := head(tagContent, ",")
		if len(tagValue) == 0 {
			tagValue = field.Name
		}
		skip := false
		jsonName := ""
		if tag == jsonTag {
			jsonName = parentJSONName + "." + tagValue
		}
		if tagValue == "-" {
			skip = true
			if tag == jsonTag {
				jsonName = parentJSONName + "." + field.Name
			}
		}
		if jsonName != "" {
			jsonName = strings.TrimPrefix(jsonName, ".")
			newParentJSONName = jsonName
		}
		var options []string
		var opt string
		var required bool
		for len(opts) > 0 {
			opt, opts = head(opts, ",")
			options = append(options, opt)
			if opt == requiredTagOpt {
				required = true
			}
		}
		tagInfos = append(tagInfos, TagInfo{
			Key:      tag,
			Value:    tagValue,
			JSONName: jsonName,
			Required: required,
			Skip:     skip,
			Default:  defaultValue,
			Options:  options,
		})
	}
	if len(newParentJSONName) == 0 {
		newParentJSONName = strings.TrimPrefix(parentJSONName+"."+field.Name, ".")
	}

	return tagInfos, newParentJSONName, needValidate
}

// 获取字段的默认标签。
func getDefaultFieldTags(field reflect.StructField) (tagInfos []TagInfo) {
	defaultVal := ""
	if val, ok := field.Tag.Lookup(defaultTag); ok {
		defaultVal = val
	}

	tags := []string{pathTag, formTag, queryTag, cookieTag, headerTag, jsonTag, fileNameTag}
	for _, tag := range tags {
		tagInfos = append(tagInfos, TagInfo{Key: tag, Value: field.Name, Default: defaultVal})
	}

	return
}

func getFieldTagInfoByTag(field reflect.StructField, tag string) []TagInfo {
	var tagInfos []TagInfo
	if content, ok := field.Tag.Lookup(tag); ok {
		tagValue, opts := head(content, ",")
		if len(tagValue) == 0 {
			tagValue = field.Name
		}
		skip := false
		if tagValue == "-" {
			skip = true
		}
		var options []string
		var opt string
		var required bool
		for len(opts) > 0 {
			opt, opts = head(opts, ",")
			options = append(options, opt)
			if opt == requiredTagOpt {
				required = true
			}
		}
		tagInfos = append(tagInfos, TagInfo{Key: tag, Value: tagValue, Options: options, Required: required, Skip: skip})
	} else {
		tagInfos = append(tagInfos, TagInfo{Key: tag, Value: field.Name})
	}

	return tagInfos
}
