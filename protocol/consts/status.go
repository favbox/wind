package consts

import (
	"fmt"
	"sync/atomic"
)

const (
	statusMessageMin = 100
	statusMessageMax = 511
)

// HTTP 状态码，来自 net/http 的部分。
const (
	StatusContinue           = 100 // RFC 7231, 6.2.1
	StatusSwitchingProtocols = 101 // RFC 7231, 6.2.2
	StatusProcessing         = 102 // RFC 2518, 10.1

	StatusOK                   = 200 // RFC 7231, 6.3.1
	StatusCreated              = 201 // RFC 7231, 6.3.2
	StatusAccepted             = 202 // RFC 7231, 6.3.3
	StatusNonAuthoritativeInfo = 203 // RFC 7231, 6.3.4
	StatusNoContent            = 204 // RFC 7231, 6.3.5
	StatusResetContent         = 205 // RFC 7231, 6.3.6
	StatusPartialContent       = 206 // RFC 7233, 4.1
	StatusMultiStatus          = 207 // RFC 4918, 11.1
	StatusAlreadyReported      = 208 // RFC 5842, 7.1
	StatusIMUsed               = 226 // RFC 3229, 10.4.1

	StatusMultipleChoices   = 300 // RFC 7231, 6.4.1
	StatusMovedPermanently  = 301 // RFC 7231, 6.4.2
	StatusFound             = 302 // RFC 7231, 6.4.3
	StatusSeeOther          = 303 // RFC 7231, 6.4.4
	StatusNotModified       = 304 // RFC 7232, 4.1
	StatusUseProxy          = 305 // RFC 7231, 6.4.5
	_                       = 306 // RFC 7231, 6.4.6 (Unused)
	StatusTemporaryRedirect = 307 // RFC 7231, 6.4.7
	StatusPermanentRedirect = 308 // RFC 7538, 3

	StatusBadRequest                   = 400 // RFC 7231, 6.5.1 客户端请求的语法错误，服务器无法理解
	StatusUnauthorized                 = 401 // RFC 7235, 3.1 客户端未通过服务端的身份验证
	StatusPaymentRequired              = 402 // RFC 7231, 6.5.2 保留，将来用于数字支付系统
	StatusForbidden                    = 403 // RFC 7231, 6.5.3 客户端没有访问内容的权限
	StatusNotFound                     = 404 // RFC 7231, 6.5.4 服务器找不到请求的资源
	StatusMethodNotAllowed             = 405 // RFC 7231, 6.5.5 目标资源不支持该请求方法
	StatusNotAcceptable                = 406 // RFC 7231, 6.5.6 服务端没发现符合用户代理给定标准的内容
	StatusProxyAuthRequired            = 407 // RFC 7235, 3.2 需要代理对客户端进行身份验证
	StatusRequestTimeout               = 408 // RFC 7231, 6.5.7 用于关闭客户端闲置连接时发送的消息
	StatusConflict                     = 409 // RFC 7231, 6.5.8 请求与服务器当前状态相冲突
	StatusGone                         = 410 // RFC 7231, 6.5.9 服务器已永久删除此内容且无转发地址
	StatusLengthRequired               = 411 // RFC 7231, 6.5.10 Content-Length 标头未定义但服务端需要它
	StatusPreconditionFailed           = 412 // RFC 7232, 4.2 服务端无法满足客户端的先决条件
	StatusRequestEntityTooLarge        = 413 // RFC 7231, 6.5.11 请求实体大于服务器定义的限制
	StatusRequestURITooLong            = 414 // RFC 7231, 6.5.12 请求 URI 的长度大于服务器定义的限制
	StatusUnsupportedMediaType         = 415 // RFC 7231, 6.5.13
	StatusRequestedRangeNotSatisfiable = 416 // RFC 7233, 4.4
	StatusExpectationFailed            = 417 // RFC 7231, 6.5.14
	StatusTeapot                       = 418 // RFC 7168, 2.3.3
	StatusUnprocessableEntity          = 422 // RFC 4918, 11.2
	StatusLocked                       = 423 // RFC 4918, 11.3
	StatusFailedDependency             = 424 // RFC 4918, 11.4
	StatusUpgradeRequired              = 426 // RFC 7231, 6.5.15
	StatusPreconditionRequired         = 428 // RFC 6585, 3
	StatusTooManyRequests              = 429 // RFC 6585, 4
	StatusRequestHeaderFieldsTooLarge  = 431 // RFC 6585, 5
	StatusUnavailableForLegalReasons   = 451 // RFC 7725, 3

	StatusInternalServerError           = 500 // RFC 7231, 6.6.1 服务器内部错误，无法完成请求
	StatusNotImplemented                = 501 // RFC 7231, 6.6.2 服务器不支持请求的功能，无法完成请求
	StatusBadGateway                    = 502 // RFC 7231, 6.6.3 网关或代理服务器从远程服务器收到了无效的响应
	StatusServiceUnavailable            = 503 // RFC 7231, 6.6.4 因维护或超载而停机。尽量标注不缓存的 Retry-After 头告知恢复时间
	StatusGatewayTimeout                = 504 // RFC 7231, 6.6.5 无法及时获得响应
	StatusHTTPVersionNotSupported       = 505 // RFC 7231, 6.6.6 服务器不支持请求中使用的 HTTP 版本
	StatusVariantAlsoNegotiates         = 506 // RFC 2295, 8.1
	StatusInsufficientStorage           = 507 // RFC 4918, 11.5
	StatusLoopDetected                  = 508 // RFC 5842, 7.2
	StatusNotExtended                   = 510 // RFC 2774, 7
	StatusNetworkAuthenticationRequired = 511 // RFC 6585, 6
)

