// Package bytestr 定义一些常用的字节化的字符串。
package bytestr

import "github.com/favbox/wind/protocol/consts"

var (
	DefaultServerName  = []byte("wind")
	DefaultUserAgent   = []byte("wind")
	DefaultContentType = []byte("text/plain; charset=utf-8")
)

var (
	StrBackSlash        = []byte("\\")
	StrSlash            = []byte("/")
	StrSlashSlash       = []byte("//")
	StrSlashDotDot      = []byte("/..")
	StrSlashDotSlash    = []byte("/./")
	StrSlashDotDotSlash = []byte("/../")
	StrCRLF             = []byte("\r\n")
	StrHTTP             = []byte("http")
	StrHTTPS            = []byte("https")
	StrHTTP11           = []byte("HTTP/1.1")
	StrColon            = []byte(":")
	StrStar             = []byte("*")
	StrColonSlashSlash  = []byte("://")
	StrColonSpace       = []byte(": ")
	StrCommaSpace       = []byte(", ")
	StrAt               = []byte("@")
	StrSD               = []byte("sd")

	StrResponseContinue = []byte("HTTP/1.1 100 Continue\r\n\r\n")

	StrGet     = []byte(consts.MethodGet)
	StrHead    = []byte(consts.MethodHead)
	StrPost    = []byte(consts.MethodPost)
	StrPut     = []byte(consts.MethodPut)
	StrDelete  = []byte(consts.MethodDelete)
	StrConnect = []byte(consts.MethodConnect)
	StrOptions = []byte(consts.MethodOptions)
	StrTrace   = []byte(consts.MethodTrace)
	StrPatch   = []byte(consts.MethodPatch)

	StrExpect           = []byte(consts.HeaderExpect)
	StrConnection       = []byte(consts.HeaderConnection)
	StrContentLength    = []byte(consts.HeaderContentLength)
	StrContentType      = []byte(consts.HeaderContentType)
	StrDate             = []byte(consts.HeaderDate)
	StrHost             = []byte(consts.HeaderHost)
	StrServer           = []byte(consts.HeaderServer)
	StrTransferEncoding = []byte(consts.HeaderTransferEncoding)

	StrUserAgent          = []byte(consts.HeaderUserAgent)
	StrCookie             = []byte(consts.HeaderCookie)
	StrLocation           = []byte(consts.HeaderLocation)
	StrContentRange       = []byte(consts.HeaderContentRange)
	StrContentEncoding    = []byte(consts.HeaderContentEncoding)
	StrAcceptEncoding     = []byte(consts.HeaderAcceptEncoding)
	StrSetCookie          = []byte(consts.HeaderSetCookie)
	StrAuthorization      = []byte(consts.HeaderAuthorization)
	StrRange              = []byte(consts.HeaderRange)
	StrLastModified       = []byte(consts.HeaderLastModified)
	StrAcceptRanges       = []byte(consts.HeaderAcceptRanges)
	StrIfModifiedSince    = []byte(consts.HeaderIfModifiedSince)
	StrTE                 = []byte(consts.HeaderTE)
	StrTrailer            = []byte(consts.HeaderTrailer)
	StrMaxForwards        = []byte(consts.HeaderMaxForwards)
	StrProxyConnection    = []byte(consts.HeaderProxyConnection)
	StrProxyAuthenticate  = []byte(consts.HeaderProxyAuthenticate)
	StrProxyAuthorization = []byte(consts.HeaderProxyAuthorization)
	StrWWWAuthenticate    = []byte(consts.HeaderWWWAuthenticate)

	StrCookieExpires        = []byte("expires")
	StrCookieDomain         = []byte("domain")
	StrCookiePath           = []byte("path")
	StrCookieHTTPOnly       = []byte("HttpOnly")
	StrCookieSecure         = []byte("secure")
	StrCookieMaxAge         = []byte("max-age")
	StrCookieSameSite       = []byte("SameSite")
	StrCookieSameSiteLax    = []byte("Lax")
	StrCookieSameSiteStrict = []byte("Strict")
	StrCookieSameSiteNone   = []byte("None")

	StrClose               = []byte("close")
	StrGzip                = []byte("gzip")
	StrDeflate             = []byte("deflate")
	StrKeepAlive           = []byte("keep-alive") // 用于指明连接为保活的长连接
	StrUpgrade             = []byte("Upgrade")
	StrChunked             = []byte("chunked")
	StrIdentity            = []byte("identity") // 用于指代编码形式为：自身（如未经压缩和修改）
	Str100Continue         = []byte("100-continue")
	StrPostArgsContentType = []byte("application/x-www-form-urlencoded")
	StrMultipartFormData   = []byte("multipart/form-data")
	StrBoundary            = []byte("boundary")
	StrBytes               = []byte("bytes")
	StrTextSlash           = []byte("text/")
	StrApplicationSlash    = []byte("application/")
	StrBasicSpace          = []byte("Basic ")

	StrClientPreface = []byte(consts.ClientPreface) // http2 必须由客户端新连接发送的字符串
)
