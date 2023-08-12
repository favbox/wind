package adaptor

import (
	"bytes"
	"net/http"

	"github.com/favbox/wind/protocol"
)

// GetCompatRequest 获取基础函数兼容的标准库请求，非全部函数。
func GetCompatRequest(req *protocol.Request) (*http.Request, error) {
	r, err := http.NewRequest(string(req.Method()), req.URI().String(), bytes.NewReader(req.Body()))
	if err != nil {
		return nil, err
	}

	h := make(map[string][]string)
	req.Header.VisitAll(func(key, value []byte) {
		h[string(key)] = append(h[string(key)], string(value))
	})
	r.Header = h

	return r, nil
}

// CopyToWindRequest 拷贝标准库请求的网址、主机、方法、协议、标头，且共享正文读取器。
func CopyToWindRequest(r *http.Request, req *protocol.Request) error {
	req.Header.SetRequestURI(r.RequestURI)
	req.Header.SetHost(r.Host)
	req.Header.SetMethod(r.Method)
	req.Header.SetProtocol(r.Proto)
	for k, v := range r.Header {
		for _, vv := range v {
			req.Header.Add(k, vv)
		}
	}
	if r.Body != nil {
		req.SetBodyStream(r.Body, req.Header.ContentLength())
	}
	return nil
}
