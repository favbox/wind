package client

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/favbox/wind/common/config"
	"github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/common/timer"
	"github.com/favbox/wind/internal/bytestr"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/consts"
)

const defaultMaxRedirectsCount = 16

var (
	errTimeout          = errors.New(errors.ErrTimeout, errors.ErrorTypePublic, "host client")
	errTooManyRedirects = errors.NewPublic("执行请求时检测到太多重定向")
	errMissingLocation  = errors.NewPublic("缺少重定向的位置标头")

	clientURLResponseChPool sync.Pool
)

type Doer interface {
	Do(ctx context.Context, req *protocol.Request, resp *protocol.Response) error
}

// HostClient 用于在多个上游主机间平衡 http 请求。
type HostClient interface {
	Doer
	SetDynamicConfig(dc *DynamicConfig) // 设置动态配置
	CloseIdleConnections()              // 关闭闲置连接
	ShouldRemove() bool                 // 汇报是否应移除
	ConnectionCount() int               // 返回连接数
}

// DynamicConfig 用于请求的动态配置信息。
type DynamicConfig struct {
	Addr     string
	ProxyURI *protocol.URI
	IsTLS    bool
}

// RetryIfFunc 通过请求、响应或错误，判断是否需要重试。
type RetryIfFunc func(req *protocol.Request, resp *protocol.Response, err error) bool

type clientURLResponse struct {
	statusCode int
	body       []byte
	err        error
}

// DefaultRetryIf 默认重试条件，主要用于幂等请求。
func DefaultRetryIf(req *protocol.Request, resp *protocol.Response, err error) bool {
	// 如果请求体不可回放，则无法重试
	if req.IsBodyStream() {
		return false
	}

	// 是否为幂等请求
	if isIdempotent(req, resp, err) {
		return true
	}

	// 若服务器在发送相应之前关闭了连接，请重试非幂等请求。
	//
	// 若服务器在超时时关闭闲置长连接，则可能出现这种情况。
	//
	// Apache 和 Nginx 通常会这样做。
	if err == io.EOF {
		return true
	}

	return false
}

func isIdempotent(req *protocol.Request, resp *protocol.Response, err error) bool {
	return req.Header.IsGet() ||
		req.Header.IsHead() ||
		req.Header.IsPut() ||
		req.Header.IsDelete() ||
		req.Header.IsOptions() ||
		req.Header.IsTrace()
}

func GetURL(ctx context.Context, dst []byte, url string, c Doer, requestOptions ...config.RequestOption) (statusCode int, body []byte, err error) {
	req := protocol.AcquireRequest()
	req.SetOptions(requestOptions...)

	statusCode, body, err = doRequestFollowRedirectsBuffer(ctx, req, dst, url, c)

	protocol.ReleaseRequest(req)
	return statusCode, body, err
}

func GetURLTimeout(ctx context.Context, dst []byte, url string, timeout time.Duration, c Doer, requestOptions ...config.RequestOption) (statusCode int, body []byte, err error) {
	deadline := time.Now().Add(timeout)
	return GetURLDeadline(ctx, dst, url, deadline, c, requestOptions...)
}

func GetURLDeadline(ctx context.Context, dst []byte, url string, deadline time.Time, c Doer, requestOptions ...config.RequestOption) (statusCode int, body []byte, err error) {
	timeout := -time.Since(deadline)
	if timeout <= 0 {
		return 0, nil, errTimeout
	}

	var ch chan clientURLResponse
	chv := clientURLResponseChPool.Get()
	if chv == nil {
		chv = make(chan clientURLResponse, 1)
	}
	ch = chv.(chan clientURLResponse)

	req := protocol.AcquireRequest()
	req.SetOptions(requestOptions...)

	// 注意当 errTimeout 时请求会继续执行，直至触发客户端指定的 ReadTimeout。
	// 这有助于通过 MaxConns* 并发请求来限制慢速主机的负载。
	//
	// 如果没有这种 hack，慢速主机的负载可能会超过 MaxConn* 并发请求，
	// 因为客户端上超时的请求通常会在主机上继续执行。
	go func() {
		statusCodeCopy, bodyCopy, errCopy := doRequestFollowRedirectsBuffer(ctx, req, dst, url, c)
		ch <- clientURLResponse{
			statusCode: statusCodeCopy,
			body:       bodyCopy,
			err:        errCopy,
		}
	}()

	tc := timer.AcquireTimer(timeout)
	select {
	case resp := <-ch:
		protocol.ReleaseRequest(req)
		clientURLResponseChPool.Put(chv)
		statusCode = resp.statusCode
		body = resp.body
		err = resp.err
	case <-tc.C:
		body = dst
		err = errTimeout
	}
	timer.ReleaseTimer(tc)

	return statusCode, body, err
}

