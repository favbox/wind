package protocol

import (
	"bytes"
	"io"

	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/internal/nocopy"
)

const (
	argsNoValue  = true
	ArgsHasValue = false
)

var nilByteSlice = []byte{}

type argsKV struct {
	key     []byte
	value   []byte
	noValue bool
}

func (kv *argsKV) GetKey() []byte {
	return kv.key
}

func (kv *argsKV) GetValue() []byte {
	return kv.value
}

// Args 维护键值对参数。
type Args struct {
	noCopy nocopy.NoCopy

	args []argsKV
	buf  []byte
}

// Set 设置 'key=value' 参数。
func (a *Args) Set(key, value string) {
	a.args = setArg(a.args, key, value, ArgsHasValue)
}

// Reset 清除查询参数。
func (a *Args) Reset() {
	a.args = a.args[:0]
}

// CopyTo 拷贝所有参数至目标。
func (a *Args) CopyTo(dst *Args) {
	dst.Reset()
	dst.args = copyArgs(dst.args, a.args)
}

// Del 从查询参数中删除指定键的参数。
func (a *Args) Del(key string) {
	a.args = delAllArgs(a.args, key)
}

// DelBytes 从查询参数中删除指定键的参数。
func (a *Args) DelBytes(key []byte) {
	a.args = delAllArgs(a.args, bytesconv.B2s(key))
}

// Has 返回指定的键是否存在于 Args 中。
func (a *Args) Has(key string) bool {
	return hasArg(a.args, key)
}

// String 返回查询参数的字符串表示形式。
func (a *Args) String() string {
	return string(a.QueryString())
}

// QueryString 返回参数的查询字符串。
func (a *Args) QueryString() []byte {
	a.buf = a.AppendBytes(a.buf[:0])
	return a.buf
}

// AppendBytes 附加到 dst 并返回。
func (a *Args) AppendBytes(dst []byte) []byte {
	for i, n := 0, len(a.args); i < n; i++ {
		kv := &a.args[i]
		dst = bytesconv.AppendQuotedArg(dst, kv.key)
		if !kv.noValue {
			dst = append(dst, '=')
			if len(kv.value) > 0 {
				dst = bytesconv.AppendQuotedArg(dst, kv.value)
			}
		}
		if i+1 < n {
			dst = append(dst, '&')
		}
	}
	return dst
}

// ParseBytes 解析包含查询参数的字节切片。
func (a *Args) ParseBytes(b []byte) {
	a.Reset()

	var s argsScanner
	s.b = b

	var kv *argsKV
	a.args, kv = allocArg(a.args)
	for s.next(kv) {
		if len(kv.key) > 0 || len(kv.value) > 0 {
			a.args, kv = allocArg(a.args)
		}
	}
	a.args = releaseArg(a.args)

	if len(a.args) == 0 {
		return
	}
}

// Peek 返回指定键的查询参数值。
func (a *Args) Peek(key string) []byte {
	return peekArgStr(a.args, key)
}

// PeekExists 返回指定键的查询参数值及是否存在。
func (a *Args) PeekExists(key string) (string, bool) {
	return peekArgStrExists(a.args, key)
}

// VisitAll 对每个参数执行 f，类似于map。
// f 在返回后不能保留对 key 和 value 的引用。
// 如果需要你得制作 key/value 的副本。
func (a *Args) VisitAll(f func(key, value []byte)) {
	visitArgs(a.args, f)
}

// Len 返回查询参数的数量。
func (a *Args) Len() int {
	return len(a.args)
}

// WriteTo 将查询字符串写入 w。
//
// WriteTo 实现 io.WriteTo 接口。
func (a *Args) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(a.QueryString())
	return int64(n), err
}

// Add 添加键值对参数。
//
// 可以为同一个键添加多个值。
func (a *Args) Add(key, value string) {
	a.args = appendArg(a.args, key, value, ArgsHasValue)
}

type argsScanner struct {
	b []byte
}

