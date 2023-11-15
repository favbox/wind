package config

import (
	"time"

	"github.com/favbox/wind/protocol/consts"
)

type Config struct {
	DisableKeepalive bool          // 是否禁用长连接，默认否
	EnableTrace      bool          // 是否启用链路追踪
	ReadTimeout      time.Duration // 读取正文的超时时长

	// 多路复用：限制每个连接上同时运行 http.Handler ServeHTTP 的协程数。
	// 负数或零意为不限制。
	// TODO: 待实现
	MaxHandlers int

	// 指定每个客户端同时可打开的并发流的数量。
	// 与 MaxHandlers 无关。
	// 若为零，则根据 HTTP/2 规范的建议默认为 100个。
	MaxConcurrentStreams uint32

	// 指定服务器将读取的最大帧大小。有效值的范围是[16K,16M]。若为零，则使用默认值。
	MaxReadFrameSize uint32

	// 是否允许禁止使用 HTTP/2 规范的密码套件。
	PermitProhibitedCipherSuites bool

	// IdleTime 指定了空闲客户端在多长时间内应使用 GOAWAY 帧关闭。
	// PING 帧不被视为 IdleTimeout 的活动。
	IdleTimeout time.Duration

	// 是每个连接的初始流控制窗口的大小。
	// HTTP/2 规范不允许该值小于 65535 或大于 2^32-1。若超限则使用默认值。
	MaxUploadBufferPerConnection int32

	// 是每个流的初始流窗口的大小。
	// HTTP/2 规范不允许该值大于 2^32-1。若超限则使用默认值。
	MaxUploadBufferPerStream int32
}

// Option 用于设置 HTTP2 Config 的唯一结构体。
type Option struct {
	F func(o *Config)
}

func (o *Config) Apply(opts []Option) {
	for _, opt := range opts {
		opt.F(o)
	}
}

// WithReadTimeout 用于设置读取正文的超时时长。
func WithReadTimeout(t time.Duration) Option {
	return Option{F: func(o *Config) {
		o.ReadTimeout = t
	}}
}

// WithDisableKeepalive 用于设置是否禁用长连接。默认 false。
func WithDisableKeepalive(b bool) Option {
	return Option{F: func(o *Config) {
		o.DisableKeepalive = b
	}}
}

// WithMaxConcurrentStreams 指定每个客户端同时可打开的并发流的数量。
func WithMaxConcurrentStreams(n uint32) Option {
	return Option{F: func(o *Config) {
		o.MaxConcurrentStreams = n
	}}
}

// WithMaxReadFrameSize 指定服务器将读取的最大帧大小。
func WithMaxReadFrameSize(n uint32) Option {
	return Option{F: func(o *Config) {
		o.MaxReadFrameSize = n
	}}
}

// WithPermitProhibitedCipherSuites 用于设置是否允许禁止chipher套件。
func WithPermitProhibitedCipherSuites(b bool) Option {
	return Option{F: func(o *Config) {
		o.PermitProhibitedCipherSuites = b
	}}
}

// WithIdleTimeout 设置连接的空闲超时时间。默认 consts.DefaultMaxIdleConnDuration
func WithIdleTimeout(t time.Duration) Option {
	return Option{F: func(o *Config) {
		o.IdleTimeout = t
	}}
}

// WithMaxUploadBufferPerConnection 设置每个连接的初始流控制窗口的大小。
func WithMaxUploadBufferPerConnection(n int32) Option {
	return Option{F: func(o *Config) {
		o.MaxUploadBufferPerConnection = n
	}}
}

// WithMaxUploadBufferPerStream 设置每个流的初始流窗口的大小。
func WithMaxUploadBufferPerStream(n int32) Option {
	return Option{F: func(o *Config) {
		o.MaxUploadBufferPerStream = n
	}}
}

func NewConfig(opts ...Option) *Config {
	c := &Config{
		IdleTimeout: consts.DefaultMaxIdleConnDuration,
	}
	c.Apply(opts)
	return c
}
