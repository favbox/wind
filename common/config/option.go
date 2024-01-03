package config

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/favbox/wind/app/server/registry"
	"github.com/favbox/wind/network"
)

const (
	defaultKeepAliveTimeout   = 1 * time.Minute
	defaultReadTimeout        = 3 * time.Minute
	defaultWaitExitTimeout    = 5 * time.Second
	defaultNetwork            = "tcp"
	defaultAddr               = ":8888"
	defaultBasePath           = "/"
	defaultMaxRequestBodySize = 4 * 1024 * 1024
	defaultReadBufferSize     = 4 * 1024
)

// Option 是用于配置 Options 唯一结构体。
type Option struct {
	F func(o *Options)
}

// Options 是配置项的结构体。
type Options struct {
	// KeepAliveTimeout 是长连接的超时时间，默认 1 分钟，通常无需关心，仅需关心 IdleTimeout。
	KeepAliveTimeout time.Duration

	// ReadTimeout 是网络库读取的超时时间，默认 3 分钟，0 代表永不超时。
	ReadTimeout time.Duration

	// WriteTimeout 是网络库写入的超时时间，默认为 0，即永不超时。
	WriteTimeout time.Duration

	// IdleTime 是长连接的闲置超时，超时则关闭。 默认为 ReadTimeout 即 3 分钟，0 代表永不超时。
	IdleTimeout time.Duration

	// 是否将 /foo/ 重定向到 /foo，或者反过来。默认重定向。
	RedirectTrailingSlash bool

	// 将 /FOO 和 /..//FOO 重定向到 /foo，默认不重定向。
	RedirectFixedPath bool

	// 请求方法不匹配但有同路径其他方法，返回 405 方法不允许而非 404 找不到。
	HandleMethodNotAllowed bool

	// 移除额外的斜杠。
	RemoveExtraSlash bool

	// 使用原始未转义的路径进行路由匹配，默认不使用。
	UseRawPath bool

	// 转义路径值，如 `%2F` -> `/`。
	// 若 UseRawPath 为 false（默认情况），
	// 则 UnescapePathValues 实为 true，因为 URI.Path() 会被使用，它已转义。
	// 若此值为 false，需配合 WithUseRawPath(true) 使用。
	// 默认开启转义(true)。
	UnescapePathValues bool

	MaxRequestBodySize           int           // 正文的最大请求字节数，默认 4MB
	MaxKeepBodySize              int           // 正文的最大保留字节数，默认 4MB
	GetOnly                      bool          // 是否仅支持 GET 请求，默认否
	DisableKeepalive             bool          // 是否禁用长连接，默认否
	DisablePreParseMultipartForm bool          // 是否不预先解析多部分表单，默认否
	NoDefaultDate                bool          // 禁止响应头添加 Date 的默认字段值，默认否
	NoDefaultContentType         bool          // 禁止响应头添加 Content-Type 的默认字段值，默认否
	StreamRequestBody            bool          // 是否流式处理请求体，默认否
	NoDefaultServerHeader        bool          // 是否不要默认的服务器名称标头，默认否
	DisablePrintRoute            bool          // 是否禁止打印路由，默认否
	Network                      string        // 网络协议，可选 "tcp", "udp", "unix"(unix domain socket)，默认 "tcp"
	Addr                         string        // 监听地址，默认 ":8888"
	BasePath                     string        // 基本路径，默认 "/"
	ExitWaitTimeout              time.Duration // 优雅退出的等待时间，默认 5s
	TLS                          *tls.Config
	ALPN                         bool  // 是否打开 ALPN 应用层协议协商的开关，默认否
	H2C                          bool  // 是否打开 HTTP/2 Cleartext （明文）协议开关，默认否
	ReadBufferSize               int   // 初始的读缓冲大小，默认 4KB。通常无需设置。
	Tracers                      []any // 链路跟踪控制器器，默认零长度切片
	TraceLevel                   any   // 跟踪级别，默认 stats.LevelDetailed
	ListenConfig                 *net.ListenConfig

	BindConfig      any // 请求参数绑定器的配置项
	ValidateConfig  any // 请求参数验证器的配置项
	CustomBinder    any // 自定义请求参数绑定器
	CustomValidator any // 自定义请求参数验证器

	// TransporterNewer 是传输器的自定义创建函数。
	TransporterNewer func(opt *Options) network.Transporter
	// AltTransporterNewer 是替补的传输器自定义创建函数。
	AltTransporterNewer func(opt *Options) network.Transporter

	// 在 netpoll 库中，OnAccept 是在接受连接之后且加到 epoll 之前调用的。OnConnect 是在加到 epoll 之后调用的。
	// 区别在于 OnConnect 能取数据，而 OnAccept 不能。例如想检查对端IP是否在黑名单中，可使用 OnAccept。
	//
	// 在 go/net 中，OnAccept 是在接受连接之后且建立 tls 连接之前调用的。建立 tls 连接后执行 OnConnect。
	OnAccept  func(conn net.Conn) context.Context
	OnConnect func(ctx context.Context, conn network.Conn) context.Context

	// 用于服务注册。
	Registry registry.Registry

	// 用于服务注册的信息。
	RegistryInfo *registry.Info

	// 是否自动重载 HTML 模板?
	AutoReloadRender bool

	// HTML 模板自动重载时间间隔。
	// 默认为0，即根据文件变更事件立即重载。
	AutoReloadInterval time.Duration

	// 若设置该选项，则标头名称将原样传递而不用规范化。
	// 禁用标头名称的规范化，可能仅对其他客户端的代理响应有用。
	//
	// 默认值：false
	//
	// 默认情况下，请求和响应的标头名称均会规范化，即：
	// 第一个字母和分隔符'-'后面的第一个字母会被转为大写，其他字母会被转为小写。
	// 示例：
	//
	//	* HOST -> Host
	//	* content-type -> Content-Type
	//	* cONTENT-lenGTH -> Content-Length
	DisableHeaderNamesNormalizing bool
}

// Apply 将指定的一组配置方法 opts 应用到配置项上。
func (o *Options) Apply(opts []Option) {
	for _, opt := range opts {
		opt.F(o)
	}
}

// NewOptions 创建基于给定配置函数的配置项。
func NewOptions(opts []Option) *Options {
	options := &Options{
		KeepAliveTimeout:              defaultKeepAliveTimeout,
		ReadTimeout:                   defaultReadTimeout,
		IdleTimeout:                   defaultReadTimeout,
		RedirectTrailingSlash:         true,
		UnescapePathValues:            true,
		Network:                       defaultNetwork,
		Addr:                          defaultAddr,
		BasePath:                      defaultBasePath,
		MaxRequestBodySize:            defaultMaxRequestBodySize,
		MaxKeepBodySize:               defaultMaxRequestBodySize,
		ExitWaitTimeout:               defaultWaitExitTimeout,
		ReadBufferSize:                defaultReadBufferSize,
		Tracers:                       []any{},
		TraceLevel:                    new(any),
		Registry:                      registry.NoopRegistry,
		DisableHeaderNamesNormalizing: false,
	}
	options.Apply(opts)
	return options
}
