package ext

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/cloudwego/netpoll"
	errs "github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/common/mock"
	"github.com/favbox/wind/protocol"
	"github.com/stretchr/testify/assert"
)

func Test_stripSpace(t *testing.T) {
	a := stripSpace([]byte("     a"))
	b := stripSpace([]byte("b       "))
	c := stripSpace([]byte("    c     "))
	assert.Equal(t, []byte("a"), a)
	assert.Equal(t, []byte("b"), b)
	assert.Equal(t, []byte("c"), c)
}

func Test_bufferSnippet(t *testing.T) {
	a := make([]byte, 39)
	b := make([]byte, 41)
	assert.False(t, strings.Contains(BufferSnippet(a), "\"...\""))
	assert.True(t, strings.Contains(BufferSnippet(b), "\"...\""))
}

func Test_isOnlyCRLF(t *testing.T) {
	assert.True(t, isOnlyCRLF([]byte("\r\n")))
	assert.True(t, isOnlyCRLF([]byte("\n")))
}

func TestReadTrailer(t *testing.T) {
	exceptedTrailers := map[string]string{"Wind": "test"}
	zr := mock.NewZeroCopyReader("0\r\nWind: test\r\n\r\n")
	trailer := protocol.Trailer{}
	keys := make([]string, 0, len(exceptedTrailers))
	for k := range exceptedTrailers {
		keys = append(keys, k)
	}
	trailer.SetTrailers([]byte(strings.Join(keys, ", ")))
	err := ReadTrailer(&trailer, zr)
	if err != nil {
		t.Fatalf("Cannot read trailer: %v", err)
	}

	for k, v := range exceptedTrailers {
		got := trailer.Peek(k)
		if !bytes.Equal(got, []byte(v)) {
			t.Fatalf("Unexpected trailer %q. Expected %q. Got %q", k, v, got)
		}
	}
}

func TestReadTrailerError(t *testing.T) {
	// with bad trailer
	zr := mock.NewZeroCopyReader("0\r\nWind: test\r\nContent-Type: aaa\r\n\r\n")
	trailer := protocol.Trailer{}
	err := ReadTrailer(&trailer, zr)
	if err == nil {
		t.Fatalf("expecting error.")
	}

	// eof
	er := mock.EOFReader{}
	trailer = protocol.Trailer{}
	err = ReadTrailer(&trailer, &er)
	assert.Equal(t, io.EOF, err)
}

func TestReadTrailer1(t *testing.T) {
	exceptedTrailers := map[string]string{}
	zr := mock.NewZeroCopyReader("0\r\n\r\n")
	trailer := protocol.Trailer{}
	err := ReadTrailer(&trailer, zr)
	if err != nil {
		t.Fatalf("Cannot read trailer: %v", err)
	}

	for k, v := range exceptedTrailers {
		got := trailer.Peek(k)
		if !bytes.Equal(got, []byte(v)) {
			t.Fatalf("Unexpected trailer %q. Expected %q. Got %q", k, v, got)
		}
	}
}

func TestReadRawHeaders(t *testing.T) {
	s := "HTTP/1.1 200 OK\r\n" +
		"EmptyValue1:\r\n" +
		"Content-Type: foo/bar;\r\n\tnewline;\r\n another/newline\r\n" +
		"Foo: Bar\r\n" +
		"Multi-Line: one;\r\n two\r\n" +
		"Values: v1;\r\n v2; v3;\r\n v4;\tv5\r\n" +
		"Content-Length: 5\r\n\r\n" +
		"HELLOaaa"

	var dst []byte
	rawHeaders, index, err := ReadRawHeaders(dst, []byte(s))
	assert.Nil(t, err)
	assert.Equal(t, s[:index], string(rawHeaders))
}

func TestBodyChunked(t *testing.T) {
	body := "foobar baz aaa bbb ccc"
	chunk := "16\r\nfoobar baz aaa bbb ccc\r\n0\r\n"
	b := bytes.NewBufferString(body)

	var w bytes.Buffer
	zw := netpoll.NewWriter(&w)
	WriteBodyChunked(zw, b)

	assert.Equal(t, chunk, w.String())

	zr := mock.NewZeroCopyReader(chunk)
	rb, err := ReadBody(zr, -1, 0, nil)
	assert.Nil(t, err)
	assert.Equal(t, body, string(rb))
}

func TestBodyFixedSize(t *testing.T) {
	body := mock.CreateFixedBody(10)
	b := bytes.NewBuffer(body)

	var w bytes.Buffer
	zw := netpoll.NewWriter(&w)
	WriteBodyFixedSize(zw, b, int64(len(body)))

	assert.Equal(t, body, w.Bytes())

	zr := mock.NewZeroCopyReader(string(body))
	rb, err := ReadBody(zr, len(body), 0, nil)
	assert.Nil(t, err)
	assert.Equal(t, body, rb)
}

func TestBodyFixedSizeQuickPath(t *testing.T) {
	conn := mock.NewBrokenConn("")
	err := WriteBodyFixedSize(conn.Writer(), conn, 0)
	assert.Nil(t, err)
}

func TestBodyIdentity(t *testing.T) {
	body := mock.CreateFixedBody(1024)
	zr := mock.NewZeroCopyReader(string(body))
	rb, err := ReadBody(zr, -2, 0, nil)
	assert.Nil(t, err)
	assert.Equal(t, string(body), string(rb))
}

func TestBodySkipTrailer(t *testing.T) {
	t.Run("TestBodySkipTrailer", func(t *testing.T) {
		body := mock.CreateFixedBody(10)
		trailer := map[string]string{"Foo": "chunked shit"}
		chunkedBody := mock.CreateChunkedBody(body, trailer, true)
		r := mock.NewSlowReadConn(string(chunkedBody))
		err := SkipTrailer(r)
		assert.Nil(t, err)
		_, err = r.ReadByte()
		assert.NotNil(t, err)
		assert.True(t, errors.Is(err, netpoll.ErrEOF))
	})

	t.Run("TestBodySkipTrailerError", func(t *testing.T) {
		//  timeout error
		sr := mock.NewSlowReadConn("")
		err := SkipTrailer(sr)
		assert.NotNil(t, err)
		assert.True(t, errors.Is(err, errs.ErrTimeout))
		//  EOF error
		er := &mock.EOFReader{}
		err = SkipTrailer(er)
		assert.NotNil(t, err)
		assert.True(t, errors.Is(err, io.EOF))
	})
}