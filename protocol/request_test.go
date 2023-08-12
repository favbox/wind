package protocol

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/favbox/wind/common/bytebufferpool"
	"github.com/favbox/wind/common/compress"
	"github.com/favbox/wind/common/config"
	"github.com/favbox/wind/protocol/consts"
	"github.com/stretchr/testify/assert"
)

func TestRequestBodyWriterWrite(t *testing.T) {
	w := requestBodyWriter{&Request{}}
	_, _ = w.Write([]byte("test"))
	assert.Equal(t, "test", string(w.r.body.B))
}

func TestRequestScheme(t *testing.T) {
	req := NewRequest("", "ptth://127.0.0.1:8080", nil)
	assert.Equal(t, "ptth", string(req.Scheme()))
	req = NewRequest("", "127.0.0.1:8080", nil)
	assert.Equal(t, "http", string(req.Scheme()))
	assert.Equal(t, true, req.IsURIParsed())
}

func TestRequestHost(t *testing.T) {
	req := &Request{}
	req.SetHost("127.0.0.1:8080")
	assert.Equal(t, "127.0.0.1:8080", string(req.Host()))
}

func TestRequestSwapBody(t *testing.T) {
	reqA := &Request{}
	reqA.SetBodyRaw([]byte("testA"))
	reqB := &Request{}
	reqB.SetBodyRaw([]byte("testB"))
	SwapRequestBody(reqA, reqB)
	assert.Equal(t, "testA", string(reqB.bodyRaw))
	assert.Equal(t, "testB", string(reqA.bodyRaw))
	reqA.SetBody([]byte("testA"))
	reqB.SetBody([]byte("testB"))
	SwapRequestBody(reqA, reqB)
	assert.Equal(t, "testA", string(reqB.body.B))
	assert.Equal(t, "", string(reqB.bodyRaw))
	assert.Equal(t, "testB", string(reqA.body.B))
	assert.Equal(t, "", string(reqA.bodyRaw))
	reqA.SetBodyStream(strings.NewReader("testA"), len("testA"))
	reqB.SetBodyStream(strings.NewReader("testB"), len("testB"))
	SwapRequestBody(reqA, reqB)
	body := make([]byte, 5)
	_, _ = reqB.bodyStream.Read(body)
	assert.Equal(t, "testA", string(body))
	_, _ = reqA.bodyStream.Read(body)
	assert.Equal(t, "testB", string(body))
}

func TestRequestKnownSizeStreamMultipartFormWithFile(t *testing.T) {
	t.Parallel()

	s := `------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="f1"

value1
------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="fileaaa"; filename="TODO"
Content-Type: application/octet-stream

- SessionClient with referer and cookies support.
- Client with requests' pipelining support.
- ProxyHandler similar to FSHandler.
- WebSockets. See https://tools.ietf.org/html/rfc6455 .
- HTTP/2.0. See https://tools.ietf.org/html/rfc7540 .

------WebKitFormBoundaryJwfATyF8tmxSJnLg--
tailfoobar`
	mr := strings.NewReader(s)
	r := NewRequest("POST", "/upload", mr)
	r.Header.SetContentLength(521)
	r.Header.SetContentTypeBytes([]byte("multipart/form-data; boundary=----WebKitFormBoundaryJwfATyF8tmxSJnLg"))
	assert.Equal(t, false, r.HasMultipartForm())
	f, err := r.MultipartForm()
	assert.Equal(t, true, r.HasMultipartForm())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer r.RemoveMultipartFormFiles()

	// verify tail
	tail, err := io.ReadAll(mr)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(tail) != "tailfoobar" {
		t.Fatalf("unexpected tail %q. Expecting %q", tail, "tailfoobar")
	}

	// verify values
	if len(f.Value) != 1 {
		t.Fatalf("unexpected number of values in multipart form: %d. Expecting 1", len(f.Value))
	}
	for k, vv := range f.Value {
		if k != "f1" {
			t.Fatalf("unexpected value name %q. Expecting %q", k, "f1")
		}
		if len(vv) != 1 {
			t.Fatalf("unexpected number of values %d. Expecting 1", len(vv))
		}
		v := vv[0]
		if v != "value1" {
			t.Fatalf("unexpected value %q. Expecting %q", v, "value1")
		}
	}

	// verify files
	if len(f.File) != 1 {
		t.Fatalf("unexpected number of file values in multipart form: %d. Expecting 1", len(f.File))
	}
	for k, vv := range f.File {
		if k != "fileaaa" {
			t.Fatalf("unexpected file value name %q. Expecting %q", k, "fileaaa")
		}
		if len(vv) != 1 {
			t.Fatalf("unexpected number of file values %d. Expecting 1", len(vv))
		}
		v := vv[0]
		if v.Filename != "TODO" {
			t.Fatalf("unexpected filename %q. Expecting %q", v.Filename, "TODO")
		}
		ct := v.Header.Get("Content-Type")
		if ct != "application/octet-stream" {
			t.Fatalf("unexpected content-type %q. Expecting %q", ct, "application/octet-stream")
		}
	}

	firstFile, err := r.FormFile("fileaaa")
	assert.Equal(t, "TODO", firstFile.Filename)
	assert.Nil(t, err)
}

