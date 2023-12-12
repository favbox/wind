package http1

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/favbox/wind/app/client/retry"
	"github.com/favbox/wind/common/config"
	errs "github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/common/timer"
	"github.com/favbox/wind/common/wlog"
	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/internal/bytestr"
	"github.com/favbox/wind/internal/nocopy"
	"github.com/favbox/wind/network"
	"github.com/favbox/wind/network/dialer"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/client"
	"github.com/favbox/wind/protocol/consts"
	"github.com/favbox/wind/protocol/http1/proxy"
	reqI "github.com/favbox/wind/protocol/http1/req"
	respI "github.com/favbox/wind/protocol/http1/resp"
)

var (
	clientConnPool sync.Pool

	errTimeout          = errs.New(errs.ErrTimeout, errs.ErrorTypePublic, "host client")
	errConnectionClosed = errs.NewPublic("服务器在返回首个响应字节之前关闭了连接。请确保服务器在关闭连接之前返回 'Connection: close' 响应头")
)

type ClientOptions struct {
	// 客户端名称。用于 User-Agent 请求标头。
	Name string

	// 若在请求时排除 User-Agent 标头，则设为真。
	NoDefaultUserAgentHeader bool

	// 用于建立主机连接的回调
	//
	// 若未设置，则使用默认拨号器。
	Dialer network.Dialer

	// 建立主机连接的超时时间。
	//
	// 若为设置，则使用默认拨号超时时间。
	DialTimeout time.Duration

	// 双重拨号，若为真，则尝试同时连接 ipv4 和 ipv6 的主机地址。
	//
	// 该选项仅当使用默认 TCP 拨号器时可用，如 Dialer 未设置。
	//
	// 默认只连接到 ipv4 地址，因为 ipv6 在全球很多网络中处于故障状态。
	DialDualStack bool

	// 是否对主机连接使用 TLS（也叫 SSL 或 HTTPS），可选。
	TLSConfig *tls.Config

	// 最大主机连接数。
	//
	// 使用 HostClient 时，可通过 HostClient.SetMaxConns 更改此值。
	MaxConns int

	// Keep-alive 长连接超过此时长会被关闭。
	//
	// 默认不限时长。
	MaxConnDuration time.Duration

	// 闲置连接超过此时长后会被关闭。
	//
	// 默认值为 DefaultMaxIdleConnDuration。
	MaxIdleConnDuration time.Duration

	// 完整响应的最大读取时长（包括正文）。
	//
	// 默认不限时长。
	ReadTimeout time.Duration

	// 完整请求的最大写入时长（包括正文）。
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
	DisableHeaderNamesNormalizing bool

	// 若为真，则标头名称按原样传递，而无需规范化。
	//
	// 禁用路径的规范化，可能对代理期望保留原始路径的传入请求有用。
	DisablePathNormalizing bool

	// 等待一个闲置连接的最大时长。
	//
	// 默认不等待，立即返回 ErrNoFreeConns。
	MaxConnWaitTimeout time.Duration

	// 是否流式处理响应正文
	ResponseBodyStream bool

	// 与重试相关的所有配置
	RetryConfig *retry.Config

	RetryIfFunc client.RetryIfFunc

	// 观察主机客户端的状态
	StateObserve config.HostClientStateFunc

	// 观察间隔时长
	ObservationInterval time.Duration
}

// HostClient 在 Addr 列举的主机之间平衡 http 请求。并发不安全，拷贝不安全。
type HostClient struct {
	noCopy nocopy.NoCopy

	*ClientOptions

	// 逗号分隔的上游 HTTP 服务器主机地址列表，以循环方式传递给 Dialer。
	//
	// 如果使用默认拨号程序，则每个地址都可能包含端口。
	// 例如：
	//
	//	- foobar.com:80
	//	- foobar.com:443
	//	- foobar.com:8080
	Addr     string
	IsTLS    bool
	ProxyURI *protocol.URI

	clientName  atomic.Value
	lastUseTime uint32

	connsLock  sync.Mutex
	connsCount int
	conns      []*clientConn
	connsWait  *wantConnQueue

	addrsLock sync.Mutex
	addrs     []string
	addrIdx   uint32

	tlsConfigMap     map[string]*tls.Config
	tlsConfigMapLock sync.Mutex

	pendingRequests int32

	connsCleanerRun bool

	closed chan struct{}
}

// NewHostClient 创建新的主机客户端。
func NewHostClient(c *ClientOptions) client.HostClient {
	hc := &HostClient{
		ClientOptions: c,
		closed:        make(chan struct{}),
	}
	return hc
}

func (c *HostClient) Close() error {
	close(c.closed)
	return nil
}

