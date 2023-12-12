package protocol

import (
	"bytes"
	"path/filepath"
	"sync"

	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/internal/bytestr"
	"github.com/favbox/wind/internal/nocopy"
)

var uriPool = &sync.Pool{
	New: func() any {
		return &URI{}
	},
}

// AcquireURI 从池中取空白 URI。用完 ReleaseURI 回池，以减少内存分配。
func AcquireURI() *URI {
	return uriPool.Get().(*URI)
}

// ReleaseURI 将AcquireURI 取出 URI 放回池中。放后勿碰，以防竞争。
func ReleaseURI(u *URI) {
	u.Reset()
	uriPool.Put(u)
}

// ParseURI 解析 uriStr 为 *URI。
func ParseURI(uriStr string) *URI {
	uri := &URI{}
	uri.Parse(nil, []byte(uriStr))

	return uri
}

// Proxy 定义返回给定请求代理网址的函数。
type Proxy func(*Request) (*URI, error)

// ProxyURI 返回给定网址的代理函数。
func ProxyURI(fixedURI *URI) Proxy {
	return func(*Request) (*URI, error) {
		return fixedURI, nil
	}
}

// URI 是完全限定的网址结构体。提供了协议、主机、路径、查询字符串、哈希等读写方法。
type URI struct {
	noCopy nocopy.NoCopy

	scheme       []byte
	host         []byte
	pathOriginal []byte
	path         []byte
	queryString  []byte
	hash         []byte

	queryArgs       Args
	parsedQueryArgs bool

	DisablePathNormalizing bool

	fullURI    []byte
	requestURI []byte

	username []byte
	password []byte
}

// AppendBytes 附加到 dst 并返回。
func (u *URI) AppendBytes(dst []byte) []byte {
	dst = u.appendSchemeHost(dst)
	dst = append(dst, u.RequestURI()...)
	if len(u.hash) > 0 {
		dst = append(dst, '#')
		dst = append(dst, u.hash...)
	}
	return dst
}

// FullURI 返回形如 {Scheme}://{Host}{RequestURI}#{Hash} 的完整 uri。
func (u *URI) FullURI() []byte {
	u.fullURI = u.AppendBytes(u.fullURI[:0])
	return u.fullURI
}

// Parse 解析 host/uri 为 URI。两者必有一个携带 scheme 和 host。
func (u *URI) Parse(host, uri []byte) {
	u.parse(host, uri, false)
}

// 原始URL可能形如 scheme:path。
// （scheme 必须是 [a-zA-Z][a-zA-Z0-9+-.]*）
// 如此，返回 scheme, path；否则返回 nil, rawURL。
func getScheme(rawURL []byte) (scheme, path []byte) {
	for i := 0; i < len(rawURL); i++ {
		c := rawURL[i]
		switch {
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z':
			// 什么都不做
		case '0' <= c && c <= '9' || c == '+' || c == '-' || c == '.':
			if i == 0 {
				return nil, rawURL
			}
		case c == ':':
			return checkSchemeWhenCharIsColon(i, rawURL)
		default:
			// 我们遇到了无效字符，因此没有又要的方案
			return nil, rawURL
		}
	}
	return nil, rawURL
}

