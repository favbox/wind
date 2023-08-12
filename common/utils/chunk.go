package utils

import (
	"bytes"
	"io"

	"github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/internal/bytestr"
	"github.com/favbox/wind/network"
)

var errBrokenChunk = errors.NewPublic("无法在分块数据结尾找到 crlf")

// ParseChunkSize 解析 r 的分块个数。
func ParseChunkSize(r network.Reader) (int, error) {
	n, err := bytesconv.ReadHexInt(r)
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return -1, err
	}
	for {
		c, err := r.ReadByte()
		if err != nil {
			return -1, errors.NewPublicf("无法在块大小的后面读到 '\r': %s", err)
		}
		// 跳过块大小后尾随的所有空白
		if c == ' ' {
			continue
		}
		if c != '\r' {
			return -1, errors.NewPublicf("块大小的后面发现异常字符 %q。期望 %q", c, '\r')
		}
		break
	}
	c, err := r.ReadByte()
	if err != nil {
		return -1, errors.NewPublicf("无法在块大小的后面读到 '\n': %s", err)
	}
	if c != '\n' {
		return -1, errors.NewPublicf("块大小的后面发现异常字符 %q。期望 %q", c, '\n')
	}
	return n, nil
}

// SkipCRLF 跳过读取器开头的回车换行符 crlf。
func SkipCRLF(reader network.Reader) error {
	p, err := reader.Peek(len(bytestr.StrCRLF))
	reader.Skip(len(p))
	if err != nil {
		return err
	}
	if !bytes.Equal(p, bytestr.StrCRLF) {
		return errBrokenChunk
	}

	return nil
}