func TestRequestUnknownSizeStreamMultipartFormWithFile(t *testing.T) {
	t.Parallel()

	s := `------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="f1"

value1
------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="fileaaa"; filename="TODO"
Content-Type: application/octet-stream

- SessionClient with referer and cookies support.
- Client with requests' pipelining support.
- ProxyHandler similar to FSHandler.
- WebSockets. See https://tools.ietf.org/html/rfc6455 .
- HTTP/2.0. See https://tools.ietf.org/html/rfc7540 .

------WebKitFormBoundaryJwfATyF8tmxSJnLg--
tailfoobar`
	mr := strings.NewReader(s)
	r := NewRequest("POST", "/upload", mr)
	r.Header.SetContentLength(-1)
	r.Header.SetContentTypeBytes([]byte("multipart/form-data; boundary=----WebKitFormBoundaryJwfATyF8tmxSJnLg"))

	f, err := r.MultipartForm()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer r.RemoveMultipartFormFiles()

	// 如果内容长度未知，则必须消耗所有数据
	tail, err := io.ReadAll(mr)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(tail) != "" {
		t.Fatalf("unexpected tail %q. Expecting empty string", tail)
	}

	// verify values
	if len(f.Value) != 1 {
		t.Fatalf("unexpected number of values in multipart form: %d. Expecting 1", len(f.Value))
	}
	for k, vv := range f.Value {
		if k != "f1" {
			t.Fatalf("unexpected value name %q. Expecting %q", k, "f1")
		}
		if len(vv) != 1 {
			t.Fatalf("unexpected number of values %d. Expecting 1", len(vv))
		}
		v := vv[0]
		if v != "value1" {
			t.Fatalf("unexpected value %q. Expecting %q", v, "value1")
		}
	}

	// verify files
	if len(f.File) != 1 {
		t.Fatalf("unexpected number of file values in multipart form: %d. Expecting 1", len(f.File))
	}
	for k, vv := range f.File {
		if k != "fileaaa" {
			t.Fatalf("unexpected file value name %q. Expecting %q", k, "fileaaa")
		}
		if len(vv) != 1 {
			t.Fatalf("unexpected number of file values %d. Expecting 1", len(vv))
		}
		v := vv[0]
		if v.Filename != "TODO" {
			t.Fatalf("unexpected filename %q. Expecting %q", v.Filename, "TODO")
		}
		ct := v.Header.Get("Content-Type")
		if ct != "application/octet-stream" {
			t.Fatalf("unexpected content-type %q. Expecting %q", ct, "application/octet-stream")
		}
	}
}

