package binding

// StructValidator 表示一个请求参数的结构体验证器接口。
type StructValidator interface {
	ValidateStruct(any) error // 可接收任何类型，但只处理结构体或结构体指针。
	Engine() any              // 返回底层验证器
	ValidateTag() string      // 返回验证标签
}
