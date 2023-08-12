// Package nocopy 定义禁止拷贝结构体。
package nocopy

// NoCopy 定义禁止拷贝结构体。
type NoCopy struct{}

func (*NoCopy) Lock()   {}
func (*NoCopy) Unlock() {}
