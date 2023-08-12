package protocol

import (
	"bytes"
	"sync"
	"time"

	"github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/common/utils"
	"github.com/favbox/wind/common/wlog"
	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/internal/bytestr"
	"github.com/favbox/wind/internal/nocopy"
)

const (
	// CookieSameSiteDisabled 移除 SameSite 标识符
	CookieSameSiteDisabled CookieSameSite = iota
	// CookieSameSiteDefaultMode 设置 SameSite 标识符
	CookieSameSiteDefaultMode
	// CookieSameSiteLaxMode 设置带有 "Lax" 参数的 SameSite 标识符。
	CookieSameSiteLaxMode
	// CookieSameSiteStrictMode 设置带有 "Strict" 参数的 SameSite 标识符。
	CookieSameSiteStrictMode
	// CookieSameSiteNoneMode 设置带有 "None" 参数的 SameSite 标识符
	// 详见 https://tools.ietf.org/html/draft-west-cookie-incrementalism-00
	CookieSameSiteNoneMode
)

var zeroTime time.Time

var (
	errNoCookies = errors.NewPublic("未找到Cookie")

	// CookieExpireUnlimited 表示不会过期的 cookie。
	CookieExpireUnlimited = zeroTime

	// CookieExpireDelete 可在 Cookie.Expire 上设置，以使其过期。
	CookieExpireDelete = time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
)

var cookiePool = &sync.Pool{
	New: func() any {
		return &Cookie{}
	},
}

// AcquireCookie 从池中取空白 Cookie。用完 ReleaseCookie 回池，以减少内存分配。
func AcquireCookie() *Cookie {
	return cookiePool.Get().(*Cookie)
}

// ReleaseCookie 将 AcquireCookie 取出的 Cookie 放回池中。放后勿碰，以防竞争。
func ReleaseCookie(c *Cookie) {
	c.Reset()
	cookiePool.Put(c)
}

// CookieSameSite 指定 Cookie 设置相同网站标记的枚举模式。
// 详见 https://tools.ietf.org/html/draft-ietf-httpbis-cookie-same-site-00
type CookieSameSite int

// Cookie 表示 HTTP cookie。协程不安全，不可拷贝。
type Cookie struct {
	noCopy nocopy.NoCopy

	bufKV argsKV
	buf   []byte

	key   []byte
	value []byte

	expire time.Time // 到期时间
	maxAge int       // 存活秒数，优先级高于到期时间

	domain []byte
	path   []byte

	httpOnly bool
	secure   bool
	sameSite CookieSameSite
}

// AppendBytes 附加到 dst 并返回。
func (c *Cookie) AppendBytes(dst []byte) []byte {
	if len(c.key) > 0 {
		dst = append(dst, c.key...)
		dst = append(dst, '=')
	}
	dst = append(dst, c.value...)

	if c.maxAge > 0 {
		dst = append(dst, ';', ' ')
		dst = append(dst, bytestr.StrCookieMaxAge...)
		dst = append(dst, '=')
		dst = bytesconv.AppendUint(dst, c.maxAge)
	} else if !c.expire.IsZero() {
		c.bufKV.value = bytesconv.AppendHTTPDate(c.bufKV.value[:0], c.expire)
		dst = appendCookiePart(dst, bytestr.StrCookieExpires, c.bufKV.value)
	}
	if len(c.domain) > 0 {
		dst = appendCookiePart(dst, bytestr.StrCookieDomain, c.domain)
	}
	if len(c.path) > 0 {
		dst = appendCookiePart(dst, bytestr.StrCookiePath, c.path)
	}
	if c.httpOnly {
		dst = append(dst, ';', ' ')
		dst = append(dst, bytestr.StrCookieHTTPOnly...)
	}
	if c.secure {
		dst = append(dst, ';', ' ')
		dst = append(dst, bytestr.StrCookieSecure...)
	}
	switch c.sameSite {
	case CookieSameSiteDefaultMode:
		dst = append(dst, ';', ' ')
		dst = append(dst, bytestr.StrCookieSameSite...)
	case CookieSameSiteLaxMode:
		dst = appendCookiePart(dst, bytestr.StrCookieSameSite, bytestr.StrCookieSameSiteLax)
	case CookieSameSiteStrictMode:
		dst = appendCookiePart(dst, bytestr.StrCookieSameSite, bytestr.StrCookieSameSiteStrict)
	case CookieSameSiteNoneMode:
		dst = appendCookiePart(dst, bytestr.StrCookieSameSite, bytestr.StrCookieSameSiteNone)
	}
	return dst
}

