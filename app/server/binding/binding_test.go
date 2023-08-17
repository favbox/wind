package binding

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"reflect"
	"testing"

	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/route/param"
	"github.com/stretchr/testify/assert"
)

func TestBindAndValidate(t *testing.T) {
	type TestBind struct {
		A string               `query:"a"`
		B []string             `query:"b"`
		C string               `query:"c"`
		D string               `header:"d"`
		E string               `path:"e"`
		F string               `form:"f"`
		G multipart.FileHeader `form:"g"`
		H string               `cookie:"h"`
	}

	s := `------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="f"

fff
------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="g"; filename="TODO"
Content-Type: application/octet-stream

- SessionClient with referer and cookies support.
- Client with requests' pipelining support.
- ProxyHandler similar to FSHandler.
- WebSockets. See https://tools.ietf.org/html/rfc6455 .
- HTTP/2.0. See https://tools.ietf.org/html/rfc7540 .

------WebKitFormBoundaryJwfATyF8tmxSJnLg--
tailfoobar`

	mr := bytes.NewBufferString(s)
	r := protocol.NewRequest("POST", "/foo", mr)
	r.SetRequestURI("/foo/bar?a=aaa&b=b1&b=b2&c&i=19")
	r.SetHeader("d", "ddd")
	r.Header.SetContentLength(len(s))
	r.Header.SetContentTypeBytes([]byte("multipart/form-data; boundary=----WebKitFormBoundaryJwfATyF8tmxSJnLg"))

	r.SetCookie("h", "hhh")

	para := param.Params{
		{Key: "e", Value: "eee"},
	}

	// 测试绑定并验证
	SetLooseZeroMode(true)
	var req TestBind
	err := BindAndValidate(r, &req, para)
	assert.Nil(t, err)
	assert.Equal(t, "aaa", req.A)
	assert.Equal(t, 2, len(req.B))
	assert.Equal(t, "", req.C)
	assert.Equal(t, "ddd", req.D)
	assert.Equal(t, "eee", req.E)
	assert.Equal(t, "fff", req.F)
	assert.Equal(t, "TODO", req.G.Filename)
	assert.Equal(t, "hhh", req.H)

	// 测试绑定
	req = TestBind{}
	err = Bind(r, &req, para)
	assert.Nil(t, err)
	assert.Equal(t, "aaa", req.A)
	assert.Equal(t, 2, len(req.B))
	assert.Equal(t, "", req.C)
	assert.Equal(t, "ddd", req.D)
	assert.Equal(t, "eee", req.E)
	assert.Equal(t, "fff", req.F)
	assert.Equal(t, "TODO", req.G.Filename)
	assert.Equal(t, "hhh", req.H)

	type TestValidate struct {
		I int `query:"i" vd:"$>20"`
	}

	// 测试绑定并验证
	var bindReq TestValidate
	err = BindAndValidate(r, &bindReq, para)
	assert.NotNil(t, err)

	// 测试绑定
	bindReq = TestValidate{}
	err = Bind(r, &bindReq, para)
	assert.Nil(t, err)
	assert.Equal(t, 19, bindReq.I)
	err = Validate(&bindReq)
	assert.NotNil(t, err)
}

func TestJsonBind(t *testing.T) {
	data := `{"a":"aaa", "b":["b1","b2"], "c":"ccc", "d":"100"}`
	mr := bytes.NewBufferString(data)
	r := protocol.NewRequest("POST", "/foo", mr)
	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	r.SetHeader("d", "ddd")
	r.Header.SetContentLength(len(data))

	type Test struct {
		A string   `json:"a"`
		B []string `json:"b"`
		C string   `json:"c"`
		D int      `json:"d,string"`
	}
	var req Test

	err := BindAndValidate(r, &req, nil)
	assert.Nil(t, err)
	assert.Equal(t, "aaa", req.A)
	assert.Equal(t, 2, len(req.B))
	assert.Equal(t, "ccc", req.C)
	// 注意: 默认情况下不支持json中的字符串到go的int转换。
	// 你可添加 "string" 标记或用其他 json 解码库来支持此特性。
	assert.Equal(t, 100, req.D)

	req = Test{}
	UseGJSONUnmarshaler()
	err = BindAndValidate(r, &req, nil)
	assert.Nil(t, err)
	assert.Equal(t, "aaa", req.A)
	assert.Equal(t, 2, len(req.B))
	assert.Equal(t, "ccc", req.C)
	// 注意: 默认情况下不支持json中的字符串到go的int转换。
	// 你可添加 "string" 标记或用其他 json 解码库来支持此特性。
	assert.Equal(t, 100, req.D)
}

