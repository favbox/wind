package protocol

import (
	"bytes"

	errs "github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/common/utils"
	"github.com/favbox/wind/internal/bytestr"
	"github.com/favbox/wind/protocol/consts"
)

// Trailer 是 HTTP 响应标头的挂车，规定了哪个标头用来挂载分块消息的元信息。
type Trailer struct {
	h                  []argsKV
	bufKV              argsKV
	disableNormalizing bool
}

// Get 返回指定键的值。
func (t *Trailer) Get(key string) string {
	return string(t.Peek(key))
}

// Peek 返回指定键的值。别存值引用。改用副本。
func (t *Trailer) Peek(key string) []byte {
	k := getHeaderKeyBytes(&t.bufKV, key, t.disableNormalizing)
	return peekArgBytes(t.h, k)
}

// Del 删除所有指定键。
func (t *Trailer) Del(key string) {
	k := getHeaderKeyBytes(&t.bufKV, key, t.disableNormalizing)
	t.h = delAllArgsBytes(t.h, k)
}

// VisitAll 在每个头信息应用函数 f。
func (t *Trailer) VisitAll(f func(key, value []byte)) {
	visitArgs(t.h, f)
}

// Set 设置指定的键值对 Trailer。
func (t *Trailer) Set(key, value string) error {
	initHeaderKV(&t.bufKV, key, value, t.disableNormalizing)
	return t.setArgBytes(t.bufKV.key, t.bufKV.value, ArgsHasValue)
}

// Add 添加指定键的值。 支持单键多值，如需单键单值可使用 Set。
func (t *Trailer) Add(key, value string) error {
	initHeaderKV(&t.bufKV, key, value, t.disableNormalizing)
	return t.addArgBytes(t.bufKV.key, t.bufKV.value, ArgsHasValue)
}

// UpdateArgBytes 更新标头。
func (t *Trailer) UpdateArgBytes(key, value []byte) error {
	if IsBadTrailer(key) {
		return errs.NewPublicf("禁止使用的 Trailer 键: %q", key)
	}

	t.h = updateArgBytes(t.h, key, value)
	return nil
}

// GetTrailers 获取标头中的 Trailer 切片。
func (t *Trailer) GetTrailers() []argsKV {
	return t.h
}

// Empty 判断 Trailer 标头切片是否为空。
func (t *Trailer) Empty() bool {
	return len(t.h) == 0
}

// GetBytes 返回英文逗号分割的 Trailer 键名称字节切片。
func (t *Trailer) GetBytes() []byte {
	var dst []byte
	for i, n := 0, len(t.h); i < n; i++ {
		kv := &t.h[i]
		dst = append(dst, kv.key...)
		if i+1 < n {
			dst = append(dst, bytestr.StrCommaSpace...)
		}
	}
	return dst
}

// ResetSkipNormalize 重置 Trailer 标头切片，但跳过 disableNormalizing。
func (t *Trailer) ResetSkipNormalize() {
	t.h = t.h[:0]
}

// Reset 重置 Trailer 标头切片，并初始 disableNormalizing。
func (t *Trailer) Reset() {
	t.disableNormalizing = false
	t.ResetSkipNormalize()
}

// DisableNormalizing 禁用键名称的规范化。
func (t *Trailer) DisableNormalizing() {
	t.disableNormalizing = true
}

// IsDisableNormalizing 判断当前是否禁用键名称的规范化。
func (t *Trailer) IsDisableNormalizing() bool {
	return t.disableNormalizing
}

// CopyTo 复制 Trailer 到目标。
func (t *Trailer) CopyTo(dst *Trailer) {
	dst.Reset()

	dst.disableNormalizing = t.disableNormalizing
	dst.h = copyArgs(dst.h, t.h)
}

