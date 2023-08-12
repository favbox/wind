package app

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/favbox/wind/common/mock"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/consts"
	"github.com/favbox/wind/protocol/http1/resp"
	"github.com/stretchr/testify/assert"
)

func TestNewVHostPathRewriter(t *testing.T) {
	t.Parallel()

	var ctx RequestContext
	var req protocol.Request
	req.Header.SetHost("foobar.com")
	req.SetRequestURI("/foo/bar/baz")
	req.CopyTo(&ctx.Request)

	f := NewVHostPathRewriter(0)
	path := f(&ctx)
	assert.Equal(t, "/foobar.com/foo/bar/baz", string(path))

	ctx.Request.Reset()
	ctx.Request.SetRequestURI("https://aaa.bbb.cc/one/two/three/four?asdf=dsf")
	f = NewVHostPathRewriter(2)
	path = f(&ctx)
	assert.Equal(t, "/aaa.bbb.cc/three/four", string(path))
}

func TestNewVHostPathRewriterMaliciousHost(t *testing.T) {
	var ctx RequestContext
	var req protocol.Request
	req.Header.SetHost("/../../../etc/passwd")
	req.SetRequestURI("/foo/bar/baz")
	req.CopyTo(&ctx.Request)

	f := NewVHostPathRewriter(0)
	path := f(&ctx)
	expectedPath := "/invalid-host/foo/bar/baz"
	if string(path) != expectedPath {
		t.Fatalf("unexpected path %q. Expecting %q", path, expectedPath)
	}
}

func testPathNotFound(t *testing.T, pathNotFoundFunc HandlerFunc) {
	var ctx RequestContext
	var req protocol.Request
	req.SetRequestURI("http//some.url/file")
	req.CopyTo(&ctx.Request)

	fs := &FS{
		Root:         "./",
		PathNotFound: pathNotFoundFunc,
	}
	fs.NewRequestHandler()(context.Background(), &ctx)

	if pathNotFoundFunc == nil {
		// different to ...
		if !bytes.Equal(ctx.Response.Body(),
			[]byte("无法打开请求的路径")) {
			t.Fatalf("response defers. Response: %q", ctx.Response.Body())
		}
	} else {
		// Equals to ...
		if bytes.Equal(ctx.Response.Body(),
			[]byte("无法打开请求的路径")) {
			t.Fatalf("response defers. Response: %q", ctx.Response.Body())
		}
	}
}

func TestPathNotFound(t *testing.T) {
	t.Parallel()

	testPathNotFound(t, nil)
}

func TestPathNotFoundFunc(t *testing.T) {
	t.Parallel()

	testPathNotFound(t, func(c context.Context, ctx *RequestContext) {
		ctx.WriteString("没找到 呵呵")
	})
}

func TestServeFileHead(t *testing.T) {
	t.Parallel()

	var ctx RequestContext
	var req protocol.Request
	req.Header.SetMethod(consts.MethodHead)
	req.SetRequestURI("http://foobar.com/baz")
	req.CopyTo(&ctx.Request)

	ServeFile(&ctx, "fs.go")

	var r protocol.Response
	r.SkipBody = true
	s := resp.GetHTTP1Response(&ctx.Response).String()
	zr := mock.NewZeroCopyReader(s)
	if err := resp.Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	ce := r.Header.ContentEncoding()
	if len(ce) > 0 {
		t.Fatalf("Unexpected 'Content-Encoding' %q", ce)
	}

	body := r.Body()
	if len(body) > 0 {
		t.Fatalf("unexpected response body %q. Expecting empty body", body)
	}

	expectedBody, err := getFileContents("/fs.go")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	contentLength := r.Header.ContentLength()
	if contentLength != len(expectedBody) {
		t.Fatalf("unexpected Content-Length: %d. expecting %d", contentLength, len(expectedBody))
	}
}

func getFileContents(path string) ([]byte, error) {
	path = "." + path
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ioutil.ReadAll(f)
}