func (s *argsScanner) next(kv *argsKV) bool {
	if len(s.b) == 0 {
		return false
	}
	kv.noValue = ArgsHasValue

	isKey := true
	k := 0
	for i, c := range s.b {
		switch c {
		case '=':
			if isKey {
				isKey = false
				kv.key = decodeArgAppend(kv.key[:0], s.b[:i])
				k = i + 1
			}
		case '&':
			if isKey {
				kv.key = decodeArgAppend(kv.key[:0], s.b[:i])
				kv.value = kv.value[:0]
				kv.noValue = argsNoValue
			} else {
				kv.value = decodeArgAppend(kv.value[:0], s.b[k:i])
			}
			s.b = s.b[i+1:]
			return true
		}
	}

	if isKey {
		kv.key = decodeArgAppend(kv.key[:0], s.b)
		kv.value = kv.value[:0]
		kv.noValue = argsNoValue
	} else {
		kv.value = decodeArgAppend(kv.value[:0], s.b[k:])
	}
	s.b = s.b[len(s.b):]
	return true
}

// 对切片中的每个键值对都应用 f 函数。
func visitArgs(args []argsKV, f func(key, value []byte)) {
	for i, n := 0, len(args); i < n; i++ {
		kv := &args[i]
		f(kv.key, kv.value)
	}
}

func peekArgStrExists(args []argsKV, key string) (string, bool) {
	for i, n := 0, len(args); i < n; i++ {
		kv := &args[i]
		if string(kv.key) == key {
			return string(kv.value), true
		}
	}
	return "", false
}

// 从 args 切片中获取键 key 对应的参数值。
func peekArgStr(args []argsKV, key string) []byte {
	for i, n := 0, len(args); i < n; i++ {
		kv := &args[i]
		if string(kv.key) == key {
			return kv.value
		}
	}
	return nil
}

// 获取参数切片中指定键的值。
func peekArgBytes(args []argsKV, key []byte) []byte {
	for i, n := 0, len(args); i < n; i++ {
		kv := &args[i]
		if bytes.Equal(kv.key, key) {
			if kv.value != nil {
				return kv.value
			}
			return nilByteSlice
		}
	}
	return nil
}

func peekAllArgBytesToDst(dst [][]byte, h []argsKV, k []byte) [][]byte {
	for i, n := 0, len(h); i < n; i++ {
		kv := &h[i]
		if bytes.Equal(kv.key, k) {
			dst = append(dst, kv.value)
		}
	}
	return dst
}

// 释放切片中的最后一个参数
func releaseArg(args []argsKV) []argsKV {
	return args[:len(args)-1]
}

// 解码源参数字节切片并附加至目标。
// 源参数可能已编码，其中可能包含 % 或 +。
func decodeArgAppend(dst, src []byte) []byte {
	if bytes.IndexByte(src, '%') < 0 && bytes.IndexByte(src, '+') < 0 {
		// 快速路径：src 不包含编码字符
		return append(dst, src...)
	}

	// 慢路径
	for i, n := 0, len(src); i < n; i++ {
		c := src[i]
		if c == '%' {
			if i+2 >= len(src) {
				return append(dst, src[i:]...)
			}
			x2 := bytesconv.Hex2intTable[src[i+2]]
			x1 := bytesconv.Hex2intTable[src[i+1]]
			if x1 == 16 || x2 == 16 {
				dst = append(dst, '%')
			} else {
				dst = append(dst, x1<<4|x2)
				i += 2
			}
		} else if c == '+' {
			dst = append(dst, ' ')
		} else {
			dst = append(dst, c)
		}
	}

	return dst
}

// 解码源参数字节切片并附加至目标。
// 源参数可能已编码，其中可能包含 %，但不包含 +。
func decodeArgAppendNoPlus(dst, src []byte) []byte {
	if bytes.IndexByte(src, '%') < 0 {
		// 快速路径：src 不包含编码字符
		return append(dst, src...)
	}

	// 慢路径，需要做十六进制解码
	for i, n := 0, len(src); i < n; i++ {
		c := src[i]
		if c == '%' {
			if i+2 >= len(src) {
				return append(dst, src[i:]...)
			}
			x2 := bytesconv.Hex2intTable[src[i+2]]
			x1 := bytesconv.Hex2intTable[src[i+1]]
			if x1 == 16 || x2 == 16 {
				dst = append(dst, '%')
			} else {
				dst = append(dst, x1<<4|x2)
				i += 2
			}
		} else {
			dst = append(dst, c)
		}
	}
	return dst
}