// SetTrailers 按指定的 trailers 切片，批量设置一组 Trailer。
func (t *Trailer) SetTrailers(trailers []byte) (err error) {
	t.ResetSkipNormalize()
	for i := -1; i+1 < len(trailers); {
		trailers = trailers[i+1:]
		i = bytes.IndexByte(trailers, ',')
		if i < 0 {
			i = len(trailers)
		}
		trailerKey := trailers[:i]
		for len(trailerKey) > 0 && trailerKey[0] == ' ' {
			trailerKey = trailerKey[1:]
		}
		for len(trailerKey) > 0 && trailerKey[len(trailerKey)-1] == ' ' {
			trailerKey = trailerKey[:len(trailerKey)-1]
		}

		utils.NormalizeHeaderKey(trailerKey, t.disableNormalizing)
		err = t.addArgBytes(trailerKey, nilByteSlice, argsNoValue)
	}
	return
}

// Header 返回 Trailer 标头的分行字节切片。
func (t *Trailer) Header() []byte {
	t.bufKV.value = t.AppendBytes(t.bufKV.value[:0])
	return t.bufKV.value
}

// AppendBytes 按行附加到 dst 并返回。
func (t *Trailer) AppendBytes(dst []byte) []byte {
	for i, n := 0, len(t.h); i < n; i++ {
		kv := &t.h[i]
		dst = appendHeaderLine(dst, kv.key, kv.value)
	}

	dst = append(dst, bytestr.StrCRLF...)
	return dst
}

// 添加指定键的值。
func (t *Trailer) addArgBytes(key []byte, value []byte, noValue bool) error {
	if IsBadTrailer(key) {
		return errs.NewPublicf("禁止使用的 Trailer 键: %q", key)
	}
	t.h = appendArgBytes(t.h, key, value, noValue)
	return nil
}

// 更新或追加指定键的值。
func (t *Trailer) setArgBytes(key, value []byte, noValue bool) error {
	if IsBadTrailer(key) {
		return errs.NewPublicf("禁止使用的 Trailer 键: %q", key)
	}
	t.h = setArgBytes(t.h, key, value, noValue)
	return nil
}

// IsBadTrailer 判断指定的 key 是否为禁用的键名称。
func IsBadTrailer(key []byte) bool {
	switch key[0] | 0x20 {
	case 'a':
		return utils.CaseInsensitiveCompare(key, bytestr.StrAuthorization)
	case 'c':
		if len(key) >= len(consts.HeaderContentType) && utils.CaseInsensitiveCompare(key[:8], bytestr.StrContentType[:8]) {
			// skip compare prefix 'Content-'
			return utils.CaseInsensitiveCompare(key[8:], bytestr.StrContentEncoding[8:]) ||
				utils.CaseInsensitiveCompare(key[8:], bytestr.StrContentLength[8:]) ||
				utils.CaseInsensitiveCompare(key[8:], bytestr.StrContentType[8:]) ||
				utils.CaseInsensitiveCompare(key[8:], bytestr.StrContentRange[8:])
		}
		return utils.CaseInsensitiveCompare(key, bytestr.StrConnection)
	case 'e':
		return utils.CaseInsensitiveCompare(key, bytestr.StrExpect)
	case 'h':
		return utils.CaseInsensitiveCompare(key, bytestr.StrHost)
	case 'k':
		return utils.CaseInsensitiveCompare(key, bytestr.StrKeepAlive)
	case 'm':
		return utils.CaseInsensitiveCompare(key, bytestr.StrMaxForwards)
	case 'p':
		if len(key) >= len(consts.HeaderProxyConnection) && utils.CaseInsensitiveCompare(key[:6], bytestr.StrProxyConnection[:6]) {
			// 跳过对比前缀 'Proxy-'
			return utils.CaseInsensitiveCompare(key[6:], bytestr.StrProxyConnection[6:]) ||
				utils.CaseInsensitiveCompare(key[6:], bytestr.StrProxyAuthenticate[6:]) ||
				utils.CaseInsensitiveCompare(key[6:], bytestr.StrProxyAuthorization[6:])
		}

	case 'r':
		return utils.CaseInsensitiveCompare(key, bytestr.StrRange)
	case 't':
		return utils.CaseInsensitiveCompare(key, bytestr.StrTE) ||
			utils.CaseInsensitiveCompare(key, bytestr.StrTrailer) ||
			utils.CaseInsensitiveCompare(key, bytestr.StrTransferEncoding)
	case 'w':
		return utils.CaseInsensitiveCompare(key, bytestr.StrWWWAuthenticate)
	}
	return false
}