// PostURL 发送 POST 请求并将相应填充值 dst。
func PostURL(ctx context.Context, dst []byte, url string, postArgs *protocol.Args, c Doer, requestOptions ...config.RequestOption) (statusCode int, body []byte, err error) {
	req := protocol.AcquireRequest()
	req.Header.SetMethodBytes(bytestr.StrPost)
	req.Header.SetContentTypeBytes(bytestr.StrPostArgsContentType)
	req.SetOptions(requestOptions...)

	if postArgs != nil {
		if _, err = postArgs.WriteTo(req.BodyWriter()); err != nil {
			return 0, nil, err
		}
	}

	statusCode, body, err = doRequestFollowRedirectsBuffer(ctx, req, dst, url, c)

	protocol.ReleaseRequest(req)
	return
}

func DoRequestFollowRedirects(ctx context.Context, req *protocol.Request, resp *protocol.Response, url string, maxRedirectsCount int, c Doer) (statusCode int, body []byte, err error) {
	redirectsCount := 0

	for {
		req.SetRequestURI(url)
		req.ParseURI()

		if err = c.Do(ctx, req, resp); err != nil {
			break
		}
		statusCode = resp.Header.StatusCode()
		if !StatusCodeIsRedirect(statusCode) {
			break
		}

		redirectsCount++
		if redirectsCount > maxRedirectsCount {
			err = errTooManyRedirects
			break
		}
		location := resp.Header.PeekLocation()
		if len(location) == 0 {
			err = errMissingLocation
			break
		}
		url = getRedirectURL(url, location)
	}

	return
}

func DoTimeout(ctx context.Context, req *protocol.Request, resp *protocol.Response, timeout time.Duration, c Doer) error {
	if timeout <= 0 {
		return errTimeout
	}
	// 注意：它将覆盖 reqTimeout。
	req.SetOptions(config.WithRequestTimeout(timeout))
	return c.Do(ctx, req, resp)
}

func DoDeadline(ctx context.Context, req *protocol.Request, resp *protocol.Response, deadline time.Time, c Doer) error {
	timeout := time.Until(deadline)
	if timeout <= 0 {
		return errTimeout
	}
	// 注意：它将覆盖 reqTimeout。
	req.SetOptions(config.WithRequestTimeout(timeout))
	return c.Do(ctx, req, resp)
}

func StatusCodeIsRedirect(statusCode int) bool {
	return statusCode == consts.StatusMovedPermanently ||
		statusCode == consts.StatusFound ||
		statusCode == consts.StatusSeeOther ||
		statusCode == consts.StatusTemporaryRedirect ||
		statusCode == consts.StatusPermanentRedirect
}

func doRequestFollowRedirectsBuffer(ctx context.Context, req *protocol.Request, dst []byte, url string, c Doer) (statusCode int, body []byte, err error) {
	resp := protocol.AcquireResponse()
	bodyBuf := resp.BodyBuffer()
	oldBody := bodyBuf.B
	bodyBuf.B = dst

	statusCode, _, err = DoRequestFollowRedirects(ctx, req, resp, url, defaultMaxRedirectsCount, c)

	// 在 HTTP2 中，客户端使用流模式创建请求，其主体位于正文流中。
	// 在 HTTP1 中，只有客户端接收的主体大小超过了最大主体尺寸，且客户端处于流模式才能触发。
	body = resp.Body()
	bodyBuf.B = oldBody
	protocol.ReleaseResponse(resp)

	return statusCode, body, err
}

func getRedirectURL(baseURL string, location []byte) string {
	u := protocol.AcquireURI()
	u.Update(baseURL)
	u.UpdateBytes(location)
	redirectURI := u.String()
	protocol.ReleaseURI(u)
	return redirectURI
}
