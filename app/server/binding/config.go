package binding

import (
	"reflect"

	"github.com/favbox/wind/app/server/binding/internal/decoder"
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
	// 意为：解码器是否可将数字解析为 Number 类型而非 float64。
	// 用于 BindJSON()。
	//
	// 默认值：false，即不使用 Number，保持 float64。
	EnableDecoderUseNumber bool

	// 是否允许 JSON 解码器调用 DisallowUnknownFields 方法。
	//
	// 意为：当目标是结构体而输入与目标中非忽略的导出字段不匹配时，解码器是否返回错误。
	// 用于 BindJSON()。
	//
	// 默认值：false，即不禁用未知字段。
	EnableDecoderDisallowUnknownFields bool

	// 注册自定义类型的解码器。
	// time.Time 已默认注册。
	TypeUnmarshalFuncs map[reflect.Type]decoder.CustomizedDecodeFunc
}
