package param

// Param 路由参数。 支持命名路由参数 ":" 和 通配路由参数 "*"
type Param struct {
	Key   string
	Value string
}

// Params 路由参数切片，有序。
type Params []Param

// Get 返回与 name 匹配的第一个参数的值。 若无匹配，则返回空白字符串。
func (ps Params) Get(name string) (string, bool) {
	for _, entry := range ps {
		if entry.Key == name {
			return entry.Value, true
		}
	}
	return "", false
}

// ByName 返回与 name 匹配的第一个参数的值。若无匹配，则返回空白字符串。
func (ps Params) ByName(name string) string {
	v, _ := ps.Get(name)
	return v
}