// Cookie 返回其字节切片形式。
func (c *Cookie) Cookie() []byte {
	c.buf = c.AppendBytes(c.buf[:0])
	return c.buf
}

// Key 返回 Cookie 的键。
func (c *Cookie) Key() []byte {
	return c.key
}

// SetKey 设置 Cookie 的键。
func (c *Cookie) SetKey(key string) {
	c.key = append(c.key[:0], key...)
}

// SetKeyBytes 设置 Cookie 的键。
func (c *Cookie) SetKeyBytes(key []byte) {
	c.key = append(c.key[:0], key...)
}

// Value 返回 Cookie 的值。
func (c *Cookie) Value() []byte {
	return c.value
}

// SetValue 设置 Cookie 的值。
func (c *Cookie) SetValue(value string) {
	warnIfInvalid(bytesconv.S2b(value))
	c.value = append(c.value[:0], value...)
}

// SetValueBytes 设置 Cookie 的值。
func (c *Cookie) SetValueBytes(value []byte) {
	warnIfInvalid(value)
	c.value = append(c.value[:0], value...)
}

// Domain 返回 Cookie 的域名。
func (c *Cookie) Domain() []byte {
	return c.domain
}

// SetDomain 设置 Cookie 域名。
func (c *Cookie) SetDomain(domain string) {
	c.domain = append(c.domain[:0], domain...)
}

// Path 返回 Cookie 的路径。
func (c *Cookie) Path() []byte {
	return c.path
}

// SetPath 设置 Cookie 的路径。
func (c *Cookie) SetPath(path string) {
	c.buf = append(c.buf[:0], path...)
	c.path = normalizePath(c.path, c.buf)
}

// SetPathBytes 设置 Cookie 路径。
func (c *Cookie) SetPathBytes(path []byte) {
	c.buf = append(c.buf[:0], path...)
	c.path = normalizePath(c.path, c.buf)
}

// Expire 返回 Cookie 的过期时间。
func (c *Cookie) Expire() time.Time {
	expire := c.expire
	if expire.IsZero() {
		expire = CookieExpireUnlimited
	}
	return expire
}

// SetExpire 设置 Cookie 的过期时间。
//
// 设为 CookieExpireDelete 可删除客户端上的 Cookie。
//
// 默认情况下，Cookie 的生存期受浏览器会话限制。
func (c *Cookie) SetExpire(expire time.Time) {
	c.expire = expire
}

// MaxAge 返回 Cookie 的到期秒数，如无 maxAge 则返回 0。
func (c *Cookie) MaxAge() int {
	return c.maxAge
}

// SetMaxAge 设置 Cookie 的到期秒数，该设置优先于过期时间设置。
//
// 设置为 0 意为取消设置。
func (c *Cookie) SetMaxAge(seconds int) {
	c.maxAge = seconds
}

// HTTPOnly 返回 Cookie 是否仅用于 http。
func (c *Cookie) HTTPOnly() bool {
	return c.httpOnly
}

// SetHTTPOnly 设置 Cookie 是否仅用于 http。
func (c *Cookie) SetHTTPOnly(httpOnly bool) {
	c.httpOnly = httpOnly
}

// Secure 返回 Cookie 是否是安全的。
func (c *Cookie) Secure() bool {
	return c.secure
}

