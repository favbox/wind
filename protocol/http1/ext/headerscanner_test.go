package ext

import (
	"bufio"
	"errors"
	"net/http"
	"strings"
	"testing"

	errs "github.com/favbox/gosky/wind/pkg/common/errors"
	"github.com/stretchr/testify/assert"
)

func TestHasHeaderValue(t *testing.T) {
	s := []byte("Expect: 100-continue, User-Agent: foo, Host: 127.0.0.1, Connection: Keep-Alive, Content-Length: 5")
	assert.True(t, HasHeaderValue(s, []byte("Connection: Keep-Alive")))
	assert.False(t, HasHeaderValue(s, []byte("Connection: Keep-Alive1")))
}

func TestResponseHeaderMultiLineValue(t *testing.T) {
	firstLine := "HTTP/1.1 200 OK\r\n"
	rawHeaders := "EmptyValue1:\r\n" +
		"Content-Type: foo/bar;\r\n\tnewline;\r\n another/newline\r\n" +
		"Foo: Bar\r\n" +
		"Multi-Line: one;\r\n two\r\n" +
		"Values: v1;\r\n v2; v3;\r\n v4;\tv5\r\n" +
		"\r\n"

	// 与http响应相比
	response, err := http.ReadResponse(bufio.NewReader(strings.NewReader(firstLine+rawHeaders)), nil)
	assert.Nil(t, err)
	defer func() { response.Body.Close() }()

	hs := &HeaderScanner{}
	hs.B = []byte(rawHeaders)
	hs.DisableNormalizing = false
	hmap := make(map[string]string, len(response.Header))
	for hs.Next() {
		if len(hs.Key) > 0 {
			hmap[string(hs.Key)] = string(hs.Value)
		}
	}

	for name, vals := range response.Header {
		got := hmap[name]
		want := vals[0]
		assert.Equal(t, want, got)
	}
}

func TestHeaderScannerError(t *testing.T) {
	t.Run("TestHeaderScannerErrorInvalidName", func(t *testing.T) {
		rawHeaders := "Host: go.dev\r\nGopher-New-\r\n Line: This is a header on multiple lines\r\n\r\n"
		testTestHeaderScannerError(t, rawHeaders, errInvalidName)
	})
	t.Run("TestHeaderScannerErrorNeedMore", func(t *testing.T) {
		rawHeaders := "This is a header on multiple lines"
		testTestHeaderScannerError(t, rawHeaders, errs.ErrNeedMore)

		rawHeaders = "Gopher-New-\r\n Line"
		testTestHeaderScannerError(t, rawHeaders, errs.ErrNeedMore)
	})
}

func testTestHeaderScannerError(t *testing.T, rawHeaders string, expectError error) {
	hs := &HeaderScanner{}
	hs.B = []byte(rawHeaders)
	hs.DisableNormalizing = false
	for hs.Next() {
	}
	assert.NotNil(t, hs.Err)
	assert.True(t, errors.Is(hs.Err, expectError))
}
