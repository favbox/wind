package server

import (
	"context"
	"crypto/tls"
	"net"
	"strings"
	"time"

	"github.com/favbox/wind/app/server/registry"
	"github.com/favbox/wind/common/config"
	"github.com/favbox/wind/common/tracer"
	"github.com/favbox/wind/common/tracer/stats"
	"github.com/favbox/wind/network"
	"github.com/favbox/wind/network/standard"
)

// WithHostPorts 指定监听的地址和端口。默认值：":8888"。
func WithHostPorts(addr string) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.Addr = addr
	}}
}

// WithBasePath 设置基本路径。默认值：`/`。
func WithBasePath(basePath string) config.Option {
	return config.Option{F: func(o *config.Options) {
		// 必须以 "/" 作为前缀和后缀，否则就拼接上 "/"
		if !strings.HasPrefix(basePath, "/") {
			basePath = "/" + basePath
		}
		if !strings.HasSuffix(basePath, "/") {
			basePath = basePath + "/"
		}
		o.BasePath = basePath
	}}
}

// WithReadTimeout 设置网络库读取数据超时时间。默认值 3 分钟。
//
// 当读超时时连接将关闭。
func WithReadTimeout(t time.Duration) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.ReadTimeout = t
	}}
}

// WithWriteTimeout 设置网络库写入数据超时时间。默认值：无限长。
//
// 当写超时时连接将关闭。
func WithWriteTimeout(t time.Duration) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.WriteTimeout = t
	}}
}

// WithIdleTimeout 设置长连接闲置的超时时间。默认值 3 分钟。
//
// 当闲置时间超时时连接将关闭，以免受行为不端的客户端的攻击。
func WithIdleTimeout(t time.Duration) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.IdleTimeout = t
	}}
}

// WithKeepAliveTimeout 设置长连接超时时间。
//
// 在大多数情况下，无需关心该选项。
// 默认值：1 分钟。
func WithKeepAliveTimeout(t time.Duration) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.KeepAliveTimeout = t
	}}
}

// WithRedirectTrailingSlash 自动根据末尾的 /转发。
//   - 路由器只有 /foo/ 则 /foo 重定向到 /foo；
//   - 路由器只有 /foo 则 /foo/ 会重定向到 /foo/。
//
// 默认值：开启。
func WithRedirectTrailingSlash(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.RedirectTrailingSlash = b
	}}
}

// WithRedirectFixedPath 若无匹配路由，尝试修复并重新匹配。
//   - 若匹配成功且为 GET 请求，则返回 301 状态码并重定向；
//   - 若匹配成功但非 GET 请求，则返回 308 状态码并重定向。
//
// 默认值：不开启。
func WithRedirectFixedPath(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.RedirectFixedPath = b
	}}
}

// WithHandleMethodNotAllowed 请求方法不匹配但有同路径其他方法，返回 405 方法不允许而非 404 找不到。
// 默认值：关闭。
func WithHandleMethodNotAllowed(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.HandleMethodNotAllowed = b
	}}
}

// WithRemoveExtraSlash 移除额外的空格再进行路由匹配。
// 如：/user/:name，开启后 /user//mike 也可匹配上参数。
// 默认值：不使用。
func WithRemoveExtraSlash(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.RemoveExtraSlash = b
	}}
}

// WithUseRawPath 使用原始未转义的路径进行路由匹配。
// 默认值：不使用。
func WithUseRawPath(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.UseRawPath = b
	}}
}

// WithUnescapePathValues 转义路径后再进行路由匹配。
//
// 如 `%2F` -> `/`：
//   - 若 UseRawPath 为 false（默认情况）， UnescapePathValues 实为 true，因为 .URI.Path() 会被使用，它已转义。
//   - 若此值为 false，需配合 WithUseRawPath(true) 使用。
//
// 默认值：true。开启转义。
func WithUnescapePathValues(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.UnescapePathValues = b
	}}
}

// WithDisablePreParseMultipartForm 不预先解析多部分表单，可以通过 ctx.Request.Body() 获取正文后由用户处理。
// 默认值：false，不禁用预先解析。
func WithDisablePreParseMultipartForm(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.DisablePreParseMultipartForm = b
	}}
}

// WithMaxRequestBodySize 限制请求正文的最大字节数。
// 默认值：4MB。
func WithMaxRequestBodySize(bs int) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.MaxRequestBodySize = bs
	}}
}

// WithMaxKeepBodySize 限制回收时保留的请求体和响应体的最大字节数。
//
// 大于此大小的正文缓冲区将被放回缓冲池。
//
// 默认值：4MB。
func WithMaxKeepBodySize(bs int) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.MaxKeepBodySize = bs
	}}
}

