package req

import (
	"strings"
	"testing"

	"github.com/favbox/gosky/wind/pkg/common/test/mock"
	"github.com/favbox/gosky/wind/pkg/protocol"
	"github.com/favbox/gosky/wind/pkg/protocol/consts"
)

func TestRequestContinueReadBody(t *testing.T) {
	t.Parallel()
	s := "PUT /foo/bar HTTP/1.1\r\nExpect: 100-continue\r\nContent-Length: 5\r\nContent-Type: foo/bar\r\n\r\nabcdef4343"
	zr := mock.NewZeroCopyReader(s)

	var r protocol.Request
	if err := Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if err := ContinueReadBody(&r, zr, 0, true); err != nil {
		t.Fatalf("error when reading request body: %s", err)
	}
	body := r.Body()
	if string(body) != "abcde" {
		t.Fatalf("unexpected body %q. Expecting %q", body, "abcde")
	}

	tail, err := zr.Peek(zr.Len())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(tail) != "f4343" {
		t.Fatalf("unexpected tail %q. Expecting %q", tail, "f4343")
	}
}

func TestRequestReadNoBody(t *testing.T) {
	t.Parallel()

	var r protocol.Request

	s := "GET / HTTP/1.1\r\n\r\n"

	zr := mock.NewZeroCopyReader(s)
	if err := Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	r.SetHost("foobar")
	headerStr := r.Header.String()
	if strings.Contains(headerStr, "Content-Length: ") {
		t.Fatalf("unexpected Content-Length")
	}
}

func TestRequestRead(t *testing.T) {
	t.Parallel()

	var r protocol.Request

	s := "POST / HTTP/1.1\r\n\r\n"

	zr := mock.NewZeroCopyReader(s)
	if err := Read(&r, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	r.SetHost("foobar")
	headerStr := r.Header.String()
	if !strings.Contains(headerStr, "Content-Length: ") {
		t.Fatalf("should contain Content-Length")
	}
	cLen := r.Header.Peek(consts.HeaderContentLength)
	if string(cLen) != "0" {
		t.Fatalf("unexpected Content-Length: %s, Expecting 0", string(cLen))
	}
}

func TestRequestReadNoBodyStreaming(t *testing.T) {
	t.Parallel()

	var r protocol.Request
	r.Header.SetContentLength(-2)
	r.Header.SetMethod("GET")

	s := ""

	zr := mock.NewZeroCopyReader(s)
	if err := ContinueReadBodyStream(&r, zr, 2048, true); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	r.SetHost("foobar")
	headerStr := r.Header.String()
	if strings.Contains(headerStr, "Content-Length: ") {
		t.Fatalf("unexpected Content-Length")
	}
}
