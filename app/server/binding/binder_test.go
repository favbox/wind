package binding

import (
	"fmt"
	"testing"

	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/consts"
	"github.com/favbox/wind/route/param"
	"github.com/stretchr/testify/assert"
)

type mockRequest struct {
	Req *protocol.Request
}

func newMockRequest() *mockRequest {
	return &mockRequest{
		Req: &protocol.Request{},
	}
}

func (m *mockRequest) SetRequestURI(uri string) *mockRequest {
	m.Req.SetRequestURI(uri)
	return m
}

func (m *mockRequest) SetFile(param, fileName string) *mockRequest {
	m.Req.SetFile(param, fileName)
	return m
}

func (m *mockRequest) SetHeader(key, value string) *mockRequest {
	m.Req.Header.Set(key, value)
	return m
}

func (m *mockRequest) SetHeaders(key, value string) *mockRequest {
	m.Req.Header.Set(key, value)
	return m
}

func (m *mockRequest) SetPostArg(key, value string) *mockRequest {
	m.Req.PostArgs().Add(key, value)
	return m
}

func (m *mockRequest) SetUrlEncodedContentType() *mockRequest {
	m.Req.Header.SetContentTypeBytes([]byte("application/x-www-form-urlencoded"))
	return m
}

func (m *mockRequest) SetJSONContentType() *mockRequest {
	m.Req.Header.SetContentTypeBytes([]byte(consts.MIMEApplicationJSON))
	return m
}

func (m *mockRequest) SetProtobufContentType() *mockRequest {
	m.Req.Header.SetContentTypeBytes([]byte(consts.MIMEPROTOBUF))
	return m
}

func (m *mockRequest) SetBody(data []byte) *mockRequest {
	m.Req.SetBody(data)
	m.Req.Header.SetContentLength(len(data))
	return m
}

func TestBind_BaseType(t *testing.T) {
	type Req struct {
		Version int    `path:"v"`
		ID      int    `query:"id"`
		Header  string `header:"H"`
		Form    string `form:"f"`
	}

	req := newMockRequest().
		SetRequestURI("http://foobar.com?id=12").
		SetHeaders("H", "header").
		SetPostArg("f", "form").
		SetUrlEncodedContentType()
	var params param.Params
	params = append(params, param.Param{
		Key:   "v",
		Value: "1",
	})

	var result Req
	err := DefaultBinder().Bind(req.Req, &result, params)
	assert.Nil(t, err)
	assert.Equal(t, 1, result.Version)
	assert.Equal(t, 12, result.ID)
	assert.Equal(t, "header", result.Header)
	assert.Equal(t, "form", result.Form)
}

func TestBind_SliceType(t *testing.T) {
	type Req struct {
		ID   *[]int    `query:"id"`
		Str  [3]string `query:"str"`
		Byte []byte    `query:"b"`
	}

	IDs := []int{11, 12, 13}
	Strs := [3]string{"qwe", "asd", "zxc"}
	Bytes := []byte("123")

	req := newMockRequest().
		SetRequestURI(fmt.Sprintf("http://foobar.com?id=%d&id=%d&id=%d&str=%s&str=%s&str=%s&b=%d&b=%d&b=%d", IDs[0], IDs[1], IDs[2], Strs[0], Strs[1], Strs[2], Bytes[0], Bytes[1], Bytes[2]))

	var result Req
	err := DefaultBinder().Bind(req.Req, &result, nil)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(*result.ID))
	for idx, val := range IDs {
		assert.Equal(t, val, (*result.ID)[idx])
	}
	for idx, val := range Strs {
		assert.Equal(t, val, result.Str[idx])
	}
	assert.Equal(t, 3, len(result.Byte))
	for idx, val := range Bytes {
		assert.Equal(t, val, result.Byte[idx])
	}
}