func TestRequestStreamMultipartFormWithFileGzip(t *testing.T) {
	t.Parallel()

	s := `------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="f1"

value1
------WebKitFormBoundaryJwfATyF8tmxSJnLg
Content-Disposition: form-data; name="fileaaa"; filename="TODO"
Content-Type: application/octet-stream

- SessionClient with referer and cookies support.
- Client with requests' pipelining support.
- ProxyHandler similar to FSHandler.
- WebSockets. See https://tools.ietf.org/html/rfc6455 .
- HTTP/2.0. See https://tools.ietf.org/html/rfc7540 .

------WebKitFormBoundaryJwfATyF8tmxSJnLg--
tailfoobar`

	ns := compress.AppendGzipBytes(nil, []byte(s))

	mr := bytes.NewBuffer(ns)
	r := NewRequest("POST", "/upload", mr)
	r.Header.Set("Content-Encoding", "gzip")
	r.Header.SetContentLength(len(s))
	r.Header.SetContentTypeBytes([]byte("multipart/form-data; boundary=----WebKitFormBoundaryJwfATyF8tmxSJnLg"))

	f, err := r.MultipartForm()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer r.RemoveMultipartFormFiles()

	// verify values
	if len(f.Value) != 1 {
		t.Fatalf("unexpected number of values in multipart form: %d. Expecting 1", len(f.Value))
	}
	for k, vv := range f.Value {
		if k != "f1" {
			t.Fatalf("unexpected value name %q. Expecting %q", k, "f1")
		}
		if len(vv) != 1 {
			t.Fatalf("unexpected number of values %d. Expecting 1", len(vv))
		}
		v := vv[0]
		if v != "value1" {
			t.Fatalf("unexpected value %q. Expecting %q", v, "value1")
		}
	}

	// verify files
	if len(f.File) != 1 {
		t.Fatalf("unexpected number of file values in multipart form: %d. Expecting 1", len(f.File))
	}
	for k, vv := range f.File {
		if k != "fileaaa" {
			t.Fatalf("unexpected file value name %q. Expecting %q", k, "fileaaa")
		}
		if len(vv) != 1 {
			t.Fatalf("unexpected number of file values %d. Expecting 1", len(vv))
		}
		v := vv[0]
		if v.Filename != "TODO" {
			t.Fatalf("unexpected filename %q. Expecting %q", v.Filename, "TODO")
		}
		ct := v.Header.Get("Content-Type")
		if ct != "application/octet-stream" {
			t.Fatalf("unexpected content-type %q. Expecting %q", ct, "application/octet-stream")
		}
	}
}

func TestRequestMultipartFormBoundary(t *testing.T) {
	r := &Request{}
	r.SetMultipartFormBoundary("----boundary----")
	assert.Equal(t, "----boundary----", r.MultipartFormBoundary())
}

func TestRequestSetQueryString(t *testing.T) {
	r := &Request{}
	r.SetQueryString("test")
	assert.Equal(t, "test", string(r.URI().queryString))
}

func TestRequestSetFormData(t *testing.T) {
	r := &Request{}
	data := map[string]string{"username": "admin"}
	r.SetFormData(data)
	assert.Equal(t, "username", string(r.postArgs.args[0].key))
	assert.Equal(t, "admin", string(r.postArgs.args[0].value))
	assert.Equal(t, true, r.parsedPostArgs)
	assert.Equal(t, consts.MIMEApplicationHTMLForm, string(r.Header.contentType))

	r = &Request{}
	value := map[string][]string{"item": {"apple", "peach"}}
	r.SetFormDataFromValues(value)
	assert.Equal(t, "item", string(r.postArgs.args[0].key))
	assert.Equal(t, "apple", string(r.postArgs.args[0].value))
	assert.Equal(t, "item", string(r.postArgs.args[1].key))
	assert.Equal(t, "peach", string(r.postArgs.args[1].value))
}