// TestQueryParamInconsistency 测试 GetQuery() 的不一致性，request.go 中 GetFunc() 的另一个单元测试与此类似。
func TestQueryParamInconsistency(t *testing.T) {
	r := protocol.NewRequest("GET", "/foo", nil)
	r.SetRequestURI("/foo/bar?para1=hertz&para2=binding")

	type QueryPara struct {
		Para1 string  `query:"para1"`
		Para2 *string `query:"para2"`
	}
	var req QueryPara
	err := BindAndValidate(r, &req, nil)
	assert.Nil(t, err)

	beforePara1 := deepCopyString(req.Para1)
	beforePara2 := deepCopyString(*req.Para2)
	r.URI().QueryArgs().Set("para1", "test")
	r.URI().QueryArgs().Set("para2", "test")
	afterPara1 := req.Para1
	afterPara2 := *req.Para2
	assert.Equal(t, beforePara1, afterPara1)
	assert.Equal(t, beforePara2, afterPara2)
}

func deepCopyString(str string) string {
	tmp := make([]byte, len(str))
	copy(tmp, str)
	c := string(tmp)

	return c
}

func TestBindingFile(t *testing.T) {
	type FileParas struct {
		F   *multipart.FileHeader `form:"F1"`
		F1  multipart.FileHeader
		Fs  []multipart.FileHeader  `form:"F1"`
		Fs1 []*multipart.FileHeader `form:"F1"`
		F2  *multipart.FileHeader   `form:"F2"`
	}

	s := `------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="f"

fff
------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="F1"; filename="TODO1"
Content-Type: application/octet-stream

- SessionClient with referer and cookies support.
- Client with requests' pipelining support.
- ProxyHandler similar to FSHandler.
- WebSockets. See https://tools.ietf.org/html/rfc6455 .
- HTTP/2.0. See https://tools.ietf.org/html/rfc7540 .
------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="F1"; filename="TODO2"
Content-Type: application/octet-stream

- SessionClient with referer and cookies support.
- Client with requests' pipelining support.
- ProxyHandler similar to FSHandler.
- WebSockets. See https://tools.ietf.org/html/rfc6455 .
- HTTP/2.0. See https://tools.ietf.org/html/rfc7540 .
------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="F2"; filename="TODO3"
Content-Type: application/octet-stream

- SessionClient with referer and cookies support.
- Client with requests' pipelining support.
- ProxyHandler similar to FSHandler.
- WebSockets. See https://tools.ietf.org/html/rfc6455 .
- HTTP/2.0. See https://tools.ietf.org/html/rfc7540 .

------WebKitFormBoundaryJwfATyF8tmxSJnLg--
tailfoobar`

	mr := bytes.NewBufferString(s)
	r := protocol.NewRequest("POST", "/foo", mr)
	r.SetRequestURI("/foo/bar?a=aaa&b=b1&b=b2&c&i=19")
	r.SetHeader("d", "ddd")
	r.Header.SetContentLength(len(s))
	r.Header.SetContentTypeBytes([]byte("multipart/form-data; boundary=----WebKitFormBoundaryJwfATyF8tmxSJnLg"))

	var req FileParas
	err := BindAndValidate(r, &req, nil)
	assert.Nil(t, err)
	assert.Equal(t, "TODO1", req.F.Filename)
	assert.Equal(t, "TODO1", req.F1.Filename)
	assert.Equal(t, 2, len(req.Fs))
	assert.Equal(t, 2, len(req.Fs1))
	assert.Equal(t, "TODO3", req.F2.Filename)
}

type BindError struct {
	ErrType, FailField, Msg string
}

// Error implements error interface.
func (e *BindError) Error() string {
	if e.Msg != "" {
		return e.ErrType + ": expr_path=" + e.FailField + ", cause=" + e.Msg
	}
	return e.ErrType + ": expr_path=" + e.FailField + ", cause=invalid"
}

type ValidateError struct {
	ErrType, FailField, Msg string
}

