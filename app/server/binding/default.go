package binding

import (
	"bytes"
	stdJson "encoding/json"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"sync"

	exprValidator "github.com/bytedance/go-tagexpr/v2/validator"
	inDecoder "github.com/favbox/wind/app/server/binding/internal/decoder"
	wjson "github.com/favbox/wind/common/json"
	"github.com/favbox/wind/common/utils"
	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/consts"
	"github.com/favbox/wind/route/param"
	"google.golang.org/protobuf/proto"
)

const (
	pathTag            = "path"
	queryTag           = "query"
	headerTag          = "header"
	formTag            = "form"
	defaultValidateTag = "vd"
)

type decoderInfo struct {
	decoder      inDecoder.Decoder
	needValidate bool
}

var defaultBind = NewBinder(nil)

// DefaultBinder 返回默认的请求参数绑定器。
func DefaultBinder() Binder {
	return defaultBind
}

// NewBinder 创建给定配置的请求参数绑定器。
func NewBinder(config *BindConfig) Binder {
	if config == nil {
		bindConfig := NewBindConfig()
		bindConfig.initTypeUnmarshal()
		return &defaultBinder{
			config: bindConfig,
		}
	}

	config.initTypeUnmarshal()
	if config.Validator == nil {
		config.Validator = DefaultValidator()
	}
	return &defaultBinder{
		config: config,
	}
}

// BindAndValidate 将 *protocol.Request 的数据绑定到 obj，并按需验证。
// 注意：
//
//	obj 应为指针类型。
func BindAndValidate(req *protocol.Request, obj any, pathParams param.Params) error {
	return DefaultBinder().BindAndValidate(req, obj, pathParams)
}

// Bind 将 *protocol.Request 的数据绑定到 obj。
// 注意：
//
//	obj 应为指针类型。
func Bind(req *protocol.Request, obj any, pathParam param.Params) error {
	return DefaultBinder().Bind(req, obj, pathParam)
}

// Validate 使用 "vd" 标签来验证 obj。
// 注意：
//
//	obj 应为指针类型。
//	Validate 应在 Bind 之后调用。
func Validate(obj any) error {
	return DefaultValidator().ValidateStruct(obj)
}

type defaultBinder struct {
	config             *BindConfig
	decoderCache       sync.Map
	pathDecoderCache   sync.Map
	queryDecoderCache  sync.Map
	headerDecoderCache sync.Map
	formDecoderCache   sync.Map
}

func (b *defaultBinder) Name() string {
	return "wind"
}

func (b *defaultBinder) BindAndValidate(req *protocol.Request, v any, params param.Params) error {
	return b.bindTagAndValidate(req, v, params, "")
}

func (b *defaultBinder) Bind(req *protocol.Request, v any, params param.Params) error {
	return b.bindTag(req, v, params, "")
}

func (b *defaultBinder) BindPath(req *protocol.Request, v any, params param.Params) error {
	return b.bindTag(req, v, params, pathTag)
}

func (b *defaultBinder) BindQuery(req *protocol.Request, v any) error {
	return b.bindTag(req, v, nil, queryTag)
}

func (b *defaultBinder) BindHeader(req *protocol.Request, v any) error {
	return b.bindTag(req, v, nil, headerTag)
}

func (b *defaultBinder) BindForm(req *protocol.Request, v any) error {
	return b.bindTag(req, v, nil, formTag)
}

func (b *defaultBinder) BindJSON(req *protocol.Request, v any) error {
	return b.decodeJSON(bytes.NewReader(req.Body()), v)
}

func (b *defaultBinder) BindProtobuf(req *protocol.Request, v any) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("%s 未实现 'proto.Message'", v)
	}
	return proto.Unmarshal(req.Body(), msg)
}