// CloseIdleConnections 关闭闲置连接。不会中断使用中的连接。
func (c *HostClient) CloseIdleConnections() {
	c.connsLock.Lock()
	scratch := append([]*clientConn{}, c.conns...)
	for i := range c.conns {
		c.conns[i] = nil
	}
	c.conns = c.conns[:0]
	c.connsLock.Unlock()

	for _, cc := range scratch {
		c.closeConn(cc)
	}
}

func (c *HostClient) closeConn(cc *clientConn) {
	c.decConnsCount()
	cc.c.Close()
	releaseClientConn(cc)
}

// ConnectionCount 返回连接池中的闲置连接的数量。
func (c *HostClient) ConnectionCount() (count int) {
	c.connsLock.Lock()
	count = len(c.conns)
	c.connsLock.Unlock()
	return
}

// ConnPoolState 返回主机客户端的连接池状态。
func (c *HostClient) ConnPoolState() config.ConnPoolState {
	c.connsLock.Lock()
	defer c.connsLock.Unlock()
	cps := config.ConnPoolState{
		PoolConnNum:  len(c.conns),
		TotalConnNum: c.connsCount,
		Addr:         c.Addr,
	}

	if c.connsWait != nil {
		cps.WaitConnNum = c.connsWait.len()
	}

	return cps
}

// Do 执行给定的 http 请求并填充给定的 http 响应。
//
// Request 至少包含非空的完整网址（包括方案和主机）或非空的主机头+请求网址。
//
// 该函数不遵循重定向。用 Get* 做重定向。
//
// 若 resp 为空，则不处理 Response。
//
// ErrNoFreeConns 将在到主机的所有 HostClient.MaxConns 连接都繁忙时返回。
//
// 推荐获取 req 和 resp 的方式为 AcquireRequest 和 AcquireResponse，在性能关键代码中可提升性能。
func (c *HostClient) Do(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
	var (
		err                error
		canIdempotentRetry bool               // 能否幂等重试
		isDefaultRetryFunc                    = true
		attempts           uint               = 0 // 当前尝试次数
		connAttempts       uint               = 0 // 客户端连接尝试次数
		maxAttempts        uint               = 1 // 最多尝试次数
		isRequestRetryable client.RetryIfFunc = client.DefaultRetryIf
	)
	retryCfg := c.ClientOptions.RetryConfig
	if retryCfg != nil {
		maxAttempts = retryCfg.MaxAttemptTimes
	}

	if c.ClientOptions.RetryIfFunc != nil {
		isRequestRetryable = c.ClientOptions.RetryIfFunc
		// 若用户提供了自定义重试函数，则 canIdempotentRetry 不再有意义。
		// 用户将通过自定义重试函数完全控制重试逻辑。
		isDefaultRetryFunc = false
	}

	atomic.AddInt32(&c.pendingRequests, 1)
	req.Options().StartRequest()
	for {
		select {
		case <-ctx.Done():
			req.CloseBodyStream()
			return ctx.Err()
		default:
		}

		canIdempotentRetry, err = c.do(req, resp)
		// 若无自定义重试且 err == nil，则循环将直接退出。
		if err == nil && isDefaultRetryFunc {
			if connAttempts != 0 {
				wlog.SystemLogger().Warnf("客户端连接尝试次数：%d，网址：%s。"+
					"这主要是因为对端提前关闭了池中的连接。"+
					"若该数过高，则表明长连接基本已不可用，"+
					"尝试把请求改为短链接。\n", connAttempts, req.URI().FullURI())
			}
			break
		}

		// 此连接在连接池时已被对端关闭。
		//
		// 这种情况可能发生在闲置的长连接因超时而被服务器关闭。
		//
		// Apache 和 Nginx 通常这么做。
		if canIdempotentRetry && client.DefaultRetryIf(req, resp, err) && errors.Is(err, errs.ErrBadPoolConn) {
			connAttempts++
			continue
		}

		if isDefaultRetryFunc {
			break
		}

		attempts++
		if attempts >= maxAttempts {
			break
		}

		// 检查是否应重试此请求
		if !isRequestRetryable(req, resp, err) {
			break
		}

		wait := retry.Delay(attempts, err, retryCfg)
		// 等待 wait 时间后重试
		time.Sleep(wait)
	}
	atomic.AddInt32(&c.pendingRequests, -1)

	if err == io.EOF {
		err = errConnectionClosed
	}
	return err
}

// DoDeadline 执行给定的 http 请求并等待响应直至到达截止时间。
//
// Request 至少包含非空的完整网址（包括方案和主机）或非空的主机头+请求网址。
//
// 该函数不遵循重定向。用 Get* 做重定向。
//
// 若 resp 为空，则忽略 Response 处理。
//
// errTimeout 将在截止时间到达后被返回。
//
// ErrNoFreeConns 将在到主机的所有 HostClient.MaxConns 连接都繁忙时返回。
//
// 推荐获取 req 和 resp 的方式为 AcquireRequest 和 AcquireResponse，在性能关键代码中可提升性能。
func (c *HostClient) DoDeadline(ctx context.Context, req *protocol.Request, resp *protocol.Response, deadline time.Time) error {
	// 设置请求的超时时长，然后还是用上面的 Do 方法执行。
	return client.DoDeadline(ctx, req, resp, deadline, c)
}

