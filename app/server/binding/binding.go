package binding

import (
	"encoding/json"
	"reflect"

	"github.com/bytedance/go-tagexpr/v2/binding"
	"github.com/bytedance/go-tagexpr/v2/binding/gjson"
	"github.com/bytedance/go-tagexpr/v2/validator"
	hjson "github.com/favbox/wind/common/json"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/route/param"
)

func init() {
	binding.ResetJSONUnmarshaler(hjson.Unmarshal)
}

var defaultBinder = binding.Default()

// BindAndValidate 绑定请求数据 req 至 obj 并按需验证。
//
// 注意：obj 应为一个指针。
func BindAndValidate(req *protocol.Request, obj any, pathParams param.Params) error {
	return defaultBinder.IBindAndValidate(obj, wrapRequest(req), pathParams)
}

// Bind 绑定请求数据 req 至 obj。
//
// 注意：obj 应为一个指针。
func Bind(req *protocol.Request, obj any, pathParams param.Params) error {
	return defaultBinder.IBind(obj, wrapRequest(req), pathParams)
}

// Validate 用 "vd" 标签验证 obj。
// 注意：
//
//	obj 应为一个指针。
//	验证应在 Bind 之后再调用。
func Validate(obj any) error {
	return defaultBinder.Validate(obj)
}

// SetLooseZeroMode 设置宽松零值模式。若设为 true，空字符串请求参数将绑定到参数的零值。
// 注意：
//
//	默认值为 false;
//	适应于这些参数类型：query/header/cookie/form。
func SetLooseZeroMode(enable bool) {
	defaultBinder.SetLooseZeroMode(enable)
}

// SetErrorFactory 设置绑定错误和验证错误的处理工厂。
// 注意：若工厂为空，则使用默认工厂。
func SetErrorFactory(bindErrFactory, validatingErrFactory func(failField, msg string) error) {
	defaultBinder.SetErrorFactory(bindErrFactory, validatingErrFactory)
}

// MustRegTypeUnmarshal 注册类型的解码函数。
// 注意：
//
//	若出错则会触发恐慌。
//	一旦调用，将持续生效。
func MustRegTypeUnmarshal(t reflect.Type, fn func(v string, emptyAsZero bool) (reflect.Value, error)) {
	binding.MustRegTypeUnmarshal(t, fn)
}

// MustRegValidateFunc 注册验证函数。
// 注意：
//
//	若 force=true 则允许覆盖已存在的同名函数。
//	一旦调用，将持续生效。
func MustRegValidateFunc(funcName string, fn func(args ...interface{}) error, force ...bool) {
	validator.RegFunc(funcName, fn, force...)
}

// UseStdJSONUnmarshaler 使用 encoding/json 作为 json 库
// 注意：
//
//	当前版本默认使用 encoding/json。
//	一旦调用，将持续生效。
func UseStdJSONUnmarshaler() {
	binding.ResetJSONUnmarshaler(json.Unmarshal)
}

// UseGJSONUnmarshaler uses github.com/bytedance/go-tagexpr/v2/binding/gjson 作为 json 库
// 注意：
//
//	一旦调用，将持续生效。
func UseGJSONUnmarshaler() {
	gjson.UseJSONUnmarshaler()
}

// UseThirdPartyJSONUnmarshaler 使用第三方 json 库用于绑定。
// 注意：
//
//	一旦调用，将持续生效。
func UseThirdPartyJSONUnmarshaler(unmarshaler func(data []byte, v interface{}) error) {
	binding.ResetJSONUnmarshaler(unmarshaler)
}