func (b *defaultBinder) bindTag(req *protocol.Request, v any, params param.Params, tag string) error {
	rv, typeID := valueAndTypeID(v)
	if err := checkPointer(rv); err != nil {
		return err
	}
	rt := dereferPointer(rv)
	if rt.Kind() != reflect.Struct {
		return b.bindNonStruct(req, v)
	}

	err := b.preBindBody(req, v)
	if err != nil {
		return fmt.Errorf("绑定请求体失败，错误=%v", err)
	}

	cache := b.tagCache(tag)
	cached, ok := cache.Load(typeID)
	if ok {
		// 快速路径：已缓存的字段解码器
		decoder := cached.(decoderInfo)
		return decoder.decoder(req, params, rv.Elem())
	}

	validateTag := defaultValidateTag
	if len(b.config.Validator.ValidateTag()) != 0 {
		validateTag = b.config.Validator.ValidateTag()
	}
	decodeConfig := &inDecoder.DecodeConfig{
		LooseZeroMode:                      b.config.LooseZeroMode,
		DisableDefaultTag:                  b.config.DisableDefaultTag,
		DisableStructFieldResolve:          b.config.DisableStructFieldResolve,
		EnableDecoderUseNumber:             b.config.EnableDecoderUseNumber,
		EnableDecoderDisallowUnknownFields: b.config.EnableDecoderDisallowUnknownFields,
		ValidateTag:                        validateTag,
		TypeUnmarshalFuncs:                 b.config.TypeUnmarshalFuncs,
	}
	decoder, needValidate, err := inDecoder.GetReqDecoder(rv.Type(), tag, decodeConfig)
	if err != nil {
		return err
	}

	cache.Store(typeID, decoderInfo{decoder: decoder, needValidate: needValidate})
	return decoder(req, params, rv.Elem())
}

func (b *defaultBinder) bindTagAndValidate(req *protocol.Request, v any, params param.Params, tag string) error {
	rv, typeID := valueAndTypeID(v)

	// 确保接收器为非空指针
	if err := checkPointer(rv); err != nil {
		return err
	}

	// 将接收器
	rt := dereferPointer(rv)
	if rt.Kind() != reflect.Struct {
		return b.bindNonStruct(req, v)
	}

	err := b.preBindBody(req, v)
	if err != nil {
		return fmt.Errorf("绑定请求体失败，错误=%v", err)
	}

	cache := b.tagCache(tag)
	cached, ok := cache.Load(typeID)
	if ok {
		// 快速路径：已缓存的字段解码器
		decoder := cached.(decoderInfo)
		err = decoder.decoder(req, params, rv.Elem())
		if err != nil {
			return err
		}
		if decoder.needValidate {
			err = b.config.Validator.ValidateStruct(rv.Elem())
		}
		return err
	}

	validateTag := defaultValidateTag
	if len(b.config.Validator.ValidateTag()) != 0 {
		validateTag = b.config.Validator.ValidateTag()
	}
	decodeConfig := &inDecoder.DecodeConfig{
		LooseZeroMode:                      b.config.LooseZeroMode,
		DisableDefaultTag:                  b.config.DisableDefaultTag,
		DisableStructFieldResolve:          b.config.DisableStructFieldResolve,
		EnableDecoderUseNumber:             b.config.EnableDecoderUseNumber,
		EnableDecoderDisallowUnknownFields: b.config.EnableDecoderDisallowUnknownFields,
		ValidateTag:                        validateTag,
		TypeUnmarshalFuncs:                 b.config.TypeUnmarshalFuncs,
	}
	decoder, needValidate, err := inDecoder.GetReqDecoder(rv.Type(), tag, decodeConfig)
	if err != nil {
		return err
	}

	cache.Store(typeID, decoderInfo{decoder: decoder, needValidate: needValidate})
	err = decoder(req, params, rv.Elem())
	if err != nil {
		return err
	}
	if needValidate {
		err = b.config.Validator.ValidateStruct(rv.Elem())
	}
	return err
}

