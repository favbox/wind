package adaptor

import (
	"net/http"

	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/consts"
)

type compatResponse struct {
	resp        *protocol.Response
	header      http.Header
	wroteHeader bool
}

func (c *compatResponse) Header() http.Header {
	if c.header != nil {
		return c.header
	}
	c.header = make(map[string][]string)
	return c.header
}

func (c *compatResponse) Write(p []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(consts.StatusOK)
	}

	return c.resp.BodyWriter().Write(p)
}

func (c *compatResponse) WriteHeader(statusCode int) {
	if !c.wroteHeader {
		for k, v := range c.header {
			for _, vv := range v {
				if k == consts.HeaderContentLength {
					continue
				}
				if k == consts.HeaderSetCookie {
					cookie := protocol.AcquireCookie()
					cookie.Parse(vv)
					c.resp.Header.SetCookie(cookie)
					continue
				}
				c.resp.Header.Add(k, vv)
			}
		}
		c.wroteHeader = true
	}

	c.resp.Header.SetStatusCode(statusCode)
}

// GetCompatResponseWriter 获取基础函数兼容的标准库响应写入器，非全部函数。
func GetCompatResponseWriter(resp *protocol.Response) http.ResponseWriter {
	c := &compatResponse{resp: resp}
	c.resp.Header.SetNoDefaultContentType(true)

	h := make(map[string][]string)
	tmpKey := make([][]byte, 0, c.resp.Header.Len())
	c.resp.Header.VisitAll(func(k, v []byte) {
		h[string(k)] = append(h[string(k)], string(v))
		tmpKey = append(tmpKey, k)
	})

	for _, k := range tmpKey {
		c.resp.Header.DelBytes(k)
	}

	c.header = h
	return c
}
