package binding

// StructValidator 表示一个结构体验证器。
type StructValidator interface {
	ValidateStruct(any) error
	Engine() any
	ValidateTag() string
}