func TestRequestSetFile(t *testing.T) {
	r := &Request{}
	r.SetFile("file", "/usr/bin/test.txt")
	assert.Equal(t, &File{"/usr/bin/test.txt", "file", nil}, r.multipartFiles[0])

	files := map[string]string{"f1": "/usr/bin/test1.txt"}
	r.SetFiles(files)
	assert.Equal(t, &File{"/usr/bin/test1.txt", "f1", nil}, r.multipartFiles[1])

	assert.Equal(t, []*File{{"/usr/bin/test.txt", "file", nil}, {"/usr/bin/test1.txt", "f1", nil}}, r.MultipartFiles())
}

func TestRequestSetFileReader(t *testing.T) {
	r := &Request{}
	r.SetFileReader("file", "/usr/bin/test.txt", nil)
	assert.Equal(t, &File{"/usr/bin/test.txt", "file", nil}, r.multipartFiles[0])
}

func TestRequestSetMultipartFormData(t *testing.T) {
	r := &Request{}
	data := map[string]string{"item": "apple"}
	r.SetMultipartFormData(data)
	assert.Equal(t, &MultipartField{"item", "", "", strings.NewReader("apple")}, r.multipartFields[0])

	r = &Request{}
	fields := []*MultipartField{{"item2", "", "", strings.NewReader("apple2")}, {"item3", "", "", strings.NewReader("apple3")}}
	r.SetMultipartFields(fields...)
	assert.Equal(t, fields, r.MultipartFields())
}

func TestRequestSetBasicAuth(t *testing.T) {
	r := &Request{}
	r.SetBasicAuth("admin", "admin")
	assert.Equal(t, "Authorization", string(r.Header.h[0].key))
	assert.Equal(t, "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:admin")), string(r.Header.h[0].value))
}

func TestRequestSetAuthToken(t *testing.T) {
	r := &Request{}
	r.SetAuthToken("token")
	assert.Equal(t, "Authorization", string(r.Header.h[0].key))
	assert.Equal(t, "Bearer token", string(r.Header.h[0].value))

	r = &Request{}
	r.SetAuthSchemeToken("http", "token")
	assert.Equal(t, "Authorization", string(r.Header.h[0].key))
	assert.Equal(t, "http token", string(r.Header.h[0].value))
}

func TestRequestSetHeaders(t *testing.T) {
	r := &Request{}
	headers := map[string]string{"Key1": "value1"}
	r.SetHeaders(headers)
	assert.Equal(t, "Key1", string(r.Header.h[0].key))
	assert.Equal(t, "value1", string(r.Header.h[0].value))
}

func TestRequestSetCookie(t *testing.T) {
	r := &Request{}
	r.SetCookie("cookie1", "cookie1")
	assert.Equal(t, "cookie1", string(r.Header.cookies[0].key))
	assert.Equal(t, "cookie1", string(r.Header.cookies[0].value))

	r.SetCookies(map[string]string{"cookie2": "cookie2"})
	assert.Equal(t, "cookie2", string(r.Header.cookies[1].key))
	assert.Equal(t, "cookie2", string(r.Header.cookies[1].value))
}

func TestRequestPath(t *testing.T) {
	r := NewRequest("POST", "/upload?test", nil)
	assert.Equal(t, "/upload", string(r.Path()))
	assert.Equal(t, "test", string(r.QueryString()))
}

func TestRequestConnectionClose(t *testing.T) {
	r := NewRequest("POST", "/upload?test", nil)
	assert.Equal(t, false, r.ConnectionClose())
	r.SetConnectionClose()
	assert.Equal(t, true, r.ConnectionClose())
}

func TestRequestBodyWriteToPlain(t *testing.T) {
	t.Parallel()

	var r Request

	expectedS := "foobarbaz"
	r.AppendBodyString(expectedS)

	testBodyWriteTo(t, &r, expectedS, true)
}