// SetSecure 设置 Cookie 的安全标识。
func (c *Cookie) SetSecure(secure bool) {
	c.secure = secure
}

// SameSite 返回 Cookie 的 SameSite 模式。
func (c *Cookie) SameSite() CookieSameSite {
	return c.sameSite
}

// SetSameSite 设置 Cookie 的 SameSite 标识。
//
// 设为 CookieSameSiteNoneMode 也会讲 Secure 设为 true 以免别浏览器拒绝。
func (c *Cookie) SetSameSite(mode CookieSameSite) {
	c.sameSite = mode
	if mode == CookieSameSiteNoneMode {
		c.SetSecure(true)
	}
}

// 返回 Cookie 的字符串表达形式。
//
// 注：没有 maxAge 到期秒数，则取 expire 到期时间。
func (c *Cookie) String() string {
	return string(c.Cookie())
}

// Parse 解析指定 src 到 Cookie。
func (c *Cookie) Parse(src string) error {
	c.buf = append(c.buf[:0], src...)
	return c.ParseBytes(c.buf)
}

// ParseBytes 解析 src 至当前 Cookie c。
func (c *Cookie) ParseBytes(src []byte) error {
	c.Reset()

	var s cookieScanner
	s.b = src

	kv := &c.bufKV
	if !s.next(kv) {
		return errNoCookies
	}

	c.key = append(c.key[:0], kv.key...)
	c.value = append(c.value[:0], kv.value...)

	for s.next(kv) {
		if len(kv.key) != 0 {
			//	在名称的第一个字符上不区分大小写对比
			switch kv.key[0] | 0x20 {
			case 'm': // maxAge
				if utils.CaseInsensitiveCompare(bytestr.StrCookieMaxAge, kv.key) {
					maxAge, err := bytesconv.ParseUint(kv.value)
					if err != nil {
						return err
					}
					c.maxAge = maxAge
				}
			case 'e': // expire
				if utils.CaseInsensitiveCompare(bytestr.StrCookieExpires, kv.key) {
					v := bytesconv.B2s(kv.value)
					// 尝试与 net/http 相同的两种格式
					// 详见 https://github.com/golang/go/blob/00379be17e63a5b75b3237819392d2dc3b313a27/src/net/http/cookie.go#L133-L135
					expire, err := time.ParseInLocation(time.RFC1123, v, time.UTC)
					if err != nil {
						expire, err = time.Parse("Jan, 02-Jan-2006 15:04:05 MST", v)
						if err != nil {
							return err
						}
					}
					c.expire = expire
				}
			case 'd': // domain
				if utils.CaseInsensitiveCompare(bytestr.StrCookieDomain, kv.key) {
					c.domain = append(c.domain[:0], kv.value...)
				}
			case 'p': // path
				if utils.CaseInsensitiveCompare(bytestr.StrCookiePath, kv.key) {
					c.path = append(c.path[:0], kv.value...)
				}
			case 's': // sameSite
				if utils.CaseInsensitiveCompare(bytestr.StrCookieSameSite, kv.key) {
					// 在值的第一个字符上不分大小写对比
					switch kv.value[0] | 0x20 {
					case 'l': // lax
						if utils.CaseInsensitiveCompare(bytestr.StrCookieSameSiteLax, kv.value) {
							c.sameSite = CookieSameSiteLaxMode
						}
					case 's': // strict
						if utils.CaseInsensitiveCompare(bytestr.StrCookieSameSiteStrict, kv.value) {
							c.sameSite = CookieSameSiteStrictMode
						}
					case 'n': // none
						if utils.CaseInsensitiveCompare(bytestr.StrCookieSameSiteNone, kv.value) {
							c.sameSite = CookieSameSiteNoneMode
						}
					}
				}
			}
		} else if len(kv.value) != 0 {
			switch kv.value[0] | 0x20 {
			case 'h': // httponly
				if utils.CaseInsensitiveCompare(bytestr.StrCookieHTTPOnly, kv.value) {
					c.httpOnly = true
				}
			case 's': // secure
				if utils.CaseInsensitiveCompare(bytestr.StrCookieSecure, kv.value) {
					c.secure = true
				} else if utils.CaseInsensitiveCompare(bytestr.StrCookieSameSite, kv.value) {
					c.sameSite = CookieSameSiteDefaultMode
				}
			}
		} // 其他为空或不匹配
	}
	return nil
}