func hasArg(args []argsKV, key string) bool {
	for i, n := 0, len(args); i < n; i++ {
		kv := &args[i]
		if key == string(kv.key) {
			return true
		}
	}
	return false
}

// 删除切片中所有与指定键相同的的参数。
func delAllArgs(args []argsKV, key string) []argsKV {
	for i, n := 0, len(args); i < n; i++ {
		kv := &args[i]
		if key == string(kv.key) {
			tmp := *kv
			copy(args[i:], args[i+1:])
			n--
			i--
			args[n] = tmp
			args = args[:n]
		}
	}
	return args
}

// 删除切片中所有与指定键相同的的参数。
func delAllArgsBytes(args []argsKV, key []byte) []argsKV {
	return delAllArgs(args, bytesconv.B2s(key))
}

func copyArgs(dst, src []argsKV) []argsKV {
	if cap(dst) < len(src) {
		tmp := make([]argsKV, len(src))
		copy(tmp, dst)
		dst = tmp
	}
	n := len(src)
	dst = dst[:n]
	for i := 0; i < n; i++ {
		dstKV := &dst[i]
		srcKV := &src[i]
		dstKV.key = append(dstKV.key[:0], srcKV.key...)
		if srcKV.noValue {
			dstKV.value = dstKV.value[:0]
		} else {
			dstKV.value = append(dstKV.value[:0], srcKV.value...)
		}
	}
	return dst
}

// 更新或追加参数切片 args 中指定 key 的 value。
func setArg(args []argsKV, key, value string, noValue bool) []argsKV {
	n := len(args)
	// 更新到已有的同名键下
	for i := 0; i < n; i++ {
		kv := &args[i]
		if key == string(kv.key) {
			if noValue {
				kv.value = kv.value[:0]
			} else {
				kv.value = append(kv.value[:0], value...)
			}
			kv.noValue = noValue
			return args
		}
	}
	// 追加新的键值对
	return appendArg(args, key, value, noValue)
}

// 更新或追加参数切片 args 中指定 key 的 value。
func setArgBytes(args []argsKV, key, value []byte, noValue bool) []argsKV {
	n := len(args)
	for i := 0; i < n; i++ {
		kv := &args[i]
		if bytes.Equal(key, kv.key) {
			if noValue {
				kv.value = kv.value[:0]
			} else {
				kv.value = append(kv.value[:0], value...)
			}
			kv.noValue = noValue
			return args
		}
	}
	return appendArgBytes(args, key, value, noValue)
}

// 附加一对字符串形式的标头。
func appendArg(args []argsKV, key, value string, noValue bool) []argsKV {
	var kv *argsKV
	args, kv = allocArg(args)
	kv.key = append(kv.key[:0], key...)
	if noValue {
		kv.value = kv.value[:0]
	} else {
		kv.value = append(kv.value[:0], value...)
	}
	kv.noValue = noValue
	return args
}

// 附加一对字节切片的形式的标头。
func appendArgBytes(args []argsKV, key, value []byte, noValue bool) []argsKV {
	var kv *argsKV
	args, kv = allocArg(args)
	kv.key = append(kv.key[:0], key...)
	if noValue {
		kv.value = kv.value[:0]
	} else {
		kv.value = append(kv.value[:0], value...)
	}
	kv.noValue = noValue
	return args
}

// 更新标头中指定键的值。
func updateArgBytes(args []argsKV, key, value []byte) []argsKV {
	n := len(args)
	for i := 0; i < n; i++ {
		kv := &args[i]
		if kv.noValue && bytes.Equal(key, kv.key) {
			kv.value = append(kv.value[:0], value...)
			kv.noValue = ArgsHasValue
			return args
		}
	}
	return args
}

// 按需扩容参数切片。
//
// 有容量则扩展1个，容量不足则附加1个（容量可能翻倍）。
//
// 返回扩容后的完整切片及扩容部分的第一个新切片指针。
func allocArg(args []argsKV) ([]argsKV, *argsKV) {
	n := len(args)
	if cap(args) > n {
		args = args[:n+1]
	} else {
		args = append(args, argsKV{})
	}
	return args, &args[n]
}
