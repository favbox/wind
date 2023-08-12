package registry

import "net"

const DefaultWeight = 10

// NoopRegistry 无操作的服务注册实现。
var NoopRegistry Registry = &noopRegistry{}

// Registry 定义了服务注册所需实现的接口。
type Registry interface {
	Register(info *Info) error
	Deregister(info *Info) error
}

// Info 用于服务注册的信息。
type Info struct {
	ServiceName string            // 在 wind 中会被默认设置
	Addr        net.Addr          // 在 wind 中会被默认设置
	Weight      int               // 在 wind 中会被默认设置
	Tags        map[string]string // 其他扩展信息
}

// 无操作的服务注册实现。
type noopRegistry struct{}

func (n noopRegistry) Register(info *Info) error {
	return nil
}

func (n noopRegistry) Deregister(info *Info) error {
	return nil
}