// Reset 清除 Cookie。
func (c *Cookie) Reset() {
	c.key = c.key[:0]
	c.value = c.value[:0]
	c.expire = zeroTime
	c.maxAge = 0
	c.domain = c.domain[:0]
	c.path = c.path[:0]
	c.httpOnly = false
	c.secure = false
	c.sameSite = CookieSameSiteDisabled
}

type cookieScanner struct {
	b []byte
}

func (s *cookieScanner) next(kv *argsKV) bool {
	b := s.b
	if len(b) == 0 {
		return false
	}

	isKey := true
	k := 0
	for i, c := range b {
		switch c {
		case '=':
			if isKey {
				isKey = false
				kv.key = decodeCookieArg(kv.key, b[:i], false)
				k = i + 1
			}
		case ';':
			if isKey {
				kv.key = kv.key[:0]
			}
			kv.value = decodeCookieArg(kv.value, b[k:i], true)
			s.b = b[i+1:]
			return true
		}
	}

	if isKey {
		kv.key = kv.key[:0]
	}
	kv.value = decodeCookieArg(kv.value, b[k:], true)
	s.b = b[len(b):]
	return true
}

func appendCookiePart(dst, key, value []byte) []byte {
	dst = append(dst, ';', ' ')
	dst = append(dst, key...)
	dst = append(dst, '=')
	return append(dst, value...)
}

func appendRequestCookieBytes(dst []byte, cookies []argsKV) []byte {
	for i, n := 0, len(cookies); i < n; i++ {
		kv := &cookies[i]
		if len(kv.key) > 0 {
			dst = append(dst, kv.key...)
			dst = append(dst, '=')
		}
		dst = append(dst, kv.value...)
		if i+1 < n {
			dst = append(dst, ';', ' ')
		}
	}
	return dst
}

// 对于 Response 我们不能使用上述函数，因为响应 Cookie 中已包含 "key="。
func appendResponseCookieBytes(dst []byte, cookies []argsKV) []byte {
	for i, n := 0, len(cookies); i < n; i++ {
		kv := &cookies[i]
		dst = append(dst, kv.value...)
		if i+1 < n {
			dst = append(dst, ';', ' ')
		}
	}
	return dst
}

func decodeCookieArg(dst, src []byte, skipQuotes bool) []byte {
	// 去掉开头空格
	for len(src) > 0 && src[0] == ' ' {
		src = src[1:]
	}
	// 去掉结尾空格
	for len(src) > 0 && src[len(src)-1] == ' ' {
		src = src[:len(src)-1]
	}
	// 跳过双引号
	if skipQuotes {
		if len(src) > 1 && src[0] == '"' && src[len(src)-1] == '"' {
			src = src[1 : len(src)-1]
		}
	}
	return append(dst[:0], src...)
}

func getCookieKey(dst, src []byte) []byte {
	n := bytes.IndexByte(src, '=')
	if n >= 0 {
		src = src[:n]
	}
	return decodeCookieArg(dst, src, false)
}

// 若 Cookie 值无效则发出警告
func warnIfInvalid(value []byte) bool {
	for i := range value {
		if bytesconv.ValidCookieValueTable[value[i]] == 0 {
			wlog.SystemLogger().Warnf("Cookie.Value 包含无效字节 %q，"+
				"可能导致用户代理的兼容问题", value[i])
			return false
		}
	}
	return true
}