// Error implements error interface.
func (e *ValidateError) Error() string {
	if e.Msg != "" {
		return e.ErrType + ": expr_path=" + e.FailField + ", cause=" + e.Msg
	}
	return e.ErrType + ": expr_path=" + e.FailField + ", cause=invalid"
}

func TestSetErrorFactory(t *testing.T) {
	type TestBind struct {
		A string `query:"a,required"`
	}

	r := protocol.NewRequest("GET", "/foo", nil)
	r.SetRequestURI("/foo/bar?b=20")

	CustomBindErrFunc := func(failField, msg string) error {
		err := BindError{
			ErrType:   "bindErr",
			FailField: "[bindFailField]: " + failField,
			Msg:       "[bindErrMsg]: " + msg,
		}

		return &err
	}

	CustomValidateErrFunc := func(failField, msg string) error {
		err := ValidateError{
			ErrType:   "validateErr",
			FailField: "[validateFailField]: " + failField,
			Msg:       "[validateErrMsg]: " + msg,
		}

		return &err
	}

	SetErrorFactory(CustomBindErrFunc, CustomValidateErrFunc)

	var req TestBind
	err := Bind(r, &req, nil)
	if err == nil {
		t.Fatalf("unexpected nil, expected an error")
	}
	assert.Equal(t, "bindErr: expr_path=[bindFailField]: A, cause=[bindErrMsg]: missing required parameter", err.Error())

	type TestValidate struct {
		B int `query:"b" vd:"$>100"`
	}

	var reqValidate TestValidate
	err = Bind(r, &reqValidate, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = Validate(&reqValidate)
	if err == nil {
		t.Fatalf("unexpected nil, expected an error")
	}
	assert.Equal(t, "validateErr: expr_path=[validateFailField]: B, cause=[validateErrMsg]: ", err.Error())
}

func TestMustRegTypeUnmarshal(t *testing.T) {
	type Nested struct {
		B string
		C string
	}

	type TestBind struct {
		A Nested `query:"a,required"`
	}

	r := protocol.NewRequest("GET", "/foo", nil)
	r.SetRequestURI("/foo/bar?a=windbinding")

	MustRegTypeUnmarshal(reflect.TypeOf(Nested{}), func(v string, emptyAsZero bool) (reflect.Value, error) {
		if v == "" && emptyAsZero {
			return reflect.ValueOf(Nested{}), nil
		}
		val := Nested{
			B: v[:4],
			C: v[4:],
		}
		return reflect.ValueOf(val), nil
	})

	var req TestBind
	err := Bind(r, &req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.Equal(t, "wind", req.A.B)
	assert.Equal(t, "binding", req.A.C)
}

func TestMustRegValidateFunc(t *testing.T) {
	type TestValidate struct {
		A string `query:"a" vd:"test($)"`
	}

	r := protocol.NewRequest("GET", "/foo", nil)
	r.SetRequestURI("/foo/bar?a=123")

	MustRegValidateFunc("test", func(args ...any) error {
		if len(args) != 1 {
			return fmt.Errorf("the args must be one")
		}
		s, _ := args[0].(string)
		if s == "123" {
			return fmt.Errorf("the args can not be 123")
		}
		return nil
	})

	var req TestValidate
	err := Bind(r, &req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = Validate(&req)
	if err == nil {
		t.Fatalf("unexpected nil, expected an error")
	}
}

func TestQueryAlias(t *testing.T) {
	type MyInt int
	type MyString string
	type MyIntSlice []int
	type MyStringSlice []string
	type Test struct {
		A []MyInt       `query:"a"`
		B MyIntSlice    `query:"b"`
		C MyString      `query:"c"`
		D MyStringSlice `query:"d"`
	}

	r := protocol.NewRequest("GET", "/foo", nil)
	r.SetRequestURI("/foo/bar?a=1&a=2&b=2&b=3&c=string1&d=string2&d=string3")

	var req Test
	err := Bind(r, &req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
		return
	}
	assert.Equal(t, 2, len(req.A))
	assert.Equal(t, 1, int(req.A[0]))
	assert.Equal(t, 2, int(req.A[1]))
	assert.Equal(t, 2, len(req.B))
	assert.Equal(t, 2, req.B[0])
	assert.Equal(t, 3, req.B[1])
	assert.Equal(t, "string1", string(req.C))
	assert.Equal(t, 2, len(req.D))
	assert.Equal(t, "string2", req.D[0])
	assert.Equal(t, "string3", req.D[1])
}