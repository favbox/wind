package decoder

import (
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/route/param"
)

type getter func(req *protocol.Request, params param.Params, key string, defaultValue ...string) (ret string, exists bool)

func path(_ *protocol.Request, params param.Params, key string, defaultValue ...string) (ret string, exists bool) {
	if params != nil {
		ret, exists = params.Get(key)
	}

	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = defaultValue[0]
	}
	return ret, exists
}

func query(req *protocol.Request, _ param.Params, key string, defaultValue ...string) (ret string, exists bool) {
	if ret, exists = req.URI().QueryArgs().PeekExists(key); exists {
		return
	}

	if len(ret) == 0 && len(defaultValue) > 0 {
		ret = defaultValue[0]
	}

	return
}

func header(req *protocol.Request, _ param.Params, key string, defaultValue ...string) (ret string, exists bool) {
	if val := req.Header.Peek(key); val != nil {
		ret = string(val)
		return ret, true
	}

	if len(ret) == 0 && len(defaultValue) > 0 {
		ret = defaultValue[0]
	}

	return ret, false
}

func cookie(req *protocol.Request, _ param.Params, key string, defaultValue ...string) (ret string, exists bool) {
	if val := req.Header.Cookie(key); val != nil {
		ret = string(val)
		return ret, true
	}

	if len(ret) == 0 && len(defaultValue) > 0 {
		ret = defaultValue[0]
	}

	return ret, false
}

func postForm(req *protocol.Request, _ param.Params, key string, defaultValue ...string) (ret string, exists bool) {
	if ret, exists = req.PostArgs().PeekExists(key); exists {
		return
	}

	mf, err := req.MultipartForm()
	if err == nil && mf.Value != nil {
		for k, v := range mf.Value {
			if k == key && len(v) > 0 {
				ret = v[0]
			}
		}
	}

	if len(ret) != 0 {
		return ret, true
	}

	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = defaultValue[0]
	}

	return ret, false
}

func rawBody(req *protocol.Request, _ param.Params, key string, _ ...string) (ret string, exists bool) {
	if req.Header.ContentLength() > 0 {
		ret = string(req.Body())
		exists = true
	}

	return
}