// WithGetOnly 只接受 GET 请求，默认值：false。
func WithGetOnly(isOnly bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.GetOnly = isOnly
	}}
}

// WithKeepAlive 禁用长连接。默认值：false。
func WithKeepAlive(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.DisableKeepalive = !b
	}}
}

// WithStreamBody 在流中读取正文。
//
// 启用流式处理，可在请求的正文超过当前字节数限制时，更快地调用处理器。
//
// 默认值：false，不启用流式正文。
func WithStreamBody(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.StreamRequestBody = b
	}}
}

// WithNetwork 网络协议，可选：tcp，udp，unix（unix domain socket）。
// 默认值：tcp。
func WithNetwork(nw string) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.Network = nw
	}}
}

// WithExitWaitTime 优雅退出的等待时间。
//
// 服务器会停止建立新连接，并对关闭后的每个请求设置 'Connection: close' 标头。
// 当到达设定的时间关闭服务器。若所有连接均已关闭则可提前关闭。
//
// 默认值：5 秒。
func WithExitWaitTime(t time.Duration) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.ExitWaitTimeout = t
	}}
}

// WithTLS 配置为 TLS 服务器。
func WithTLS(cfg *tls.Config) config.Option {
	return config.Option{F: func(o *config.Options) {
		// 如无明确的传输器则用标准的，因 netpoll 尚不支持。
		if o.TransporterNewer == nil {
			o.TransporterNewer = standard.NewTransporter
		}
		o.TLS = cfg
	}}
}

// WithListenConfig 设置监听器配置。如配置是否允许端口重用。
func WithListenConfig(l *net.ListenConfig) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.ListenConfig = l
	}}
}

// WithTransport 更换网络传输器。默认值：netpoll.NewTransporter。
func WithTransport(transporter func(opts *config.Options) network.Transporter) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.TransporterNewer = transporter
	}}
}

// WithAltTransport 设置备用传输器。默认值：netpoll.NewTransporter。
func WithAltTransport(transporter func(opts *config.Options) network.Transporter) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.AltTransporterNewer = transporter
	}}
}

// WithH2C 设置是否开启 HTTP/2 客户端。默认值：false，关闭。
func WithH2C(enable bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.H2C = enable
	}}
}

// WithReadBufferSize 设置读缓冲区字节数，同时限制 HTTP 标头大小。
// 默认值：4KB。
func WithReadBufferSize(size int) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.ReadBufferSize = size
	}}
}

// WithALPN 设置是否开启 ALPN。默认值：false，关闭。
func WithALPN(enable bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.ALPN = enable
	}}
}

// WithTracer 注入链路跟踪器实例。若不注入则意为关闭。
func WithTracer(t tracer.Tracer) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.Tracers = append(o.Tracers, t)
	}}
}

// WithTraceLevel 设置链路跟踪级别。默认值：stats.LevelDetailed。
func WithTraceLevel(level stats.Level) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.TraceLevel = level
	}}
}

// WithRegistry 设置注册中心配置，服务注册信息。
// 默认值：registry.NoopRegistry, nil
func WithRegistry(r registry.Registry, info *registry.Info) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.Registry = r
		o.RegistryInfo = info
	}}
}

// WithAutoReloadRender 设置是否自动重载 HTML 模板，重载间隔。
// 若启用：
//  1. 重载间隔 = 0 意为根据文件监视机制重载（推荐）
//  2. 重载间隔 > 0 意为按间隔时间每次都重载。
//
// 默认值：false, 0。
func WithAutoReloadRender(b bool, interval time.Duration) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.AutoReloadRender = b
		o.AutoReloadInterval = interval
	}}
}

// WithDisablePrintRoute 设置是否禁止打印路由。默认值：false，不禁止。
func WithDisablePrintRoute(b bool) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.DisablePrintRoute = b
	}}
}

// WithOnAccept 设置在 netpoll 中新连接被接受但不能接收数据时的回调函数。
// 在 go net 中，它将在转为 TLS 连接之前被调用。
//
// 默认值：nil。
func WithOnAccept(fn func(conn net.Conn) context.Context) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.OnAccept = fn
	}}
}

// WithOnConnect 设置在 netpoll 中接收来自连接的数据。
// 在 go net 中，它将在转为 TLS 连接之后被调用。
//
// 默认值：nil。
func WithOnConnect(fn func(ctx context.Context, conn network.Conn) context.Context) config.Option {
	return config.Option{F: func(o *config.Options) {
		o.OnConnect = fn
	}}
}
