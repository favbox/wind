package bytesconv

import (
	"net/http"
	"reflect"
	"sync"
	"time"
	"unsafe"

	"github.com/favbox/wind/network"
)

const (
	upperHex = "0123456789ABCDEF" // 大写的十六进制字符
	lowerHex = "0123456789abcdef" // 小写的十六进制字符
)

var hexIntBufPool sync.Pool

func LowercaseBytes(b []byte) {
	for i, n := 0, len(b); i < n; i++ {
		p := &b[i]
		*p = ToLowerTable[*p]
	}
}

// B2s 将字节切片转为字符串，且不分配内存。
// 详见 https://groups.google.com/forum/#!msg/Golang-Nuts/ENgbUzYvCuU/90yGx7GUAgAJ 。
//
// 注意：如果字符串或切片的标头在未来的go版本中更改，该方法可能会出错。
func B2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// S2b 将字符串转为字节切片，且不分配内存。
//
// 注意：如果字符串或切片的标头在未来的go版本中更改，该方法可能会出错。
func S2b(s string) (b []byte) {
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh.Data = sh.Data
	bh.Len = sh.Len
	bh.Cap = sh.Len
	return b
}

// AppendQuotedArg 向 dst 追加转义后的 src 参数。等效 url.QueryEscape。
func AppendQuotedArg(dst, src []byte) []byte {
	for _, c := range src {
		switch {
		case c == ' ':
			dst = append(dst, '+')
		case QuotedArgShouldEscapeTable[int(c)] != 0:
			dst = append(dst, '%', upperHex[c>>4], upperHex[c&0xf])
		default:
			dst = append(dst, c)
		}
	}
	return dst
}

// AppendQuotedPath 向 dst 追加转义后的 src 路径。等效于 url.PathEscape。
func AppendQuotedPath(dst, src []byte) []byte {
	// 修复该问题 https://github.com/golang/go/issues/11202
	if len(src) == 1 && src[0] == '*' {
		return append(dst, '*')
	}

	for _, c := range src {
		if QuotedPathShouldEscapeTable[int(c)] != 0 {
			dst = append(dst, '%', upperHex[c>>4], upperHex[c&15])
		} else {
			dst = append(dst, c)
		}
	}
	return dst
}

// AppendUint 向 dst 追加正整数 n 并返回。
func AppendUint(dst []byte, n int) []byte {
	if n < 0 {
		panic("BUG：int 必须为正整数")
	}

	var b [20]byte
	buf := b[:]
	i := len(buf)
	var q int
	for n >= 10 {
		i--
		q = n / 10
		buf[i] = '0' + byte(n-q*10)
		n = q
	}
	i--
	buf[i] = '0' + byte(n)

	dst = append(dst, buf[i:]...)
	return dst
}

// AppendHTTPDate 向 dst 追加 HTTP 兼容时间并返回。
func AppendHTTPDate(dst []byte, date time.Time) []byte {
	return date.UTC().AppendFormat(dst, http.TimeFormat)
}

// ParseUintBuf 解析 b 中的整数。
func ParseUintBuf(b []byte) (v, n int, err error) {
	n = len(b)
	if n == 0 {
		return -1, 0, errEmptyInt
	}
	for i := 0; i < n; i++ {
		c := b[i]
		k := c - '0'
		if k > 9 {
			if i == 0 {
				return -1, i, errUnexpectedFirstChar
			}
			return v, i, nil
		}
		vNew := 10*v + int(k)
		// 测试溢出
		if vNew < v {
			return -1, i, errTooLongInt
		}
		v = vNew
	}
	return
}

// ParseUint 解析 b 中的整数。
func ParseUint(b []byte) (int, error) {
	v, n, err := ParseUintBuf(b)
	if n != len(b) {
		return -1, errUnexpectedTrailingChar
	}
	return v, err
}

// ParseHTTPDate 解析 b 中的 HTTP (RFC1123) 兼容时间。
func ParseHTTPDate(buf []byte) (time.Time, error) {
	return time.Parse(time.RFC1123, B2s(buf))
}

// WriteHexInt 向 w 写入十六进制整数值 n。
func WriteHexInt(w network.Writer, n int) error {
	if n < 0 {
		panic("BUG: int 必须为正整数")
	}

	v := hexIntBufPool.Get()
	if v == nil {
		v = make([]byte, maxHexIntChars+1)
	}
	buf := v.([]byte)

	i := len(buf) - 1
	for {
		buf[i] = lowerHex[n&0xf]
		n >>= 4
		if n == 0 {
			break
		}
		i--
	}
	safeBuf, err := w.Malloc(maxHexIntChars + 1 - i)
	copy(safeBuf, buf[i:])
	hexIntBufPool.Put(v)
	return err
}

// ReadHexInt 读取 r 中的十六进制整数值。
func ReadHexInt(r network.Reader) (int, error) {
	n := 0
	i := 0
	var k int
	for {
		buf, err := r.Peek(1)
		if err != nil {
			r.Skip(1)

			if i > 0 {
				return n, nil
			}
			return -1, err
		}

		c := buf[0]
		k = int(Hex2intTable[c])
		if k == 16 {
			if i == 0 {
				r.Skip(1)
				return -1, errEmptyHexNum
			}
			return n, nil
		}
		if i >= maxHexIntChars {
			r.Skip(1)
			return -1, errTooLargeHexNum
		}

		r.Skip(1)
		n = (n << 4) | k
		i++
	}
}