func (u *URI) parse(host, uri []byte, isTLS bool) {
	u.Reset()

	if stringContainsCTLByte(uri) {
		return
	}

	if len(host) == 0 || bytes.Contains(uri, bytestr.StrColonSlashSlash) {
		scheme, newHost, newURI := splitHostURI(host, uri)
		u.scheme = append(u.scheme, scheme...)
		bytesconv.LowercaseBytes(u.scheme)
		host = newHost
		uri = newURI
	}

	if isTLS {
		u.scheme = append(u.scheme[:0], bytestr.StrHTTPS...)
	}

	// 解析请求的主机标头中的用户名和密码
	if n := bytes.Index(host, bytestr.StrAt); n >= 0 {
		auth := host[:n]
		host = host[n+1:]

		if n := bytes.Index(auth, bytestr.StrColon); n >= 0 {
			u.username = append(u.username[:0], auth[:n]...)
			u.password = append(u.password[:0], auth[n+1:]...)
		} else {
			u.username = append(u.username[:0], auth...)
			u.password = u.password[:0]
		}
	}

	u.host = append(u.host, host...)
	bytesconv.LowercaseBytes(u.host)

	b := uri
	queryIndex := bytes.IndexByte(b, '?')
	fragmentIndex := bytes.IndexByte(b, '#')
	// 忽略片段部分的查询
	if fragmentIndex >= 0 && queryIndex > fragmentIndex {
		queryIndex = -1
	}

	if queryIndex < 0 && fragmentIndex < 0 {
		u.pathOriginal = append(u.pathOriginal, b...)
		u.path = normalizePath(u.path, u.pathOriginal)
		return
	}

	if queryIndex >= 0 {
		// Path 是 Query 之前的所有部分
		u.pathOriginal = append(u.pathOriginal, b[:queryIndex]...)
		u.path = normalizePath(u.path, u.pathOriginal)

		if fragmentIndex < 0 {
			u.queryString = append(u.queryString, b[queryIndex+1:]...)
		} else {
			u.queryString = append(u.queryString, b[queryIndex+1:fragmentIndex]...)
			u.hash = append(u.hash, b[fragmentIndex+1:]...)
		}
		return
	}

	// 有 # 片段，且无查询
	// Path 是片段前的部分
	u.pathOriginal = append(u.pathOriginal, b[:fragmentIndex]...)
	u.path = normalizePath(u.path, u.pathOriginal)
	u.hash = append(u.hash, b[fragmentIndex+1:]...)
}

// Update 更新 URI。
//
// 支持如下 newURI 类型：
//
//   - 绝对，如：http://foobar.com/aaa/bb?cc 该情况下原始uri会被 newURI 替换。
//   - 绝对但无协议，如 //foobar.com/aaa/bb?cc 该情况下原始协议会被保留。
//   - 无主机，如 /aaa/bb?cc 该情况下仅原始uri的 RequestURI 部分会被替换。
//   - 相对路径，如 xx?yy=abc 该情况下原始 RequestURI 根据新的相对路径进行更新。
func (u *URI) Update(newURI string) {
	u.UpdateBytes(bytesconv.S2b(newURI))
}

// UpdateBytes 更新 URI。
//
// 支持如下 newURI 类型：
//
//   - 绝对，如：http://foobar.com/aaa/bb?cc 该情况下原始uri会被 newURI 替换。
//   - 绝对但无协议，如 //foobar.com/aaa/bb?cc 该情况下原始协议会被保留。
//   - 无主机，如 /aaa/bb?cc 该情况下仅原始uri的 RequestURI 部分会被替换。
//   - 相对路径，如 xx?yy=abc 该情况下原始 RequestURI 根据新的相对路径进行更新。
func (u *URI) UpdateBytes(newURI []byte) {
	u.requestURI = u.updateBytes(newURI, u.requestURI)
}

// CopyTo 拷贝 URI 信息到 dst。
func (u *URI) CopyTo(dst *URI) {
	dst.Reset()
	dst.pathOriginal = append(dst.pathOriginal[:0], u.pathOriginal...)
	dst.scheme = append(dst.scheme[:0], u.scheme...)
	dst.path = append(dst.path[:0], u.path...)
	dst.queryString = append(dst.queryString[:0], u.queryString...)
	dst.hash = append(dst.hash[:0], u.hash...)
	dst.host = append(dst.host[:0], u.host...)
	dst.username = append(dst.username[:0], u.username...)
	dst.password = append(dst.password[:0], u.password...)

	u.queryArgs.CopyTo(&dst.queryArgs)
	dst.parsedQueryArgs = u.parsedQueryArgs
	dst.DisablePathNormalizing = u.DisablePathNormalizing

	// fullURI 和 requestURI 不应拷贝，因为它们是每次调用 FullURI() 和 RequestURI() 时动态创建的。
}

