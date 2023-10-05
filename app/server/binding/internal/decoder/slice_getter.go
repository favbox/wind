package decoder

import (
	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/route/param"
)

type sliceGetter func(req *protocol.Request, params param.Params, key string, defaultValue ...string) (ret []string)

func pathSlice(_ *protocol.Request, params param.Params, key string, defaultValue ...string) (ret []string) {
	var value string
	if params != nil {
		value, _ = params.Get(key)
	}

	if len(value) == 0 && len(defaultValue) != 0 {
		value = defaultValue[0]
	}
	if len(value) != 0 {
		ret = append(ret, value)
	}

	return
}

func querySlice(req *protocol.Request, _ param.Params, key string, defaultValue ...string) (ret []string) {
	req.URI().QueryArgs().VisitAll(func(queryKey, value []byte) {
		if key == bytesconv.B2s(queryKey) {
			ret = append(ret, string(value))
		}
	})

	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = append(ret, defaultValue...)
	}

	return
}

func headerSlice(req *protocol.Request, _ param.Params, key string, defaultValue ...string) (ret []string) {
	req.Header.VisitAll(func(headerKey, value []byte) {
		if key == bytesconv.B2s(headerKey) {
			ret = append(ret, string(value))
		}
	})

	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = append(ret, defaultValue...)
	}

	return
}

func cookieSlice(req *protocol.Request, _ param.Params, key string, defaultValue ...string) (ret []string) {
	req.Header.VisitAllCookie(func(cookieKey, value []byte) {
		if key == bytesconv.B2s(cookieKey) {
			ret = append(ret, string(value))
		}
	})

	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = append(ret, defaultValue...)
	}

	return
}

func postFormSlice(req *protocol.Request, _ param.Params, key string, defaultValue ...string) (ret []string) {
	req.PostArgs().VisitAll(func(formKey, value []byte) {
		if key == bytesconv.B2s(formKey) {
			ret = append(ret, string(value))
		}
	})
	if len(ret) != 0 {
		return
	}

	mf, err := req.MultipartForm()
	if err == nil && mf.Value != nil {
		for k, v := range mf.Value {
			if k == key && len(v) > 0 {
				ret = append(ret, v...)
			}
		}
	}
	if len(ret) != 0 {
		return
	}

	if len(ret) == 0 && len(defaultValue) != 0 {
		ret = append(ret, defaultValue...)
	}

	return
}

func rawBodySlice(req *protocol.Request, _ param.Params, key string, _ ...string) (ret []string) {
	if req.Header.ContentLength() > 0 {
		ret = append(ret, string(req.Body()))
	}

	return
}