func (b *defaultBinder) bindNonStruct(req *protocol.Request, v any) (err error) {
	ct := bytesconv.B2s(req.Header.ContentType())
	switch utils.FilterContentType(ct) {
	case consts.MIMEApplicationJSON:
		err = wjson.Unmarshal(req.Body(), v)
	case consts.MIMEPROTOBUF:
		msg, ok := v.(proto.Message)
		if !ok {
			return fmt.Errorf("%s 未实现 'proto.Message'", v)
		}
		err = proto.Unmarshal(req.Body(), msg)
	case consts.MIMEMultipartPOSTForm:
		form := make(url.Values)
		mf, err1 := req.MultipartForm()
		if err1 == nil && mf.Value != nil {
			for k, v := range mf.Value {
				for _, vv := range v {
					form.Add(k, vv)
				}
			}
		}
		b, _ := stdJson.Marshal(form)
		err = wjson.Unmarshal(b, v)
	case consts.MIMEApplicationHTMLForm:
		form := make(url.Values)
		req.PostArgs().VisitAll(func(formKey, value []byte) {
			form.Add(string(formKey), string(value))
		})
		b, _ := stdJson.Marshal(form)
		err = wjson.Unmarshal(b, v)
	default:
		// 使用 query 来解码
		query := make(url.Values)
		req.URI().QueryArgs().VisitAll(func(queryKey, value []byte) {
			query.Add(string(queryKey), string(value))
		})
		b, _ := stdJson.Marshal(query)
		err = wjson.Unmarshal(b, v)
	}
	return
}

func (b *defaultBinder) preBindBody(req *protocol.Request, v any) error {
	if req.Header.ContentLength() <= 0 {
		return nil
	}
	ct := bytesconv.B2s(req.Header.ContentType())
	switch utils.FilterContentType(ct) {
	case consts.MIMEApplicationJSON, consts.MIMEApplicationJSONUTF8:
		return wjson.Unmarshal(req.Body(), v)
	case consts.MIMEPROTOBUF:
		msg, ok := v.(proto.Message)
		if !ok {
			return fmt.Errorf("%s 未实现 'proto.Message'", v)
		}
		return proto.Unmarshal(req.Body(), msg)
	default:
		return nil
	}
}

func (b *defaultBinder) tagCache(tag string) *sync.Map {
	switch tag {
	case pathTag:
		return &b.pathDecoderCache
	case queryTag:
		return &b.queryDecoderCache
	case headerTag:
		return &b.headerDecoderCache
	case formTag:
		return &b.formDecoderCache
	default:
		return &b.decoderCache
	}
}

func (b *defaultBinder) decodeJSON(r io.Reader, obj any) error {
	decoder := wjson.NewDecoder(r)
	if b.config.EnableDecoderUseNumber {
		decoder.UseNumber()
	}
	if b.config.EnableDecoderDisallowUnknownFields {
		decoder.DisallowUnknownFields()
	}

	return decoder.Decode(obj)
}

var defaultValidate = NewValidator(NewValidateConfig())

// NewValidator 创建给定配置的验证器。
func NewValidator(config *ValidateConfig) StructValidator {
	validateTag := defaultValidateTag
	if config != nil && len(config.ValidateTag) != 0 {
		validateTag = config.ValidateTag
	}
	vd := exprValidator.New(validateTag).SetErrorFactory(defaultValidateErrorFactory)
	if config != nil && config.ErrFactory != nil {
		vd.SetErrorFactory(config.ErrFactory)
	}
	return &validator{
		validateTag: validateTag,
		validate:    vd,
	}
}

// DefaultValidator 返回默认验证器。
func DefaultValidator() StructValidator {
	return defaultValidate
}

var _ StructValidator = (*validator)(nil)

type validator struct {
	validateTag string
	validate    *exprValidator.Validator
}

// ValidateStruct 可接收任何类型，但只处理结构体或结构体指针。
func (v *validator) ValidateStruct(obj any) error {
	if obj == nil {
		return nil
	}
	return v.validate.Validate(obj)
}

// Engine 返回底层验证器。
func (v *validator) Engine() any {
	return v.validate
}

// ValidateTag 返回验证标签。
func (v *validator) ValidateTag() string {
	return v.validateTag
}

// 验证错误
type validateError struct {
	FailPath, Msg string
}

// 实现错误接口
func (e *validateError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	return "无效参数：" + e.FailPath
}

func defaultValidateErrorFactory(failPath, msg string) error {
	return &validateError{
		FailPath: failPath,
		Msg:      msg,
	}
}