// 返回完整 uri。
func (u *URI) String() string {
	return string(u.FullURI())
}

// Path 返回 URI 路径，例如 /foo/bar 是 http://aaa.com/foo/bar?baz=123#qwe 的路径。
//
// 返回路径都是经过url编码和规范化的，
// 例如 '//f%20obar/baz/../zzz' 变成 '/f obar/zzz'。
//
// 返回的值在下一次URI方法调用之前一直有效。
func (u *URI) Path() []byte {
	path := u.path
	if len(path) == 0 {
		path = bytestr.StrSlash
	}
	return path
}

// SetPath 设置 URI 路径。
func (u *URI) SetPath(path string) {
	u.pathOriginal = append(u.pathOriginal[:0], path...)
	u.path = normalizePath(u.path, u.pathOriginal)
}

// SetPathBytes 设置 URI 路径。
func (u *URI) SetPathBytes(path []byte) {
	u.pathOriginal = append(u.pathOriginal[:0], path...)
	u.path = normalizePath(u.path, u.pathOriginal)
}

// LastPathSegment 返回 URI 路径的最后一个片段。
//
// 范例：
//
//   - 如 /foo/bar/baz.html 返回 baz.html
//   - 如 /foo/bar/ 返回空字节切片。
//   - 如 /foobar.js 返回 foobar.js
func (u *URI) LastPathSegment() []byte {
	path := u.Path()
	n := bytes.LastIndexByte(path, '/')
	if n < 0 {
		return path
	}
	return path[n+1:]
}

// QueryArgs 返回查询参数切片。
func (u *URI) QueryArgs() *Args {
	u.parseQueryArgs()
	return &u.queryArgs
}

func (u *URI) parseQueryArgs() {
	if u.parsedQueryArgs {
		return
	}
	u.queryArgs.ParseBytes(u.queryString)
	u.parsedQueryArgs = true
}

// QueryString 返回 URI 查询字符串。
// 例如，baz=123 是 http://abc.com/foo/bar?baz=123#qwe 的查询字符串。
func (u *URI) QueryString() []byte {
	return u.queryString
}

// SetQueryString 设置 URI 查询字符串。
func (u *URI) SetQueryString(queryString string) {
	u.queryString = append(u.queryString[:0], queryString...)
	u.parsedQueryArgs = false
}

// SetQueryStringBytes 设置 URI 查询字符串。
func (u *URI) SetQueryStringBytes(queryString []byte) {
	u.queryString = append(u.queryString[:0], queryString...)
	u.parsedQueryArgs = false
}

// PathOriginal 返回 URI 中传给 URI.Parse() 的原始路径。
func (u *URI) PathOriginal() []byte {
	return u.pathOriginal
}

// Reset 清空 URI。
func (u *URI) Reset() {
	u.scheme = u.scheme[:0]
	u.host = u.host[:0]
	u.path = u.path[:0]
	u.pathOriginal = u.pathOriginal[:0]
	u.queryString = u.queryString[:0]
	u.hash = u.hash[:0]
	u.username = u.username[:0]
	u.password = u.password[:0]

	u.queryArgs.Reset()
	u.parsedQueryArgs = false
	u.DisablePathNormalizing = false

	// 没有必要设置 u.fullURI = u.fullURI[:0]，因为其是每次调用 FullURI() 自动计算的。
	// u.requestURI 同理。
}

// Scheme 返回 URI 协议，例如 http 是 http://abc.com 对应的协议。
//
// 返回的方案总是小写格式。
//
// 返回的值在下一次URI方法调用之前一直有效。
func (u *URI) Scheme() []byte {
	scheme := u.scheme
	if len(scheme) == 0 {
		scheme = bytestr.StrHTTP
	}
	return scheme
}

