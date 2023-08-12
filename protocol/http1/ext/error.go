package ext

import (
	"errors"
	"fmt"
	"io"

	errs "github.com/favbox/gosky/wind/pkg/common/errors"
)

var (
	errNeedMore     = errs.New(errs.ErrNeedMore, errs.ErrorTypePublic, "无法找到换行符")
	errBodyTooLarge = errs.New(errs.ErrBodyTooLarge, errs.ErrorTypePublic, "ext")
)

// HeaderError 返回一个标头错误。
func HeaderError(typ string, err, errParse error, b []byte) error {
	if !errors.Is(errParse, errs.ErrNeedMore) {
		return headerErrorMsg(typ, errParse, b)
	}
	if err == nil {
		return errNeedMore
	}

	// Buggy 服务器可能会在 http 正文之后留下尾随的 CRLF。
	// 将这种情况视为 EOF。
	if isOnlyCRLF(b) {
		return io.EOF
	}

	return headerErrorMsg(typ, err, b)
}

func headerErrorMsg(typ string, err error, b []byte) error {
	return errs.NewPublic(fmt.Sprintf("读取 %s 标头出错: %s。缓冲区大小=%d, 内容: %s", typ, err, len(b), BufferSnippet(b)))
}
