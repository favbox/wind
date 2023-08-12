package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/favbox/wind/common/config"
	"github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/common/hlog"
	"github.com/favbox/wind/common/utils"
	"github.com/favbox/wind/internal/bytestr"
	"github.com/favbox/wind/internal/nocopy"
	"github.com/favbox/wind/network/dialer"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/client"
	"github.com/favbox/wind/protocol/consts"
	"github.com/favbox/wind/protocol/http1"
	"github.com/favbox/wind/protocol/http1/factory"
	"github.com/favbox/wind/protocol/suite"
)

var (
	errorInvalidURI          = errors.NewPublic("无效的网址")
	errorLastMiddlewareExist = errors.NewPublic("最后一个中间件已设置")
)

var defaultClient, _ = NewClient(WithDialTimeout(consts.DefaultDialTimeout))

// Do 执行给定的 http 请求并填充给定的 http 响应。
//
// Request 至少包含非空的完整网址（包括方案和主机）或非空的主机头+请求网址。
//
// Client 确定待请求服务器的顺序如下：
//
//   - 先从 RequestURI 确定：如果请求网址包含完整的方案和主机；
//   - 其他情况按主机标头确定。
//
// 若 resp 为空，则不处理 Response。
//
// 该函数不遵循重定向。用 Get* 做重定向。
//
// ErrNoFreeConns 将在到主机的所有 HostClient.MaxConns 连接都繁忙时返回。
//
// 推荐获取 req 和 resp 的方式为 AcquireRequest 和 AcquireResponse，在性能关键代码中可提升性能。
func Do(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
	return defaultClient.Do(ctx, req, resp)
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
func DoDeadline(ctx context.Context, req *protocol.Request, resp *protocol.Response, deadline time.Time) error {
	return defaultClient.DoDeadline(ctx, req, resp, deadline)
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
func DoRedirects(ctx context.Context, req *protocol.Request, resp *protocol.Response, maxRedirectsCount int) error {
	_, _, err := client.DoRequestFollowRedirects(ctx, req, resp, req.URI().String(), maxRedirectsCount, defaultClient)
	return err
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
func DoTimeout(ctx context.Context, req *protocol.Request, resp *protocol.Response, timeout time.Duration) error {
	return defaultClient.DoTimeout(ctx, req, resp, timeout)
}

// Get 返回给定网址的状态码和响应体。
//
// dst 的内容将被响应体替换并返回，若 dst 过小将分配一个新切片。
//
// 该函数遵循重定向。使用 Do* 可手动处理重定向。
func Get(ctx context.Context, dst []byte, url string, requestOptions ...config.RequestOption) (statusCode int, body []byte, err error) {
	return defaultClient.Get(ctx, dst, url, requestOptions...)
}

// GetTimeout 返回给定网址的状态码和响应体。
//
// dst 的内容将被响应体替换并返回，若 dst 过小将分配一个新切片。
//
// 该函数遵循重定向。使用 Do* 可手动处理重定向。
//
// errTimeout 将在触达超时时长后被返回。
func GetTimeout(ctx context.Context, dst []byte, url string, timeout time.Duration, requestOptions ...config.RequestOption) (statusCode int, body []byte, err error) {
	return defaultClient.GetTimeout(ctx, dst, url, timeout, requestOptions...)
}

// GetDeadline 返回给定网址的状态码和响应体。
//
// dst 的内容将被响应体替换并返回，若 dst 过小将分配一个新切片。
//
// 该函数遵循重定向。使用 Do* 可手动处理重定向。
//
// errTimeout 将在截止时间到达后被返回。
func GetDeadline(ctx context.Context, dst []byte, url string, deadline time.Time, requestOptions ...config.RequestOption) (statusCode int, body []byte, err error) {
	return defaultClient.GetDeadline(ctx, dst, url, deadline, requestOptions...)
}

// Post 使用给定参数向指定网址发送 POST 请求。
//
// dst 的内容将被响应体替换并返回，若 dst 过小将分配一个新切片。
//
// 该函数遵循重定向。使用 Do* 可手动处理重定向。
func Post(ctx context.Context, dst []byte, url string, postArgs *protocol.Args, requestOptions ...config.RequestOption) (statusCode int, body []byte, err error) {
	return defaultClient.Post(ctx, dst, url, postArgs, requestOptions...)
}

// Client 实现 http 客户端。
//
// 禁止值拷贝 Client。可新建实例。
//
// Client 的方法是协程安全的。
type Client struct {
	noCopy nocopy.NoCopy

	options *config.ClientOptions

	// Proxy 返回给定请求的代理网址。 若出错将中止请求。
	//
	// 代理类型由 URL 方案决定，支持 "http" 和 "https"，若方案为空，则假定 "http"。
	//
	// 若 Proxy 为空或返回的 *URL 为空，则不使用代理。
	Proxy protocol.Proxy

	// 设置重试决策函数。若为空，则应用 client.DefaultRetryIf。
	RetryIfFunc client.RetryIfFunc

	clientFactory suite.ClientFactory

	mLock          sync.Mutex
	m              map[string]client.HostClient // http 主机对应的主机客户端
	ms             map[string]client.HostClient // https 主机对应的主机客户端
	mws            Middleware
	lastMiddleware Middleware
}

// NewClient 创建给定选项的客户端。
func NewClient(opts ...config.ClientOption) (*Client, error) {
	opt := config.NewClientOptions(opts)
	if opt.Dialer == nil {
		opt.Dialer = dialer.DefaultDialer()
	}
	c := &Client{
		options: opt,
		m:       make(map[string]client.HostClient),
		ms:      make(map[string]client.HostClient),
	}

	return c, nil
}

// CloseIdleConnections 关闭先前建立而当前闲置的长连接。
// 不会中断当前使用中的连接。
func (c *Client) CloseIdleConnections() {
	c.mLock.Lock()
	for _, hostClient := range c.m {
		hostClient.CloseIdleConnections()
	}
	c.mLock.Unlock()
}

// Do 执行给定的 http 请求并填充给定的 http 响应。
//
// Request 至少包含非空的完整网址（包括方案和主机）或非空的主机头+请求网址。
//
// Client 确定待请求服务器的顺序如下：
//
//   - 先从 RequestURI 确定：如果请求网址包含完整的方案和主机；
//   - 其他情况按主机标头确定。
//
// 若 resp 为空，则不处理 Response。
//
// 该函数不遵循重定向。用 Get* 做重定向。
//
// ErrNoFreeConns 将在到主机的所有 HostClient.MaxConns 连接都繁忙时返回。
//
// 推荐获取 req 和 resp 的方式为 AcquireRequest 和 AcquireResponse，在性能关键代码中可提升性能。
func (c *Client) Do(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
	if c.mws == nil {
		return c.do(ctx, req, resp)
	}
	if c.lastMiddleware != nil {
		return c.mws(c.lastMiddleware(c.do))(ctx, req, resp)
	}
	return c.mws(c.do)(ctx, req, resp)
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
func (c *Client) DoDeadline(ctx context.Context, req *protocol.Request, resp *protocol.Response, deadline time.Time) error {
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
func (c *Client) DoTimeout(ctx context.Context, req *protocol.Request, resp *protocol.Response, timeout time.Duration) error {
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
func (c *Client) DoRedirects(ctx context.Context, req *protocol.Request, resp *protocol.Response, maxRedirectsCount int) error {
	_, _, err := client.DoRequestFollowRedirects(ctx, req, resp, req.URI().String(), maxRedirectsCount, c)
	return err
}

// Get 返回给定网址的状态码和响应体。
//
// dst 的内容将被响应体替换并返回，若 dst 过小将分配一个新切片。
//
// 该函数遵循重定向。使用 Do* 可手动处理重定向。
func (c *Client) Get(ctx context.Context, dst []byte, url string, requestOptions ...config.RequestOption) (statusCode int, body []byte, err error) {
	return client.GetURL(ctx, dst, url, c, requestOptions...)
}

// GetDeadline 返回给定网址的状态码和响应体。
//
// dst 的内容将被响应体替换并返回，若 dst 过小将分配一个新切片。
//
// 该函数遵循重定向。使用 Do* 可手动处理重定向。
//
// errTimeout 将在截止时间到达后被返回。
func (c *Client) GetDeadline(ctx context.Context, dst []byte, url string, deadline time.Time, requestOptions ...config.RequestOption) (statusCode int, body []byte, err error) {
	return client.GetURLDeadline(ctx, dst, url, deadline, c, requestOptions...)
}

// GetTimeout 返回给定网址的状态码和响应体。
//
// dst 的内容将被响应体替换并返回，若 dst 过小将分配一个新切片。
//
// 该函数遵循重定向。使用 Do* 可手动处理重定向。
//
// errTimeout 将在触达超时时长后被返回。
func (c *Client) GetTimeout(ctx context.Context, dst []byte, url string, timeout time.Duration, requestOptions ...config.RequestOption) (statusCode int, body []byte, err error) {
	return client.GetURLTimeout(ctx, dst, url, timeout, c, requestOptions...)
}

// GetOptions 获取客户端选项。
func (c *Client) GetOptions() *config.ClientOptions {
	return c.options
}

// GetDialerName 获取拨号器名称。
func (c *Client) GetDialerName() (dName string, err error) {
	defer func() {
		err := recover()
		if err != nil {
			dName = "unknown"
		}
	}()

	opt := c.GetOptions()
	if opt == nil || opt.Dialer == nil {
		return "", fmt.Errorf("异常处理：无客户端选项或拨号器")
	}

	dName = reflect.TypeOf(opt.Dialer).String()
	dSlice := strings.Split(dName, ".")
	dName = dSlice[0]
	if dName[0] == '*' {
		dName = dName[1:]
	}

	return
}

// Post 使用给定参数向指定网址发送 POST 请求。
//
// dst 的内容将被响应体替换并返回，若 dst 过小将分配一个新切片。
//
// 该函数遵循重定向。使用 Do* 可手动处理重定向。
func (c *Client) Post(ctx context.Context, dst []byte, url string, postArgs *protocol.Args, requestOptions ...config.RequestOption) (statusCode int, body []byte, err error) {
	return client.PostURL(ctx, dst, url, postArgs, c, requestOptions...)
}

// SetClientFactory 设置客户端工厂。
func (c *Client) SetClientFactory(cf suite.ClientFactory) {
	c.clientFactory = cf
}

// SetProxy 设置客户端代理。
//
// 不要为一个客户端设置两次代理。
// 如果想用另一个代理，请创建其他客户端并把代理给它。
func (c *Client) SetProxy(p protocol.Proxy) {
	c.Proxy = p
}

// SetRetryIfFunc 设置重试决策函数。
func (c *Client) SetRetryIfFunc(retryIf client.RetryIfFunc) {
	c.RetryIfFunc = retryIf
}

// TakeOutLastMiddleware 返回最后一个中间件并从 Client 中移除。
//
// 记得在把它和其他中间件 chain 连接后放回原位。
func (c *Client) TakeOutLastMiddleware() Middleware {
	last := c.lastMiddleware
	c.lastMiddleware = nil
	return last
}

// Use 追加客户端中间件。
func (c *Client) Use(mws ...Middleware) {
	// 将原中间件放在前面
	middlewares := make([]Middleware, 0, 1+len(mws))
	if c.mws != nil {
		middlewares = append(middlewares, c.mws)
	}
	middlewares = append(middlewares, mws...)
	c.mws = chain(middlewares...)
}

// UseAsLast 将给定的中间件作为最后的中间件使用。
//
// 如果之前已设置最后中间件，将返回错误，以确保所有中间件都可以工作。
//
// 请使用 TakeOutLastMiddleware 取出已设置的中间件。
//
// 在之后或之前链接中间件都可以，但请记住将其放回原处。
func (c *Client) UseAsLast(mw Middleware) error {
	if c.lastMiddleware != nil {
		return errorLastMiddlewareExist
	}
	c.lastMiddleware = mw
	return nil
}

func (c *Client) do(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
	if !c.options.KeepAlive {
		req.Header.SetConnectionClose(true)
	}
	uri := req.URI()
	if uri == nil {
		return errorInvalidURI
	}

	var proxyURI *protocol.URI
	var err error

	if c.Proxy != nil {
		proxyURI, err = c.Proxy(req)
		if err != nil {
			return fmt.Errorf("获取请求的代理网址出错=%w", err)
		}
	}

	isTLS := false
	schema := uri.Scheme()
	if bytes.Equal(schema, bytestr.StrHTTPS) {
		isTLS = true
	} else if !bytes.Equal(schema, bytestr.StrHTTP) && !bytes.Equal(schema, bytestr.StrSD) {
		return fmt.Errorf("不支持的协议 %q。支持 http 和 https", schema)
	}

	host := uri.Host()
	startCleaner := false

	// 获取主机对应的客户端，并按需启动清理器
	c.mLock.Lock()
	m := c.m
	if isTLS {
		m = c.ms
	}
	h := string(host)
	hc := m[h]
	if hc == nil {
		if c.clientFactory == nil {
			// 默认加载 http1 客户端
			c.clientFactory = factory.NewClientFactory(newHttp1OptionFromClient(c))
		}
		hc, _ = c.clientFactory.NewHostClient()
		hc.SetDynamicConfig(&client.DynamicConfig{
			Addr:     utils.AddMissingPort(h, isTLS),
			ProxyURI: proxyURI,
			IsTLS:    isTLS,
		})
		m[h] = hc
		if len(m) == 1 {
			startCleaner = true
		}
	}
	c.mLock.Unlock()

	if startCleaner {
		go c.mCleaner()
	}

	return hc.Do(ctx, req, resp)
}

func (c *Client) mCleaner() {
	mustStop := false

	for {
		time.Sleep(10 * time.Second)
		c.mLock.Lock()
		for host, hostClient := range c.m {
			if hostClient.ShouldRemove() {
				delete(c.m, host)
				if f, ok := hostClient.(io.Closer); ok {
					err := f.Close()
					if err != nil {
						hlog.Warnf("清理 hostClient 出错，地址：%s，错误：%s", host, err.Error())
					}
				}
			}
		}
		if len(c.m) == 0 {
			mustStop = true
		}
		c.mLock.Unlock()

		if mustStop {
			break
		}
	}
}

func newHttp1OptionFromClient(c *Client) *http1.ClientOptions {
	return &http1.ClientOptions{
		Name:                          c.options.Name,
		NoDefaultUserAgentHeader:      c.options.NoDefaultUserAgentHeader,
		Dialer:                        c.options.Dialer,
		DialTimeout:                   c.options.DialTimeout,
		DialDualStack:                 c.options.DialDualStack,
		TLSConfig:                     c.options.TLSConfig,
		MaxConns:                      c.options.MaxConnsPerHost,
		MaxConnDuration:               c.options.MaxConnDuration,
		MaxIdleConnDuration:           c.options.MaxIdleConnDuration,
		ReadTimeout:                   c.options.ReadTimeout,
		WriteTimeout:                  c.options.WriteTimeout,
		MaxResponseBodySize:           c.options.MaxResponseBodySize,
		DisableHeaderNamesNormalizing: c.options.DisableHeaderNamesNormalizing,
		DisablePathNormalizing:        c.options.DisablePathNormalizing,
		MaxConnWaitTimeout:            c.options.MaxConnWaitTimeout,
		ResponseBodyStream:            c.options.ResponseBodyStream,
		RetryConfig:                   c.options.RetryConfig,
		RetryIfFunc:                   c.RetryIfFunc,
		StateObserve:                  c.options.HostClientStateObserve,
		ObservationInterval:           c.options.ObservationInterval,
	}
}