var (
	statusLines atomic.Value

	statusMessages = map[int]string{
		StatusContinue:           "Continue",
		StatusSwitchingProtocols: "Switching Protocols",
		StatusProcessing:         "Processing",

		StatusOK:                   "OK",
		StatusCreated:              "Created",
		StatusAccepted:             "Accepted",
		StatusNonAuthoritativeInfo: "Non-Authoritative Information",
		StatusNoContent:            "No Content",
		StatusResetContent:         "Reset Content",
		StatusPartialContent:       "Partial Content",
		StatusMultiStatus:          "Multi-Status",
		StatusAlreadyReported:      "Already Reported",
		StatusIMUsed:               "IM Used",

		StatusMultipleChoices:   "Multiple Choices",
		StatusMovedPermanently:  "Moved Permanently",
		StatusFound:             "Found",
		StatusSeeOther:          "See Other",
		StatusNotModified:       "Not Modified",
		StatusUseProxy:          "Use Proxy",
		StatusTemporaryRedirect: "Temporary Redirect",
		StatusPermanentRedirect: "Permanent Redirect",

		StatusBadRequest:                   "Bad Request",
		StatusUnauthorized:                 "Unauthorized",
		StatusPaymentRequired:              "Payment Required",
		StatusForbidden:                    "Forbidden",
		StatusNotFound:                     "404 Page not found",
		StatusMethodNotAllowed:             "Method Not Allowed",
		StatusNotAcceptable:                "Not Acceptable",
		StatusProxyAuthRequired:            "Proxy Authentication Required",
		StatusRequestTimeout:               "Request Timeout",
		StatusConflict:                     "Conflict",
		StatusGone:                         "Gone",
		StatusLengthRequired:               "Length Required",
		StatusPreconditionFailed:           "Precondition Failed",
		StatusRequestEntityTooLarge:        "Request Entity Too Large",
		StatusRequestURITooLong:            "Request URI Too Long",
		StatusUnsupportedMediaType:         "Unsupported Media Type",
		StatusRequestedRangeNotSatisfiable: "Requested Range Not Satisfiable",
		StatusExpectationFailed:            "Expectation Failed",
		StatusTeapot:                       "I'm a teapot",
		StatusUnprocessableEntity:          "Unprocessable Entity",
		StatusLocked:                       "Locked",
		StatusFailedDependency:             "Failed Dependency",
		StatusUpgradeRequired:              "Upgrade Required",
		StatusPreconditionRequired:         "Precondition Required",
		StatusTooManyRequests:              "Too Many Requests",
		StatusRequestHeaderFieldsTooLarge:  "Request Header Fields Too Large",
		StatusUnavailableForLegalReasons:   "Unavailable For Legal Reasons",

		StatusInternalServerError:           "Internal Server Error",
		StatusNotImplemented:                "Not Implemented",
		StatusBadGateway:                    "Bad Gateway",
		StatusServiceUnavailable:            "Service Unavailable",
		StatusGatewayTimeout:                "Gateway Timeout",
		StatusHTTPVersionNotSupported:       "HTTP Version Not Supported",
		StatusVariantAlsoNegotiates:         "Variant Also Negotiates",
		StatusInsufficientStorage:           "Insufficient Storage",
		StatusLoopDetected:                  "Loop Detected",
		StatusNotExtended:                   "Not Extended",
		StatusNetworkAuthenticationRequired: "Network Authentication Required",
	}
)

// StatusMessage 返回指定 HTTP 状态码的状态消息。
func StatusMessage(statusCode int) string {
	if statusCode < statusMessageMin || statusCode > statusMessageMax {
		return "未知状态码"
	}

	s := statusMessages[statusCode]
	if s == "" {
		s = "未知状态码"
	}
	return s
}

func init() {
	statusLines.Store(make(map[int][]byte))
}

// StatusLine 返回指定状态码的 HTTP 状态行。
//
// 如 HTTP/1.1 200 OK\r\n
//
// 该方法在多协程中并发安全。
func StatusLine(statusCode int) []byte {
	m := statusLines.Load().(map[int][]byte)
	h := m[statusCode]
	if h != nil {
		return h
	}

	statusText := StatusMessage(statusCode)

	h = []byte(fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, statusText))
	newM := make(map[int][]byte, len(m)+1)
	for k, v := range m {
		newM[k] = v
	}
	newM[statusCode] = h
	statusLines.Store(newM)
	return h
}
