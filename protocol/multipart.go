package protocol

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"

	"github.com/favbox/wind/common/bytebufferpool"
	"github.com/favbox/wind/common/utils"
	"github.com/favbox/wind/network"
	"github.com/favbox/wind/protocol/consts"
)

// MarshalMultipartForm 将表单编码为字节切片。
func MarshalMultipartForm(f *multipart.Form, boundary string) ([]byte, error) {
	var buf bytebufferpool.ByteBuffer
	if err := WriteMultipartForm(&buf, f, boundary); err != nil {
		return nil, err
	}
	return buf.B, nil
}

// WriteMultipartForm 将指定表单 f 和边界值 boundary 写入 w。
func WriteMultipartForm(w io.Writer, f *multipart.Form, boundary string) error {
	// 这里不关心内存分配，因为多部分表单处理很慢。
	if len(boundary) == 0 {
		panic("BUG: 表单边界值 boundary 不能为空")
	}

	mw := multipart.NewWriter(w)
	if err := mw.SetBoundary(boundary); err != nil {
		return fmt.Errorf("无法使用表单边界值 %q: %s", boundary, err)
	}

	// 编码值
	for k, vv := range f.Value {
		for _, v := range vv {
			if err := mw.WriteField(k, v); err != nil {
				return fmt.Errorf("无法写入表单字段 %q 值 %q: %s", k, v, err)
			}
		}
	}

	// 编码文件
	for k, fvv := range f.File {
		for _, fv := range fvv {
			vw, err := mw.CreatePart(fv.Header)
			zw := network.NewWriter(vw)
			if err != nil {
				return fmt.Errorf("无法创建表单文件 %q (%q): %s", k, fv.Filename, err)
			}
			fh, err := fv.Open()
			if err != nil {
				return fmt.Errorf("无法打开表单文件 %q (%q): %s", k, fv.Filename, err)
			}
			if _, err = utils.CopyZeroAlloc(zw, fh); err != nil {
				return fmt.Errorf("拷贝表单文件 %q (%q): %s 发生错误", k, fv.Filename, err)
			}
			if err = fh.Close(); err != nil {
				return fmt.Errorf("无法关闭表单文件 %q (%q): %s", k, fv.Filename, err)
			}
		}
	}

	if err := mw.Close(); err != nil {
		return fmt.Errorf("关闭表单写入器 %s 出现错误", err)
	}

	return nil
}

// ReadMultipartForm 从 r 中读取表单信息。
func ReadMultipartForm(r io.Reader, boundary string, size, maxInMemoryFileSize int) (*multipart.Form, error) {
	// 不用关心此处的内存分派，因为与多部分表单发送的数据（通常几MB）相比，以下内存分配很小。

	if size <= 0 {
		return nil, fmt.Errorf("表单大小必须大于0。给定 %d", size)
	}
	lr := io.LimitReader(r, int64(size))
	mr := multipart.NewReader(lr, boundary)
	f, err := mr.ReadForm(int64(maxInMemoryFileSize))
	if err != nil {
		return nil, fmt.Errorf("无法读取多部分表单数据体: %s", err)
	}
	return f, nil
}

// ParseMultipartForm 从 r 中读取表单信息。
func ParseMultipartForm(r io.Reader, request *Request, size, maxInMemoryFileSize int) error {
	m, err := ReadMultipartForm(r, request.multipartFormBoundary, size, maxInMemoryFileSize)
	if err != nil {
		return err
	}

	request.multipartForm = m
	return nil
}

// SetMultipartFormWithBoundary 设置表单及边界值。
func SetMultipartFormWithBoundary(req *Request, m *multipart.Form, boundary string) {
	req.multipartForm = m
	req.multipartFormBoundary = boundary
}

// AddFile 添加指定文件 path 到写入器 w。
func AddFile(w *multipart.Writer, fieldName, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return WriteMultipartFormFile(w, fieldName, filepath.Base(path), file)
}

func AddMultipartFormField(w *multipart.Writer, mf *MultipartField) error {
	partWriter, err := w.CreatePart(CreateMultipartHeader(mf.Param, mf.FileName, mf.ContentType))
	if err != nil {
		return err
	}

	_, err = io.Copy(partWriter, mf.Reader)
	return err
}

func WriteMultipartFormFile(w *multipart.Writer, fieldName, fileName string, r io.Reader) error {
	// 自动检测实际的多部分表单内容类型
	cBuf := make([]byte, 512)
	size, err := r.Read(cBuf)
	if err != nil && err != io.EOF {
		return err
	}

	partWriter, err := w.CreatePart(CreateMultipartHeader(fieldName, fileName, http.DetectContentType(cBuf[:size])))
	if err != nil {
		return err
	}

	if _, err = partWriter.Write(cBuf[:size]); err != nil {
		return err
	}

	_, err = io.Copy(partWriter, r)
	return err
}

func CreateMultipartHeader(param, fileName, contentType string) textproto.MIMEHeader {
	hdr := make(textproto.MIMEHeader)

	var contentDispositionValue string
	if len(strings.TrimSpace(fileName)) == 0 {
		contentDispositionValue = fmt.Sprintf(`form-data; name="%s"`, param)
	} else {
		contentDispositionValue = fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			param, fileName)
	}
	hdr.Set("Content-Disposition", contentDispositionValue)

	if len(contentType) > 0 {
		hdr.Set(consts.HeaderContentType, contentType)
	}
	return hdr
}