// SetScheme 设置 URI 方案，例如 http, https, ftp 等。
func (u *URI) SetScheme(scheme string) {
	u.scheme = append(u.scheme[:0], scheme...)
	bytesconv.LowercaseBytes(u.scheme)
}

// SetSchemeBytes 设置 URI 方案，例如 http, https, ftp 等。
func (u *URI) SetSchemeBytes(scheme []byte) {
	u.scheme = append(u.scheme[:0], scheme...)
	bytesconv.LowercaseBytes(u.scheme)
}

// Hash 返回 URI 哈希，例如 qew 是 http://abc.com/foo/bar?baz=123#qwe 的哈希。
func (u *URI) Hash() []byte {
	return u.hash
}

// SetHash 设置 URI 的片段哈希。
func (u *URI) SetHash(hash string) {
	u.hash = append(u.hash[:0], hash...)
}

// SetHashBytes 设置 URI 的片段哈希。
func (u *URI) SetHashBytes(hash []byte) {
	u.hash = append(u.hash[:0], hash...)
}

func (u *URI) appendSchemeHost(dst []byte) []byte {
	dst = append(dst, u.Scheme()...)
	dst = append(dst, bytestr.StrColonSlashSlash...)
	return append(dst, u.Host()...)
}

// Host 返回 URI 的主机。
func (u *URI) Host() []byte {
	return u.host
}

// SetHost 设置 URI 的主机。
func (u *URI) SetHost(host string) {
	u.host = append(u.host[:0], host...)
	bytesconv.LowercaseBytes(u.host)
}

// SetHostBytes 设置 URI 的主机。
func (u *URI) SetHostBytes(host []byte) {
	u.host = append(u.host[:0], host...)
	bytesconv.LowercaseBytes(u.host)
}

// Username 返回 URI 用户名。
func (u *URI) Username() []byte {
	return u.username
}

// SetUsername 设置 URI 用户名。
func (u *URI) SetUsername(username string) {
	u.username = append(u.username[:0], username...)
}

// SetUsernameBytes 设置 URI 用户名。
func (u *URI) SetUsernameBytes(username []byte) {
	u.username = append(u.username[:0], username...)
}

// Password 返回 URI 密码。
func (u *URI) Password() []byte {
	return u.password
}

// SetPassword 设置 URI 密码。
func (u *URI) SetPassword(password string) {
	u.password = append(u.password[:0], password...)
}

// SetPasswordBytes 设置 URI 密码。
func (u *URI) SetPasswordBytes(password []byte) {
	u.password = append(u.password[:0], password...)
}

// RequestURI 返回编码后的请求地址 - 即除了 Scheme 和 Host 的其他部分。
func (u *URI) RequestURI() []byte {
	var dst []byte
	if u.DisablePathNormalizing {
		dst = append(u.requestURI[:0], u.PathOriginal()...)
	} else {
		dst = bytesconv.AppendQuotedPath(u.requestURI[:0], u.Path())
	}
	if u.queryArgs.Len() > 0 {
		dst = append(dst, '?')
		dst = u.queryArgs.AppendBytes(dst)
	} else if len(u.queryString) > 0 {
		dst = append(dst, '?')
		dst = append(dst, u.queryString...)
	}
	u.requestURI = dst
	return u.requestURI
}