func TestRequestBodyWriteToMultipart(t *testing.T) {
	t.Parallel()

	expectedS := "--foobar\r\nContent-Disposition: form-data; name=\"key_0\"\r\n\r\nvalue_0\r\n--foobar--\r\n"

	var r Request
	SetMultipartFormWithBoundary(&r, &multipart.Form{Value: map[string][]string{"key_0": {"value_0"}}}, "foobar")

	testBodyWriteTo(t, &r, expectedS, true)
}

func TestNewRequest(t *testing.T) {
	// get
	req := NewRequest("GET", "http://www.google.com/hi", bytes.NewReader([]byte("hello")))
	assert.NotNil(t, req)
	assert.Equal(t, "GET /hi HTTP/1.1\r\nHost: www.google.com\r\n\r\n", string(req.Header.Header()))
	assert.Nil(t, req.Body())

	// post + bytes reader
	req = NewRequest("POST", "http://www.google.com/hi", bytes.NewReader([]byte("hello")))
	assert.NotNil(t, req)
	assert.Equal(t, "POST /hi HTTP/1.1\r\nHost: www.google.com\r\nContent-Type: application/x-www-form-urlencoded\r\nContent-Length: 5\r\n\r\n", string(req.Header.Header()))
	assert.Equal(t, "hello", string(req.Body()))

	// post + string reader
	req = NewRequest("POST", "http://www.google.com/hi", strings.NewReader("hello world"))
	assert.NotNil(t, req)
	assert.Equal(t, "POST /hi HTTP/1.1\r\nHost: www.google.com\r\nContent-Type: application/x-www-form-urlencoded\r\nContent-Length: 11\r\n\r\n", string(req.Header.Header()))
	assert.Equal(t, "hello world", string(req.Body()))

	// post + bytes buffer
	req = NewRequest("POST", "http://www.google.com/hi", bytes.NewBuffer([]byte("hello wind!")))
	assert.NotNil(t, req)
	assert.Equal(t, "POST /hi HTTP/1.1\r\nHost: www.google.com\r\nContent-Type: application/x-www-form-urlencoded\r\nContent-Length: 11\r\n\r\n", string(req.Header.Header()))
	assert.Equal(t, "hello wind!", string(req.Body()))

	// empty method
	req = NewRequest("", "/", bytes.NewBufferString(""))
	assert.Equal(t, "GET", string(req.Method()))
	// unstandard method
	req = NewRequest("DUMMY", "/", bytes.NewBufferString(""))
	assert.Equal(t, "DUMMY", string(req.Method()))

	// empty body
	req = NewRequest("GET", "/", nil)
	assert.NotNil(t, req)
	// wrong body
	req = NewRequest("POST", "/", errorReader{})
	_, err := req.BodyE()
	assert.Equal(t, "dummy", err.Error())
	req = NewRequest("POST", "/", errorReader{})
	body := req.Body()
	assert.Nil(t, body)

	// GET RequestURI
	req = NewRequest("GET", "http://www.google.com/hi?a=1&b=2", nil)
	assert.Equal(t, "/hi?a=1&b=2", string(req.RequestURI()))

	// POST RequestURI
	req = NewRequest("POST", "http://www.google.com/hi?a=1&b=2", nil)
	assert.Equal(t, "/hi?a=1&b=2", string(req.RequestURI()))

	// nil-interface body
	assert.Panics(t, func() {
		fake := func() *errorReader {
			return nil
		}
		req = NewRequest("POST", "/", fake())
		req.Body()
	})
}

func TestRequestResetBody(t *testing.T) {
	req := Request{}
	req.BodyBuffer()
	assert.NotNil(t, req.body)
	req.maxKeepBodySize = math.MaxUint32
	req.ResetBody()
	assert.NotNil(t, req.body)
	req.maxKeepBodySize = -1
	req.ResetBody()
	assert.Nil(t, req.body)
}