// DoTimeout 执行给定的 http 请求并在给定的超时时间内等待响应。
//
// Request 至少包含非空的完整网址（包括方案和主机）或非空的主机头+请求网址。
//
// 该函数不遵循重定向。用 Get* 做重定向。
//
// 若 resp 为空，则忽略 Response 处理。
//
// errTimeout 将在截止时间到达后被返回。
//
// ErrNoFreeConns 将在到主机的所有 HostClient.MaxConns 连接都繁忙时返回。
//
// 推荐获取 req 和 resp 的方式为 AcquireRequest 和 AcquireResponse，在性能关键代码中可提升性能。
//
// 警告：DoTimeout 不会中止请求自身，即请求将在后台继续执行，响应将被丢弃。
// 如果请求时间过长，并且连接池已满，请尝试设置 ReadTimeout。
func (c *HostClient) DoTimeout(ctx context.Context, req *protocol.Request, resp *protocol.Response, timeout time.Duration) error {
	return client.DoTimeout(ctx, req, resp, timeout, c)
}

// DoRedirects 执行给定的 http 请求 req 并设置相应的 resp，
// 遵循 maxRedirectsCount 次重定向。当触达最大重定向次数，返回 ErrTooManyRedirects。
//
// Request 至少包含非空的完整网址（包括方案和主机）或非空的主机头+请求网址。
//
// 客户端确定要请求的服务器遵循如下顺序：
//
//   - 首先，尝试使用 RequestURI，前提是 req 包含带有 schema 和 host 的完整网址；
//   - 否则，使用主机 header。
//
// 若 resp 为空，则忽略 Response 处理。
//
// ErrNoFreeConns 将在到主机的所有 HostClient.MaxConns 连接都繁忙时返回。
//
// 推荐获取 req 和 resp 的方式为 AcquireRequest 和 AcquireResponse，在性能关键代码中可提升性能。
func (c *HostClient) DoRedirects(ctx context.Context, req *protocol.Request, resp *protocol.Response, maxRedirectsCount int) error {
	_, _, err := client.DoRequestFollowRedirects(ctx, req, resp, req.URI().String(), maxRedirectsCount, c)
	return err
}

// Get 返回给定网址的状态码和响应体。
//
// dst 的内容将被响应体替换并返回，若 dst 过小将分配一个新切片。
//
// 该函数遵循重定向。使用 Do* 可手动处理重定向。
func (c *HostClient) Get(ctx context.Context, dst []byte, url string) (statusCode int, body []byte, err error) {
	return client.GetURL(ctx, dst, url, c)
}

// GetDeadline 返回给定 url 的状态码和响应体。
//
// dst 的内容将被响应体替换并返回，若 dst 过小将分配一个新切片。
//
// 该函数遵循重定向。使用 Do* 可手动处理重定向。
//
// errTimeout 将在截止时间到达后被返回。
func (c *HostClient) GetDeadline(ctx context.Context, dst []byte, url string, deadline time.Time) (statusCode int, body []byte, err error) {
	return client.GetURLDeadline(ctx, dst, url, deadline, c)
}

// GetURLTimeout 返回给定 url 的状态码和响应体。
//
// dst 的内容将被响应体替换并返回，若 dst 过小将分配一个新切片。
//
// 该函数遵循重定向。使用 Do* 可手动处理重定向。
//
// errTimeout 将在触达超时时长后被返回。
func (c *HostClient) GetTimeout(ctx context.Context, dst []byte, url string, timeout time.Duration) (statusCode int, body []byte, err error) {
	return client.GetURLTimeout(ctx, dst, url, timeout, c)
}

// Post 发送给定参数的 POST 请求至给定的 url。
//
// dst 的内容将被响应体替换并返回，若 dst 过小将分配一个新切片。
//
// 该函数遵循重定向。使用 Do* 可手动处理重定向。
//
// 若 postArgs 为空，则发送空 POST 正文。
func (c *HostClient) Post(ctx context.Context, dst []byte, url string, postArgs *protocol.Args) (statusCode int, body []byte, err error) {
	return client.PostURL(ctx, dst, url, postArgs, c)
}

var startTimeUnix = time.Now().Unix()

// LastUseTime 返回客户端上次使用的时间。
func (c *HostClient) LastUseTime() time.Time {
	n := atomic.LoadUint32(&c.lastUseTime)
	return time.Unix(startTimeUnix+int64(n), 0)
}