func (u *URI) updateBytes(newURI, buf []byte) []byte {
	if len(newURI) == 0 {
		return buf
	}

	n := bytes.Index(newURI, bytestr.StrSlashSlash)
	if n >= 0 {
		// 绝对 uri
		var b [32]byte
		schemeOriginal := b[:0]
		if len(u.scheme) > 0 {
			schemeOriginal = append([]byte(nil), u.scheme...)
		}
		if n == 0 {
			newURI = bytes.Join([][]byte{u.scheme, bytestr.StrColon, newURI}, nil)
		}
		u.Parse(nil, newURI)
		if len(schemeOriginal) > 0 && len(u.scheme) == 0 {
			u.scheme = append(u.scheme[:0], schemeOriginal...)
		}
		return buf
	}

	if newURI[0] == '/' {
		// uri 无主机
		buf = u.appendSchemeHost(buf[:0])
		buf = append(buf, newURI...)
		u.Parse(nil, buf)
		return buf
	}

	// 相对路径
	switch newURI[0] {
	case '?':
		// 只更新查询字符串
		u.SetQueryStringBytes(newURI[1:])
		return append(buf[:0], u.FullURI()...)
	case '#':
		// 只更新哈希
		u.SetHashBytes(newURI[1:])
		return append(buf[:0], u.FullURI()...)
	default:
		// 更新斜线之后的最后一个路径部分
		path := u.Path()
		n = bytes.LastIndexByte(path, '/')
		if n < 0 {
			panic("BUG: 路径必须包含至少一个 /")
		}
		buf = u.appendSchemeHost(buf[:0])
		buf = bytesconv.AppendQuotedPath(buf, path[:n+1])
		buf = append(buf, newURI...)
		u.Parse(nil, buf)
		return buf
	}
}

func splitHostURI(host, uri []byte) ([]byte, []byte, []byte) {
	scheme, path := getScheme(uri)

	if scheme == nil {
		return bytestr.StrHTTP, host, uri
	}

	uri = path[len(bytestr.StrSlashSlash):]
	n := bytes.IndexByte(uri, '/')
	if n < 0 {
		// 兼容主机后面没有/的情况，如 foobar.com?a=b
		if n = bytes.IndexByte(uri, '?'); n >= 0 {
			return scheme, uri[:n], uri[n:]
		}
		return scheme, uri, bytestr.StrSlash
	}
	return scheme, uri[:n], uri[n:]
}

// 返回字符串 s 是否包含 ASCII 控制符。
func stringContainsCTLByte(b []byte) bool {
	for i, n := 0, len(b); i < n; i++ {
		r := b[i]
		if r < ' ' || r == 0x7f {
			return true
		}
	}
	return false
}

func normalizePath(dst, src []byte) []byte {
	dst = dst[:0]
	dst = addLeadingSlash(dst, src)
	dst = decodeArgAppendNoPlus(dst, src)

	// Windows 服务器需将所有反斜线替换为正斜线，以免路径遍历攻击。
	if filepath.Separator == '\\' {
		for {
			n := bytes.IndexByte(dst, '\\')
			if n < 0 {
				break
			}
			dst[n] = '/'
		}
	}

	// 规范化所有双斜杠
	b := dst
	bSize := len(b)
	for {
		n := bytes.Index(b, bytestr.StrSlashSlash)
		if n < 0 {
			break
		}
		b = b[n:]
		// 整体左移1位，并裁掉多余的1位尾部
		copy(b, b[1:])
		b = b[:len(b)-1]
		bSize--
	}
	dst = dst[:bSize]

	// 规范化 /./ 部分
	b = dst
	for {
		n := bytes.Index(b, bytestr.StrSlashDotSlash)
		if n < 0 {
			break
		}
		nn := n + len(bytestr.StrSlashDotSlash) - 1
		copy(b[n:], b[nn:])
		b = b[:len(b)-nn+n]
	}

	// 规范化形如 /foo/../ 的部分
	for {
		n := bytes.Index(b, bytestr.StrSlashDotDotSlash)
		if n < 0 {
			break
		}
		nn := bytes.LastIndexByte(b[:n], '/')
		if nn < 0 {
			nn = 0
		}
		n += len(bytestr.StrSlashDotDotSlash) - 1
		copy(b[nn:], b[n:])
		b = b[:len(b)-n+nn]
	}

	// 规范化结尾形如 /foo/.. 的部分
	n := bytes.LastIndex(b, bytestr.StrSlashDotDot)
	if n >= 0 && n+len(bytestr.StrSlashDotDot) == len(b) {
		nn := bytes.LastIndexByte(b[:n], '/')
		if nn < 0 {
			return bytestr.StrSlash
		}
		b = b[:nn+1]
	}

	return b
}
