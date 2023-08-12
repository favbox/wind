package server

import "github.com/favbox/wind/app/server/registry"

var _ registry.Registry = (*MockRegistry)(nil)

// MockRegistry 模拟实现 registry.Registry。
type MockRegistry struct {
	RegisterFunc   func(info *registry.Info) error
	DeregisterFunc func(info *registry.Info) error
}

func (m MockRegistry) Register(info *registry.Info) error {
	if m.RegisterFunc != nil {
		return m.RegisterFunc(info)
	}
	return nil
}

func (m MockRegistry) Deregister(info *registry.Info) error {
	if m.DeregisterFunc != nil {
		return m.DeregisterFunc(info)
	}
	return nil
}
