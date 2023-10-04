package config

import (
	"crypto/tls"
	"time"

	"github.com/favbox/wind/app/client/retry"
	"github.com/favbox/wind/network"
	"github.com/favbox/wind/protocol/consts"
)

// ConnPoolState 客户端连接池的状态结构体。
type ConnPoolState struct {
	// 连接池的连接数量，这些连接是闲置连接。
	PoolConnNum int
	// 总连接数。
	TotalConnNum int
	// 挂起的连接数量。
	WaitConnNum int
	// HostClient 地址
	Addr string
}

type HostClientState interface {
	ConnPoolState() ConnPoolState
}

type HostClientStateFunc func(HostClientState)

// ClientOption 是配置客户端选项的唯一结构体。
type ClientOption struct {
	F func(o *ClientOptions)
}

// ClientOptions 客户端选项结构体。
type ClientOptions struct {
	// 连接到服务器的超时时间，默认值 consts.DefaultDialTimeout
	DialTimeout time.Duration

	// 每个主机的最大连接数，默认值 consts.DefaultMaxConnsPerHost
	MaxConnsPerHost int

	// 闲置连接超过此时长后会被关闭。
	//
	// 默认值为 consts.DefaultMaxIdleConnDuration。
	MaxIdleConnDuration time.Duration

	// 长连接最大保活时长。
	//
	// 默认不限时长。
	MaxConnDuration time.Duration

	// 等待一个闲置连接的最大时长。
	//
	// 默认不等待，立即返回 ErrNoFreeConns。
	MaxConnWaitTimeout time.Duration

	// 是否启用长连接，默认启用
	KeepAlive bool

	// 读超时（包括标头和正文）。
	//
	// 默认不限时长。
	ReadTimeout time.Duration

	// 主机连接的 TLS（也叫 SSL 或 HTTPS）配置，可选。
	TLSConfig *tls.Config

	// 是否流式响应正文。
	ResponseBodyStream bool

	// 客户端名称。用于 User-Agent 请求标头。
	Name string

	// 若在请求时排除 User-Agent 标头，则设为真。
	NoDefaultUserAgentHeader bool

	// 用于建立主机连接的回调
	//
	// 若未设置，则使用默认拨号器。
	Dialer network.Dialer

	// 双重包好，若为真，则尝试同时连接 ipv4 和 ipv6 的主机地址。
	//
	// 该选项仅当使用默认 TCP 拨号器时可用，如 Dialer 未设置。
	//
	// 默认只连接到 ipv4 地址，因为 ipv6 在全球很多网络中处于故障状态。
	DialDualStack bool

	// 写超时（包括标头和正文）。
	//
	// 默认不限时长。
	WriteTimeout time.Duration

	// 响应主体的最大字节数。超限则客户端返回 errBodyTooLarge。
	//
	// 默认不限字节数大小。
	MaxResponseBodySize int

	// 若为真，则标头名称按原样传递，而无需规范化。
	//
	// 禁用标头名称的规范化，可能对代理其他需要区分标头大小写的客户端响应有用。
	//
	// 默认情况下，请求和响应的标头名称都要规范化，例如
	// 首字母和破折号后的首字母都转为大写，其余转为小写。
	// 示例：
	//
	//	* HOST -> Host
	//	* connect-type -> Content-Type
	//	* cONTENT-lenGTH -> Content-Length
	// 默认不禁用。
	DisableHeaderNamesNormalizing bool

	// 若为真，则标头名称按原样传递，而无需规范化。
	//
	// 禁用路径的规范化，可能对代理期望保留原始路径的传入请求有用。
	// 默认不禁用。
	DisablePathNormalizing bool

	// 与重试相关的所有配置
	RetryConfig *retry.Config

	// 观察主机客户端的状态
	HostClientStateObserve HostClientStateFunc

	// 观察间隔时长
	ObservationInterval time.Duration

	// 重配主机客户端的回调钩子。
	// 若出错，则请求将被终止。
	HostClientConfigHook func(hc any) error
}

func (o *ClientOptions) Apply(opts []ClientOption) {
	for _, opt := range opts {
		opt.F(o)
	}
}

func NewClientOptions(opts []ClientOption) *ClientOptions {
	o := &ClientOptions{
		DialTimeout:         consts.DefaultDialTimeout,
		MaxConnsPerHost:     consts.DefaultMaxConnsPerHost,
		MaxIdleConnDuration: consts.DefaultMaxIdleConnDuration,
		KeepAlive:           true,
		ObservationInterval: 5 * time.Second,
	}
	o.Apply(opts)

	return o
}