// PendingRequests 返回客户端正在执行的当前请求数。
//
// 该函数可用于平衡多个 HostClient 之间的负载。
func (c *HostClient) PendingRequests() int {
	return int(atomic.LoadInt32(&c.pendingRequests))
}

func (c *HostClient) SetDynamicConfig(dc *client.DynamicConfig) {
	c.Addr = dc.Addr
	c.ProxyURI = dc.ProxyURI
	c.IsTLS = dc.IsTLS

	// 设置 addr 后开始观察，以避免数据竞争
	if c.StateObserve != nil {
		go func() {
			t := time.NewTicker(c.ObservationInterval)
			for {
				select {
				case <-c.closed:
					return
				case <-t.C:
					c.StateObserve(c)
				}
			}
		}()
	}
}

// SetMaxConns 设置可以建立到 Addr 中列出的所有主机的最大连接数。
func (c *HostClient) SetMaxConns(newMaxConns int) {
	c.connsLock.Lock()
	c.MaxConns = newMaxConns
	c.connsLock.Unlock()
}

// ShouldRemove 若连接数为 0，则可移除。
func (c *HostClient) ShouldRemove() bool {
	c.connsLock.Lock()
	defer c.connsLock.Unlock()
	return c.connsCount == 0
}

// WantConnectionCount 返回等待状态的连接数。
func (c *HostClient) WantConnectionCount() (count int) {
	return c.connsWait.len()
}

func (c *HostClient) decConnsCount() {
	if c.MaxConnWaitTimeout <= 0 {
		c.connsLock.Lock()
		c.connsCount--
		c.connsLock.Unlock()
		return
	}

	c.connsLock.Lock()
	defer c.connsLock.Unlock()
	dialed := false
	if q := c.connsWait; q != nil && q.len() > 0 {
		for q.len() > 0 {
			w := q.popFront()
			if w.waiting() {
				go c.dialConnFor(w)
				dialed = true
				break
			}
		}
	}
	if !dialed {
		c.connsCount--
	}
}

func (c *HostClient) releaseConn(cc *clientConn) {
	cc.lastUseTime = time.Now()
	if c.MaxConnWaitTimeout <= 0 {
		c.connsLock.Lock()
		c.conns = append(c.conns, cc)
		c.connsLock.Unlock()
		return
	}

	// 尝试将闲置连接传递给正在等待的连接
	c.connsLock.Lock()
	defer c.connsLock.Unlock()
	delivered := false
	if q := c.connsWait; q != nil && q.len() > 0 {
		for q.len() > 0 {
			w := q.popFront()
			if w.waiting() {
				delivered = w.tryDeliver(cc, nil)
				break
			}
		}
	}
	// 传递失败则追加
	if !delivered {
		c.conns = append(c.conns, cc)
	}
}

func (c *HostClient) dialConnFor(w *wantConn) {
	conn, err := c.dialHostHard(c.DialTimeout)
	if err != nil {
		w.tryDeliver(nil, err)
		c.decConnsCount()
		return
	}

	cc := acquireClientConn(conn)
	delivered := w.tryDeliver(cc, nil)
	if !delivered {
		// 未送达，返回闲置连接
		c.releaseConn(cc)
	}
}

func (c *HostClient) dialHostHard(dialTimeout time.Duration) (conn network.Conn, err error) {
	// 在放弃之前尝试拨打所有可用的主机

	c.addrsLock.Lock()
	n := len(c.addrs)
	c.addrsLock.Unlock()

	if n == 0 {
		// 看起来 c.addrs 尚未初始化
		n = 1
	}

	deadline := time.Now().Add(dialTimeout)
	for n > 0 {
		addr := c.nextAddr()
		tlsConfig := c.cachedTLSConfig(addr)
		conn, err = dialAddr(addr, c.Dialer, c.DialDualStack, tlsConfig, dialTimeout, c.ProxyURI, c.IsTLS)
		if err == nil {
			return conn, nil
		}
		if time.Since(deadline) >= 0 {
			break
		}
		n--
	}
	return nil, err
}

