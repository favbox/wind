package binding

import (
	"mime/multipart"
	"net/http"
	"net/url"

	"github.com/bytedance/go-tagexpr/v2/binding"
	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/protocol"
)

func wrapRequest(req *protocol.Request) binding.Request {
	r := &bindRequest{
		req: req,
	}
	return r
}

type bindRequest struct {
	req *protocol.Request
}

func (r *bindRequest) GetMethod() string {
	return bytesconv.B2s(r.req.Method())
}

func (r *bindRequest) GetQuery() url.Values {
	queryMap := make(url.Values)
	r.req.URI().QueryArgs().VisitAll(func(key, value []byte) {
		keyStr := string(key)
		values := queryMap[keyStr]
		values = append(values, string(value))
		queryMap[keyStr] = values
	})

	return queryMap
}

func (r *bindRequest) GetContentType() string {
	return bytesconv.B2s(r.req.Header.ContentType())
}

func (r *bindRequest) GetHeader() http.Header {
	header := make(http.Header)
	r.req.Header.VisitAll(func(key, value []byte) {
		keyStr := string(key)
		values := header[keyStr]
		values = append(values, string(value))
		header[keyStr] = values
	})

	return header
}

func (r *bindRequest) GetCookies() []*http.Cookie {
	var cookies []*http.Cookie
	r.req.Header.VisitAllCookie(func(key, value []byte) {
		cookies = append(cookies, &http.Cookie{
			Name:  string(key),
			Value: string(value),
		})
	})

	return cookies
}

func (r *bindRequest) GetBody() ([]byte, error) {
	return r.req.Body(), nil
}

func (r *bindRequest) GetPostForm() (url.Values, error) {
	postMap := make(url.Values)
	r.req.PostArgs().VisitAll(func(key, value []byte) {
		keyStr := string(key)
		values := postMap[keyStr]
		values = append(values, string(value))
		postMap[keyStr] = values
	})
	mf, err := r.req.MultipartForm()
	if err == nil {
		for k, v := range mf.Value {
			if len(v) > 0 {
				postMap[k] = v
			}
		}
	}

	return postMap, nil
}

func (r *bindRequest) GetForm() (url.Values, error) {
	formMap := make(url.Values)
	r.req.URI().QueryArgs().VisitAll(func(key, value []byte) {
		keyStr := string(key)
		values := formMap[keyStr]
		values = append(values, string(value))
		formMap[keyStr] = values
	})
	r.req.PostArgs().VisitAll(func(key, value []byte) {
		keyStr := string(key)
		values := formMap[keyStr]
		values = append(values, string(value))
		formMap[keyStr] = values
	})

	return formMap, nil
}

func (r *bindRequest) GetFileHeaders() (map[string][]*multipart.FileHeader, error) {
	files := make(map[string][]*multipart.FileHeader)
	mf, err := r.req.MultipartForm()
	if err == nil {
		for k, v := range mf.File {
			if len(v) > 0 {
				files[k] = v
			}
		}
	}

	return files, nil
}
