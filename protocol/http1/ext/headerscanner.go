package ext

import (
	"bytes"

	errs "github.com/favbox/gosky/wind/pkg/common/errors"
	"github.com/favbox/gosky/wind/pkg/common/utils"
)

var errInvalidName = errs.NewPublic("无效的标头名称")

// HeaderScanner 标头扫描器，用于进行 Next 迭代。
type HeaderScanner struct {
	B     []byte
	Key   []byte
	Value []byte
	Err   error

	// 标头子切片的长度
	HLen int

	DisableNormalizing bool

	// 通过判断下一行是否包含冒号来判断是标头还是当前标头的多行值。
	// 该操作的副作用是我们知道了下一个冒号和新行的索引，所以在 Next 迭代时就不需要再找了。
	nextColon   int
	nextNewLine int

	initialized bool
}

func (s *HeaderScanner) Next() bool {
	if !s.initialized {
		s.nextColon = -1
		s.nextNewLine = -1
		s.initialized = true
	}
	bLen := len(s.B)
	if bLen >= 2 && s.B[0] == '\r' && s.B[1] == '\n' {
		s.B = s.B[2:]
		s.HLen += 2
		return false
	}
	if bLen >= 1 && s.B[0] == '\n' {
		s.B = s.B[1:]
		s.HLen++
		return false
	}
	var n int
	if s.nextColon >= 0 {
		n = s.nextColon
		s.nextColon = -1
	} else {
		n = bytes.IndexByte(s.B, ':')

		// 标头名称中不能有换行符 \n，此处进行检查。
		x := bytes.IndexByte(s.B, '\n')
		if x < 0 {
			// 标头名称应始终在其后跟一个 \n，即使它是终止标头块的名称。
			s.Err = errNeedMore
			return false
		}
		if x < n {
			// : 前有换行符 \n
			s.Err = errInvalidName
			return false
		}
	}
	if n < 0 {
		s.Err = errNeedMore
		return false
	}
	s.Key = s.B[:n]
	utils.NormalizeHeaderKey(s.Key, s.DisableNormalizing)
	n++
	for len(s.B) > n && s.B[n] == ' ' {
		n++
		// the newline index is a relative index, and lines below trimmed `s.b` by `n`,
		// so the relative newline index also shifted forward. it's safe to decrease
		// to a minus value, it means it's invalid, and will find the newline again.
		s.nextNewLine--
	}
	s.HLen += n
	s.B = s.B[n:]
	if s.nextNewLine >= 0 {
		n = s.nextNewLine
		s.nextNewLine = -1
	} else {
		n = bytes.IndexByte(s.B, '\n')
	}
	if n < 0 {
		s.Err = errNeedMore
		return false
	}
	isMultiLineValue := false
	for {
		if n+1 >= len(s.B) {
			break
		}
		if s.B[n+1] != ' ' && s.B[n+1] != '\t' {
			break
		}
		d := bytes.IndexByte(s.B[n+1:], '\n')
		if d <= 0 {
			break
		} else if d == 1 && s.B[n+1] == '\r' {
			break
		}
		e := n + d + 1
		if c := bytes.IndexByte(s.B[n+1:e], ':'); c >= 0 {
			s.nextColon = c
			s.nextNewLine = d - c - 1
			break
		}
		isMultiLineValue = true
		n = e
	}
	if n >= len(s.B) {
		s.Err = errNeedMore
		return false
	}
	oldB := s.B
	s.Value = s.B[:n]
	s.HLen += n + 1
	s.B = s.B[n+1:]

	if n > 0 && s.Value[n-1] == '\r' {
		n--
	}
	for n > 0 && s.Value[n-1] == ' ' {
		n--
	}
	s.Value = s.Value[:n]
	if isMultiLineValue {
		s.Value, s.B, s.HLen = normalizeHeaderValue(s.Value, oldB, s.HLen)
	}
	return true
}

type HeaderValueScanner struct {
	B     []byte
	Value []byte
}

// 按逗号分割，判断是否还有值。
func (s *HeaderValueScanner) next() bool {
	b := s.B
	if len(b) == 0 {
		return false
	}
	n := bytes.IndexByte(b, ',')
	if n < 0 {
		s.Value = stripSpace(b)
		s.B = b[len(b):]
		return true
	}
	s.Value = stripSpace(b[:n])
	s.B = b[n+1:]
	return true
}

// HasHeaderValue 判断 s 中是否有标头值 value。
func HasHeaderValue(s, value []byte) bool {
	var vs HeaderValueScanner
	vs.B = s
	for vs.next() {
		if utils.CaseInsensitiveCompare(vs.Value, value) {
			return true
		}
	}
	return false
}
