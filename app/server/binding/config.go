package binding

import (
	stdJson "encoding/json"
	"fmt"
	"reflect"
	"time"

	exprValidator "github.com/bytedance/go-tagexpr/v2/validator"
	"github.com/favbox/wind/app/server/binding/internal/decoder"
	wjson "github.com/favbox/wind/common/json"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/route/param"
)

// BindConfig 包含默认绑定行为的可选项。
type BindConfig struct {
	// 是否为松散的零值模式。
	//
	// 意为：若设为 true 则空字符串请求参数也会绑定到参数的零值。
	// 适用于如下参数类型：query/header/cookie/form。
	//
	// 默认值：false，非松散零值模式。
	LooseZeroMode bool

	// 是否禁用默认标签。
	//
	// 意为：当字段未指定标签时是否添加默认的标签，以实现自动化绑定（会带来额外开销）。
	//
	// 默认值：false，不禁用默认标签。
	DisableDefaultTag bool

	// 是否禁用结构体字段编码器。
	//
	// 亦即：若为 false 则结构体字段将获得单独的 inDecoder.structTypeFieldTextDecoder 并用 json.Unmarshal 进行解码。
	// 常用于将 json 字段添加到查询参数中。
	//
	// 默认值：false，使用独立的结构体字段文本解码器。
	DisableStructFieldResolve bool

	// 是否允许 JSON 解码器调用 UseNumber 方法。
	//
	// 意为：开启后，解码器会将 float64 解析为 json.Number。
	// 用于 BindJSON()。
	//
	// 默认值：false，即不使用 Number，保持 float64。
	EnableDecoderUseNumber bool

	// 是否允许 JSON 解码器调用 DisallowUnknownFields 方法。
	//
	// 意为：开启后，若解码后的字段不是目标结构体定义的就报错。
	// 用于 BindJSON()。
	//
	// 默认值：false，即不禁用未知字段。
	EnableDecoderDisallowUnknownFields bool

	// 注册自定义类型的解码器。
	TypeUnmarshalFuncs map[reflect.Type]decoder.CustomizedDecodeFunc
	// 用于 BindAndValidate() 的验证。
	Validator StructValidator
}

// RegTypeUnmarshal 注册自定义类型解码器。
func (c *BindConfig) RegTypeUnmarshal(t reflect.Type, fn decoder.CustomizedDecodeFunc) error {
	switch t.Kind() {
	case reflect.Bool,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Float32, reflect.Float64,
		reflect.String:
		return fmt.Errorf("注册类型不能是基础类型")
	case reflect.Ptr:
		return fmt.Errorf("注册类型不能是指针类型")
	}
	if c.TypeUnmarshalFuncs == nil {
		c.TypeUnmarshalFuncs = make(map[reflect.Type]decoder.CustomizedDecodeFunc)
	}
	c.TypeUnmarshalFuncs[t] = fn
	return nil
}

// MustRegTypeUnmarshal 注册自定义类型解码器。若出错则会恐慌。
func (c *BindConfig) MustRegTypeUnmarshal(t reflect.Type, fn decoder.CustomizedDecodeFunc) {
	err := c.RegTypeUnmarshal(t, fn)
	if err != nil {
		panic(err)
	}
}

// 初始化默认的类型解码器(如 time.Time)。
func (c *BindConfig) initTypeUnmarshal() {
	c.MustRegTypeUnmarshal(reflect.TypeOf(time.Time{}), func(req *protocol.Request, params param.Params, text string) (reflect.Value, error) {
		if text == "" {
			return reflect.ValueOf(time.Time{}), nil
		}
		t, err := time.Parse(time.RFC3339, text)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(t), nil
	})
}

// UseThirdPartyJSONUnmarshaler 使用第三方 json 库进行请求参数绑定。
// 备注：
//
//	一经调用，将持续生效。
func (c *BindConfig) UseThirdPartyJSONUnmarshaler(fn func(data []byte, v any) error) {
	wjson.Unmarshal = fn
}

// UseStdJSONUnmarshaler 使用 encoding/json 作为 json 库。
// 注意：
//
//	单签版本默认使用 encoding/json。
//	一经调用，将持续生效。
func (c *BindConfig) UseStdJSONUnmarshaler() {
	c.UseThirdPartyJSONUnmarshaler(stdJson.Unmarshal)
}

// NewBindConfig 创建新的绑定配置。
func NewBindConfig() *BindConfig {
	return &BindConfig{
		LooseZeroMode:                      false,
		DisableDefaultTag:                  false,
		DisableStructFieldResolve:          false,
		EnableDecoderUseNumber:             false,
		EnableDecoderDisallowUnknownFields: false,
		TypeUnmarshalFuncs:                 make(map[reflect.Type]decoder.CustomizedDecodeFunc),
		Validator:                          defaultValidate,
	}
}

// ValidateErrFactory 是验证错误的工厂函数。
type ValidateErrFactory func(fieldSelector, msg string) error

// ValidateConfig 包含验证行为的可选项。
type ValidateConfig struct {
	ValidateTag string             // 验证标签，支持自定义
	ErrFactory  ValidateErrFactory // 自定义的错误处理函数
}

// MustRegValidateFunc 注册验证函数表达式。
// 注意：
//
//	若 force=true，则覆盖已存在的同名函数。
//	一经调用，将持续生效。
func (c *ValidateConfig) MustRegValidateFunc(funcName string, fn func(args ...any) error, force ...bool) {
	exprValidator.MustRegFunc(funcName, fn, force...)
}

// SetValidatorErrorFactory 自定义验证器的错误工厂函数。
func (c *ValidateConfig) SetValidatorErrorFactory(errFactory ValidateErrFactory) {
	c.ErrFactory = errFactory
}

// SetValidatorTag 自定义验证器的标签。
func (c *ValidateConfig) SetValidatorTag(tag string) {
	c.ValidateTag = tag
}

// NewValidateConfig 创建新的验证配置。
func NewValidateConfig() *ValidateConfig {
	return &ValidateConfig{}
}