func TestRequestConstructBodyStream(t *testing.T) {
	r := &Request{}
	b := []byte("test")
	r.ConstructBodyStream(&bytebufferpool.ByteBuffer{B: b}, strings.NewReader("test"))
	assert.Equal(t, "test", string(r.body.B))
	stream := make([]byte, 4)
	_, _ = r.bodyStream.Read(stream)
	assert.Equal(t, "test", string(stream))
}

func TestRequestPostArgs(t *testing.T) {
	t.Parallel()

	s := `username=admin&password=admin`
	mr := strings.NewReader(s)
	r := &Request{}
	r.SetBodyStream(mr, len(s))
	r.Header.contentType = []byte(consts.MIMEApplicationHTMLForm)
	arg := r.PostArgs()
	assert.Equal(t, "username", string(arg.args[0].key))
	assert.Equal(t, "admin", string(arg.args[0].value))
	assert.Equal(t, "password", string(arg.args[1].key))
	assert.Equal(t, "admin", string(arg.args[1].value))
	assert.Equal(t, "username=admin&password=admin", string(r.PostArgString()))
}

func TestRequestMayContinue(t *testing.T) {
	t.Parallel()

	var r Request
	if r.MayContinue() {
		t.Fatalf("MayContinue on empty request must return false")
	}

	r.Header.Set("Expect", "123sdfds")
	if r.MayContinue() {
		t.Fatalf("MayContinue on invalid Expect header must return false")
	}

	r.Header.Set("Expect", "100-continue")
	if !r.MayContinue() {
		t.Fatalf("MayContinue on 'Expect: 100-continue' header must return true")
	}
}

func TestRequestSwapBodySerial(t *testing.T) {
	t.Parallel()

	testRequestSwapBody(t)
}

// Test case for testing BasicAuth
var BasicAuthTests = []struct {
	header, username, password string
	ok                         bool
}{
	{"Basic " + base64.StdEncoding.EncodeToString([]byte("Aladdin:open sesame")), "Aladdin", "open sesame", true},

	// Case doesn't matter:
	{"BASIC " + base64.StdEncoding.EncodeToString([]byte("Aladdin:open sesame")), "Aladdin", "open sesame", true},
	{"basic " + base64.StdEncoding.EncodeToString([]byte("Aladdin:open sesame")), "Aladdin", "open sesame", true},

	{"Basic " + base64.StdEncoding.EncodeToString([]byte("Aladdin:open:sesame")), "Aladdin", "open:sesame", true},
	{"Basic " + base64.StdEncoding.EncodeToString([]byte(":")), "", "", true},
	{"Basic" + base64.StdEncoding.EncodeToString([]byte("Aladdin:open sesame")), "", "", false},
	{base64.StdEncoding.EncodeToString([]byte("Aladdin:open sesame")), "", "", false},
	{"Basic ", "", "", false},
	{"Basic Aladdin:open sesame", "", "", false},
	{`Digest username="Aladdin"`, "", "", false},
}

// struct for
type getBasicAuthTest struct {
	username, password string
	ok                 bool
}

func TestRequestBasicAuth(t *testing.T) {
	for _, tt := range BasicAuthTests {
		req := NewRequest("GET", "http://www.google.com/hi", bytes.NewReader([]byte("hello")))
		req.SetHeader("Authorization", tt.header)
		username, password, ok := req.BasicAuth()
		if ok != tt.ok || username != tt.username || password != tt.password {
			t.Fatalf("BasicAuth() = %+v, want %+v", getBasicAuthTest{username, password, ok},
				getBasicAuthTest{tt.username, tt.password, tt.ok})
		}
	}
}

// 问题：NewRequest应该创建一个不使用输入参数作为其结构的Request，
// 否则当我们将const字符串作为方法传递给NewRequest并调用req.SetMethod（）时，它会引起恐慌
func TestNewRequestWithConstParam(t *testing.T) {
	const method = "POST"
	const uri = "http://www.google.com/hi"
	req := NewRequest(method, uri, nil)
	req.SetMethod("POST")
	req.SetRequestURI("http://www.google.com/hi")
}