func dialAddr(addr string, dial network.Dialer, dialDualStack bool, tlsConfig *tls.Config, timeout time.Duration, proxyURI *protocol.URI, isTLS bool) (network.Conn, error) {
	var conn network.Conn
	var err error
	if dial == nil {
		wlog.SystemLogger().Warn("HostClient: 未指定拨号器，尝试使用默认拨号器")
		dial = dialer.DefaultDialer()
	}
	dialFunc := dial.DialConnection

	// 地址已有端口号，此处无需操作
	if proxyURI != nil {
		// 先用 tcp 连接，代理将向其添加 TLS
		conn, err = dialFunc("tcp", string(proxyURI.Host()), timeout, nil)
	} else {
		conn, err = dialFunc("tcp", addr, timeout, tlsConfig)
	}

	if err != nil {
		return nil, err
	}
	if conn == nil {
		panic("BUG: dial.DialConnection 返回了 (nil, nil)")
	}

	if proxyURI != nil {
		conn, err = proxy.SetupProxy(conn, addr, proxyURI, tlsConfig, isTLS, dial)
	}

	// conn 获取失败时必须为空，故无需关闭
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (c *HostClient) nextAddr() string {
	c.addrsLock.Lock()
	if c.addrs == nil {
		c.addrs = strings.Split(c.Addr, ",")
	}
	addr := c.addrs[0]
	if len(c.addrs) > 1 {
		addr = c.addrs[c.addrIdx%uint32(len(c.addrs))]
		c.addrIdx++
	}
	c.addrsLock.Unlock()
	return addr
}

func (c *HostClient) cachedTLSConfig(addr string) *tls.Config {
	var cfgAddr string
	if c.ProxyURI != nil && bytes.Equal(c.ProxyURI.Scheme(), bytestr.StrHTTPS) {
		cfgAddr = bytesconv.B2s(c.ProxyURI.Host())
	}

	if c.IsTLS && cfgAddr == "" {
		cfgAddr = addr
	}

	if cfgAddr == "" {
		return nil
	}

	c.tlsConfigMapLock.Lock()
	if c.tlsConfigMap == nil {
		c.tlsConfigMap = make(map[string]*tls.Config)
	}
	cfg := c.tlsConfigMap[cfgAddr]
	if cfg == nil {
		cfg = newClientTLSConfig(c.TLSConfig, cfgAddr)
		c.tlsConfigMap[cfgAddr] = cfg
	}
	c.tlsConfigMapLock.Unlock()

	return cfg
}

func (c *HostClient) do(req *protocol.Request, resp *protocol.Response) (bool, error) {
	nilResp := false
	if resp == nil {
		nilResp = true
		resp = protocol.AcquireResponse()
	}

	canIdempotentRetry, err := c.doNonNilReqResp(req, resp)

	if nilResp {
		protocol.ReleaseResponse(resp)
	}

	return canIdempotentRetry, err
}

func (c *HostClient) doNonNilReqResp(req *protocol.Request, resp *protocol.Response) (bool, error) {
	if req == nil {
		panic("BUG: req 不能为空")
	}
	if resp == nil {
		panic("BUG: resp 不能为空")
	}

	atomic.StoreUint32(&c.lastUseTime, uint32(time.Now().Unix()-startTimeUnix))

	rc := c.preHandleConfig(req.Options())

	// 先释放请求占用的资源，再发送请求，以便 GC 回收（如响应体）。
	// 在明确设置 SkipBody 的情况下，要备份起来。
	customSkipBody := resp.SkipBody
	resp.Reset()
	resp.SkipBody = customSkipBody

	if c.DisablePathNormalizing {
		req.URI().DisablePathNormalizing = true
	}
	reqTimeout := req.Options().RequestTimeout()
	begin := req.Options().StartTime()

	dialTimeout := rc.dialTimeout
	if (reqTimeout > 0 && reqTimeout < dialTimeout) || dialTimeout == 0 {
		dialTimeout = reqTimeout
	}
	cc, inPool, err := c.acquireConn(dialTimeout)
	// 若获取连接出错，立即返回错误
	if err != nil {
		return false, err
	}
	conn := cc.c

	// 设置代理网址和鉴权标头
	usingProxy := false
	if c.ProxyURI != nil && bytes.Equal(req.Scheme(), bytestr.StrHTTP) {
		usingProxy = true
		proxy.SetProxyAuthHeader(&req.Header, c.ProxyURI)
	}

	resp.ParseNetAddr(conn)

	// 关闭请求超时的连接
	shouldClose, timeout := updateReqTimeout(reqTimeout, rc.writeTimeout, begin)
	if shouldClose {
		c.closeConn(cc)
		return false, errTimeout
	}

	// 设置写入超时
	if err = conn.SetWriteTimeout(timeout); err != nil {
		c.closeConn(cc)
		// 尝试其他连接，若重试启用的话。
		return true, err
	}

	// 设置超时的长连接为关闭状态
	resetConnection := false
	if c.MaxConnDuration > 0 && time.Since(cc.createdTime) > c.MaxConnDuration && !req.ConnectionClose() {
		req.SetConnectionClose()
		resetConnection = true
	}

	// 设置 UA
	userAgentOld := req.Header.UserAgent()
	if len(userAgentOld) == 0 {
		req.Header.SetUserAgentBytes(c.getClientName())
	}

	// 将请求写入连接
	zw := c.acquireWriter(conn)
	if !usingProxy {
		err = reqI.Write(req, zw)
	} else {
		err = reqI.ProxyWrite(req, zw)
	}
	if resetConnection {
		req.Header.ResetConnectionClose()
	}

	if err == nil {
		err = zw.Flush()
	}
	// 错误发生于写入请求时，关闭连接，重试其他连接（若启用重试）
	if err != nil {
		defer c.closeConn(cc)

		errNorm, ok := conn.(network.ErrorNormalization)
		if ok {
			err = errNorm.ToWindError(err)
		}

		if !errors.Is(err, errs.ErrConnectionClosed) {
			return true, err
		}

		// 设置保护时间，以免无限循环
		if conn.SetReadTimeout(time.Second) != nil {
			return true, err
		}

		// 仅限于写入请求时链接被关闭的时候。尝试解析响应并返回。
		// 在此情况下，请求/响应被认为是成功的。
		// 否则，返回先前的错误。
		zr := c.acquireReader(conn)
		defer zr.Release()
		if respI.ReadHeaderAndLimitBody(resp, zr, c.MaxResponseBodySize) == nil {
			return false, nil
		}

		if inPool {
			err = errs.ErrBadPoolConn
		}
		return true, err
	}

	// 关闭请求超时的连接
	shouldClose, timeout = updateReqTimeout(reqTimeout, rc.readTimeout, begin)
	if shouldClose {
		c.closeConn(cc)
		return false, errTimeout
	}

	// 设置读超时，因为 golang 已修复了这个性能问题
	// 详见 https://github.com/golang/go/issues/15133#issuecomment-271571395
	if err = conn.SetReadTimeout(timeout); err != nil {
		c.closeConn(cc)
		// 重试其他连接（若启用重试）
		return true, err
	}

	// 按需跳过正文处理
	if customSkipBody || req.Header.IsHead() || req.Header.IsConnect() {
		resp.SkipBody = true
	}
	if c.DisableHeaderNamesNormalizing {
		resp.Header.DisableNormalizing()
	}
	zr := c.acquireReader(conn)

	// errs.ErrBadPoolConn 错误是在 peek 1字节读取失败时返回的，我们实际上预期会有响应。
	// 通常，这只是由于固有的关闭 keep-alive 产生的竞争，即服务器在客户端写入的同时关闭连接。
	// 底层的 err 字段通常是 io.EOF 或某种 ECONNRESET 类型的东西，它因平台而异。
	_, err = zr.Peek(1)
	if err != nil {
		zr.Release()
		c.closeConn(cc)
		// 池化连接被关闭
		if inPool && (err == io.EOF || err == syscall.ECONNRESET) {
			return true, errs.ErrBadPoolConn
		}

		// 若非池化连接，我们不该重试以避免无限死循环。
		errNorm, ok := conn.(network.ErrorNormalization)
		if ok {
			err = errNorm.ToWindError(err)
		}
		return false, err
	}

	// 此处初始化时用于在 ReadBodyStream 的闭包中传递，
	// 且该值降载读取响应头后被指派
	//
	// 这是为了解决 Response 响应和 BodyStream 正文流相互依赖的问题。
	shouldCloseConn := false

	// 真正读取响应标头和正文
	if !c.ResponseBodyStream {
		err = respI.ReadHeaderAndLimitBody(resp, zr, c.MaxResponseBodySize)
	} else {
		err = respI.ReadBodyStream(resp, zr, c.MaxResponseBodySize, func(shouldClose bool) error {
			if shouldCloseConn || shouldClose {
				c.closeConn(cc)
			} else {
				c.releaseConn(cc)
			}
			return nil
		})
	}

	if err != nil {
		zr.Release()
		c.closeConn(cc)
		// ErrBodyTooLarge 时不要重试，因为再试还一样。
		isRetry := !errors.Is(err, errs.ErrBodyTooLarge)
		return isRetry, err
	}

	zr.Release()

	shouldCloseConn = resetConnection || req.ConnectionClose() || resp.ConnectionClose()

	// 在流模式下，如果线上无内容依然可以立即关闭或释放连接。
	if c.ResponseBodyStream && resp.BodyStream() != protocol.NoResponseBody {
		return false, err
	}

	if shouldCloseConn {
		c.closeConn(cc)
	} else {
		c.releaseConn(cc)
	}

	return false, err
}

func updateReqTimeout(reqTimeout, compareTimeout time.Duration, before time.Time) (shouldCloseConn bool, timeout time.Duration) {
	if reqTimeout <= 0 {
		return false, compareTimeout
	}
	left := reqTimeout - time.Since(before)
	if left <= 0 {
		return true, 0
	}

	if compareTimeout <= 0 {
		return false, left
	}

	if left > compareTimeout {
		return false, compareTimeout
	}

	return false, left
}

type requestConfig struct {
	dialTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func (c *HostClient) preHandleConfig(o *config.RequestOptions) requestConfig {
	rc := requestConfig{
		dialTimeout:  c.DialTimeout,
		readTimeout:  c.ReadTimeout,
		writeTimeout: c.WriteTimeout,
	}
	if o.ReadTimeout() > 0 {
		rc.readTimeout = o.ReadTimeout()
	}
	if o.WriteTimeout() > 0 {
		rc.writeTimeout = o.WriteTimeout()
	}
	if o.DialTimeout() > 0 {
		rc.dialTimeout = o.DialTimeout()
	}

	return rc
}

func (c *HostClient) acquireConn(dialTimeout time.Duration) (cc *clientConn, inPool bool, err error) {
	createConn := false
	startCleaner := false

	var n int
	c.connsLock.Lock()
	n = len(c.conns)
	if n == 0 {
		maxConns := c.MaxConns
		if maxConns <= 0 {
			maxConns = consts.DefaultMaxConnsPerHost
		}
		if c.connsCount < maxConns {
			c.connsCount++
			createConn = true
			if !c.connsCleanerRun {
				startCleaner = true
				c.connsCleanerRun = true
			}
		}
	} else {
		n--
		cc = c.conns[n]
		c.conns[n] = nil
		c.conns = c.conns[:n]
	}
	c.connsLock.Unlock()

	if cc != nil {
		return cc, true, nil
	}
	if !createConn {
		if c.MaxConnWaitTimeout <= 0 {
			return nil, true, errs.ErrNoFreeConns
		}

		timeout := c.MaxConnWaitTimeout

		// 等待一个可用连接
		tc := timer.AcquireTimer(timeout)
		defer timer.ReleaseTimer(tc)

		w := &wantConn{
			ready: make(chan struct{}, 1),
		}
		defer func() {
			if err != nil {
				w.cancel(c, err)
			}
		}()

		// 注意：在设置 MaxConnWaitTimeout 的情况下，
		// 如果连接池中的连接数超过了最大连接数，并且
		// 需要在等待时建立连接，则使用 HostClient 的拨号超时，
		// 而不是请求选项中的拨号超时。
		c.queueForIdle(w)

		select {
		case <-w.ready:
			return w.conn, true, w.err
		case <-tc.C:
			return nil, true, errs.ErrNoFreeConns
		}
	}

	if startCleaner {
		go c.connsCleaner()
	}

	conn, err := c.dialHostHard(dialTimeout)
	if err != nil {
		c.decConnsCount()
		return nil, false, err
	}
	cc = acquireClientConn(conn)

	return cc, false, nil
}

func (c *HostClient) connsCleaner() {
	var (
		scratch             []*clientConn
		maxIdleConnDuration = c.MaxIdleConnDuration
	)
	if maxIdleConnDuration <= 0 {
		maxIdleConnDuration = consts.DefaultMaxIdleConnDuration
	}
	for {
		currentTime := time.Now()

		// 确定需要关闭的闲置连接
		c.connsLock.Lock()
		conns := c.conns
		n := len(conns)
		i := 0

		for i < n && currentTime.Sub(conns[i].lastUseTime) > maxIdleConnDuration {
			i++
		}
		sleepFor := maxIdleConnDuration
		if i < n {
			// +1 以便超过过期时间
			// 否则上面的 > 检查仍会失败。
			sleepFor = maxIdleConnDuration - currentTime.Sub(conns[i].lastUseTime) + 1
		}
		scratch = append(scratch[:0], conns[:i]...)
		if i > 0 {
			m := copy(conns, conns[i:])
			for i = m; i < n; i++ {
				conns[i] = nil
			}
			c.conns = conns[:m]
		}
		c.connsLock.Unlock()

		// 关闭闲置连接。
		for i, cc := range scratch {
			c.closeConn(cc)
			scratch[i] = nil
		}

		// 确定是否要停止连接清理器
		c.connsLock.Lock()
		mustStop := c.connsCount == 0
		if mustStop {
			c.connsCleanerRun = false
		}
		c.connsLock.Unlock()
		if mustStop {
			break
		}

		time.Sleep(sleepFor)
	}
}

func (c *HostClient) getClientName() []byte {
	v := c.clientName.Load()
	var clientName []byte
	if v == nil {
		clientName = []byte(c.Name)
		if len(clientName) == 0 && !c.NoDefaultUserAgentHeader {
			clientName = bytestr.DefaultUserAgent
		}
		c.clientName.Store(clientName)
	} else {
		clientName = v.([]byte)
	}
	return clientName
}

func (c *HostClient) acquireWriter(conn network.Conn) network.Writer {
	return conn
}

func (c *HostClient) acquireReader(conn network.Conn) network.Reader {
	return conn
}

func (c *HostClient) queueForIdle(w *wantConn) {
	c.connsLock.Lock()
	defer c.connsLock.Unlock()
	if c.connsWait == nil {
		c.connsWait = &wantConnQueue{}
	}
	c.connsWait.clearFront()
	c.connsWait.pushBack(w)
}

func newClientTLSConfig(c *tls.Config, addr string) *tls.Config {
	if c == nil {
		c = &tls.Config{}
	} else {
		c = c.Clone()
	}

	if c.ClientSessionCache == nil {
		c.ClientSessionCache = tls.NewLRUClientSessionCache(0)
	}

	if len(c.ServerName) == 0 {
		serverName := tlsServerName(addr)
		if serverName == "*" {
			c.InsecureSkipVerify = true
		} else {
			c.ServerName = serverName
		}
	}

	return c
}

func tlsServerName(addr string) string {
	if !strings.Contains(addr, ":") {
		return addr
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "*"
	}
	return host
}

func acquireClientConn(conn network.Conn) *clientConn {
	v := clientConnPool.Get()
	if v == nil {
		v = &clientConn{}
	}
	cc := v.(*clientConn)
	cc.c = conn
	cc.createdTime = time.Now()
	return cc
}

func releaseClientConn(cc *clientConn) {
	// 重设所有字段。
	*cc = clientConn{}
	clientConnPool.Put(cc)
}

type clientConn struct {
	c network.Conn

	createdTime time.Time
	lastUseTime time.Time
}

type wantConn struct {
	ready chan struct{}
	mu    sync.Mutex // 保护 conn, err, close(ready)
	conn  *clientConn
	err   error
}

// 尝试传递 conn, err 给当前 w，并汇报是否成功。
func (w *wantConn) tryDeliver(conn *clientConn, err error) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil || w.err != nil {
		return false
	}
	w.conn = conn
	w.err = err
	if w.conn == nil && w.err == nil {
		panic("wind: 内部错误: 滥用 tryDeliver")
	}
	close(w.ready)
	return true
}

// 返回该等待连接是否还在等待答案（连接或错误）
func (w *wantConn) waiting() bool {
	select {
	case <-w.ready:
		return false
	default:
		return true
	}
}

// 标记 w 不在等待结果（如：取消了）
// 如果连接已被传递，则调用 HostClient.releaseConn 进行释放。
func (w *wantConn) close(c *HostClient, err error) {
	w.mu.Lock()
	if w.conn == nil && w.err == nil {
		close(w.ready) // 在未来传递中发现不当行为
	}

	conn := w.conn
	w.conn = nil
	w.err = err
	w.mu.Unlock()

	if conn != nil {
		c.releaseConn(conn)
	}
}

// cancel 标记 w 不在等待结果（例如，由于取消）。
// 如果已经交付了连接，cancel 会将其与 c.releaseConn 一起返回。
func (w *wantConn) cancel(c *HostClient, err error) {
	w.mu.Lock()
	if w.conn == nil && w.err == nil {
		close(w.ready) // 在未来交付中发现不当行为
	}

	conn := w.conn
	w.conn = nil
	w.err = err
	w.mu.Unlock()

	if conn != nil {
		c.releaseConn(conn)
	}
}

// 是一个 wantConn 队列。
//
// 灵感来自 net/http/transport.go
type wantConnQueue struct {
	// This is a queue, not a deque.
	// It is split into two stages - head[headPos:] and tail.
	// popFront is trivial (headPos++) on the first stage, and
	// pushBack is trivial (append) on the second stage.
	// If the first stage is empty, popFront can swap the
	// first and second stages to remedy the situation.
	//
	// This two-stage split is analogous to the use of two lists
	// in Okasaki's purely functional queue but without the
	// overhead of reversing the list when swapping stages.
	head    []*wantConn
	headPos int
	tail    []*wantConn
}

// 返回队列中的连接数。
func (q *wantConnQueue) len() int {
	return len(q.head) - q.headPos + len(q.tail)
}

// 返回队首的等待连接但不删除它。
func (q *wantConnQueue) peekFront() *wantConn {
	if q.headPos < len(q.head) {
		return q.head[q.headPos]
	}
	if len(q.tail) > 0 {
		return q.tail[0]
	}
	return nil
}

// 移除并返回队首的等待连接。
func (q *wantConnQueue) popFront() *wantConn {
	if q.headPos >= len(q.head) {
		if len(q.tail) == 0 {
			return nil
		}
		// 把尾巴当做新的头，并清掉尾巴。
		q.head, q.headPos, q.tail = q.tail, 0, q.head[:0]
	}

	w := q.head[q.headPos]
	q.head[q.headPos] = nil
	q.headPos++
	return w
}

// 添加 w 至队列尾部。
func (q *wantConnQueue) pushBack(w *wantConn) {
	q.tail = append(q.tail, w)
}

// 清掉队首不再等待的连接，并返回其是否已被弹出。
func (q *wantConnQueue) clearFront() (cleaned bool) {
	for {
		w := q.peekFront()
		if w == nil || w.waiting() {
			return cleaned
		}
		q.popFront()
		cleaned = true
	}
}
