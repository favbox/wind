package client

import (
	"crypto/tls"
	"time"

	"github.com/favbox/wind/app/client/retry"
	"github.com/favbox/wind/common/config"
	"github.com/favbox/wind/network"
	"github.com/favbox/wind/network/dialer"
	"github.com/favbox/wind/network/standard"
	"github.com/favbox/wind/protocol/consts"
)

// WithDialTimeout 设置连接到服务器的超时时间。默认值：1 秒。
func WithDialTimeout(dialTimeout time.Duration) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.DialTimeout = dialTimeout
	}}
}

// WithDialer 指定自定义拨号器。
func WithDialer(d network.Dialer) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.Dialer = d
	}}
}

// WithDialFunc 设置拨号函数。注意：将覆盖自定义拨号器。
func WithDialFunc(f network.DialFunc, dialers ...network.Dialer) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		d := dialer.DefaultDialer()
		if len(dialers) != 0 {
			d = dialers[0]
		}
		o.Dialer = newCustomDialerWithDialFunc(d, f)
	}}
}

// WithMaxConnsPerHost 设置每个主机可建立的最大连接数。默认值：512 个。
func WithMaxConnsPerHost(mc int) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.MaxConnsPerHost = mc
	}}
}

// WithMaxIdleConnDuration 设置闲置连接的超时关闭时长。默认值：10 秒。
func WithMaxIdleConnDuration(t time.Duration) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.MaxIdleConnDuration = t
	}}
}

// WithMaxConnDuration 设置长连接的超时时长。默认值：不限时长。
func WithMaxConnDuration(t time.Duration) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.MaxConnDuration = t
	}}
}

// WithMaxConnWaitTimeout 设置等待一个闲置连接的最大时长。
func WithMaxConnWaitTimeout(t time.Duration) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.MaxConnWaitTimeout = t
	}}
}

// WithKeepAlive 设置是否启用长连接。默认值：true，启用。
func WithKeepAlive(b bool) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.KeepAlive = b
	}}
}

// WithClientReadTimeout 设置完整响应的最大读取时长（包括正文）。默认值：不限时长。
func WithClientReadTimeout(t time.Duration) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.ReadTimeout = t
	}}
}

// WithTLSConfig 设置 TLS 配置以便开启 https 安全连接。
func WithTLSConfig(cfg *tls.Config) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.TLSConfig = cfg
		o.Dialer = standard.NewDialer()
	}}
}

// WithResponseBodyStream 设置是否流式处理响应正文。默认值：false。
func WithResponseBodyStream(b bool) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.ResponseBodyStream = b
	}}
}

// WithDisableHeaderNamesNormalizing 设置是否禁用标头名称的规范化。
func WithDisableHeaderNamesNormalizing(disable bool) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.DisableHeaderNamesNormalizing = disable
	}}
}

// WithName 自定义 User-Agent 标头值。 默认值: wind。
func WithName(name string) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.Name = name
	}}
}

// WithNoDefaultUserAgentHeader 不要默认的 User-Agent 标头值。默认值：false。
func WithNoDefaultUserAgentHeader(isNoDefaultUserAgentHeader bool) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.NoDefaultUserAgentHeader = isNoDefaultUserAgentHeader
	}}
}

// WithDisablePathNormalizing 设置是否禁用路径规范化。默认值：false。
func WithDisablePathNormalizing(isDisablePathNormalizing bool) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.DisablePathNormalizing = isDisablePathNormalizing
	}}
}

// WithRetryConfig 设置重试相关的配置。
func WithRetryConfig(opts ...retry.Option) config.ClientOption {
	retryCfg := &retry.Config{
		MaxAttemptTimes: consts.DefaultMaxRetryTimes,
		Delay:           1 * time.Millisecond,
		MaxDelay:        100 * time.Millisecond,
		MaxJitter:       20 * time.Millisecond,
		DelayPolicy:     retry.CombineDelay(retry.DefaultDelayPolicy),
	}
	retryCfg.Apply(opts)

	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.RetryConfig = retryCfg
	}}
}

// WithWriteTimeout 设置完整写入的最大时长。默认值：不限时长。
func WithWriteTimeout(t time.Duration) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.WriteTimeout = t
	}}
}

// WithConnStateObserve 设置连接状态观察函数、观察的间隔时长(默认 5 秒）。
func WithConnStateObserve(hs config.HostClientStateFunc, interval ...time.Duration) config.ClientOption {
	return config.ClientOption{F: func(o *config.ClientOptions) {
		o.HostClientStateObserve = hs
		if len(interval) > 0 {
			o.ObservationInterval = interval[0]
		}
	}}
}

// customDialer 定义自定义拨号器。
type customDialer struct {
	network.Dialer
	dialFunc network.DialFunc
}

func (m *customDialer) DialConnection(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn network.Conn, err error) {
	if m.dialFunc != nil {
		return m.dialFunc(address)
	}
	return m.Dialer.DialConnection(network, address, timeout, tlsConfig)
}

func newCustomDialerWithDialFunc(dialer network.Dialer, dialFunc network.DialFunc) network.Dialer {
	return &customDialer{
		Dialer:   dialer,
		dialFunc: dialFunc,
	}
}