func TestRequestCopyToWithOptions(t *testing.T) {
	req := AcquireRequest()
	k1 := "a"
	v1 := "A"
	k2 := "b"
	v2 := "B"
	req.SetOptions(config.WithTag(k1, v1), config.WithTag(k2, v2), config.WithSD(true))
	reqCopy := AcquireRequest()
	req.CopyTo(reqCopy)
	assert.Equal(t, v1, reqCopy.options.Tag(k1))
	assert.Equal(t, v2, reqCopy.options.Tag(k2))
	assert.Equal(t, true, reqCopy.options.IsSD())
}

func TestRequestSetMaxKeepBodySize(t *testing.T) {
	r := &Request{}
	r.SetMaxKeepBodySize(1024)
	assert.Equal(t, 1024, r.maxKeepBodySize)
}

func TestRequestGetBodyAfterGetBodyStream(t *testing.T) {
	req := AcquireRequest()
	req.SetBodyString("abc")
	req.BodyStream()
	assert.Equal(t, req.Body(), []byte("abc"))
}

func TestRequestSetOptionsNotOverwrite(t *testing.T) {
	req := AcquireRequest()
	req.SetOptions(config.WithSD(true))
	req.SetOptions(config.WithTag("a", "b"))
	req.SetOptions(config.WithTag("c", "d"))
	assert.Equal(t, true, req.Options().IsSD())
	assert.Equal(t, "b", req.Options().Tag("a"))
	assert.Equal(t, "d", req.Options().Tag("c"))

	req.SetOptions(config.WithTag("a", "c"))
	assert.Equal(t, "c", req.Options().Tag("a"))
}

func TestReqSafeCopy(t *testing.T) {
	req := AcquireRequest()
	req.bodyRaw = make([]byte, 1)
	reqs := make([]*Request, 10)
	for i := 0; i < 10; i++ {
		req.bodyRaw[0] = byte(i)
		tmpReq := AcquireRequest()
		req.CopyTo(tmpReq)
		reqs[i] = tmpReq
	}
	for i := 0; i < 10; i++ {
		assert.Equal(t, []byte{byte(i)}, reqs[i].Body())
	}
}

func testRequestSwapBody(t *testing.T) {
	var b []byte
	r := &Request{}
	for i := 0; i < 20; i++ {
		bOrig := r.Body()
		b = r.SwapBody(b)
		if !bytes.Equal(bOrig, b) {
			t.Fatalf("unexpected body returned: %q. Expecting %q", b, bOrig)
		}
		r.AppendBodyString("foobar")
	}

	s := "aaaabbbbcccc"
	b = b[:0]
	for i := 0; i < 10; i++ {
		r.SetBodyStream(bytes.NewBufferString(s), len(s))
		b = r.SwapBody(b)
		if string(b) != s {
			t.Fatalf("unexpected body returned: %q. Expecting %q", b, s)
		}
		b = r.SwapBody(b)
		if len(b) > 0 {
			t.Fatalf("unexpected body with non-zero size returned: %q", b)
		}
	}
}

type bodyWriterTo interface {
	BodyWriteTo(writer io.Writer) error
	Body() []byte
}

func testBodyWriteTo(t *testing.T, bw bodyWriterTo, expectedS string, isRetainedBody bool) {
	var buf bytebufferpool.ByteBuffer
	if err := bw.BodyWriteTo(&buf); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	s := buf.B
	if string(s) != expectedS {
		t.Fatalf("unexpected result %q. Expecting %q", s, expectedS)
	}

	body := bw.Body()
	if isRetainedBody {
		if string(body) != expectedS {
			t.Fatalf("unexpected body %q. Expecting %q", body, expectedS)
		}
	} else {
		if len(body) > 0 {
			t.Fatalf("unexpected non-zero body after BodyWriteTo: %q", body)
		}
	}
}

type errorReader struct{}

func (er errorReader) Read(_ []byte) (int, error) {
	return 0, fmt.Errorf("dummy")
}
