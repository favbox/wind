package req

import (
	"testing"

	"github.com/favbox/wind/common/mock"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/consts"
	"github.com/stretchr/testify/assert"
)

func TestRequestHeader_Read(t *testing.T) {
	s := "PUT /foo/bar HTTP/1.1\r\nExpect: 100-continue\r\nUser-Agent: foo\r\nHost: 127.0.0.1\r\nConnection: Keep-Alive\r\nContent-Length: 5\r\nContent-Type: foo/bar\r\n\r\nabcdef4343"
	zr := mock.NewZeroCopyReader(s)
	rh := protocol.RequestHeader{}
	ReadHeader(&rh, zr)

	// firstline
	assert.Equal(t, []byte(consts.MethodPut), rh.Method())
	assert.Equal(t, []byte("/foo/bar"), rh.RequestURI())
	assert.True(t, rh.IsHTTP11())

	// headers
	assert.Equal(t, 5, rh.ContentLength())
	assert.Equal(t, []byte("foo/bar"), rh.ContentType())
	count := 0
	rh.VisitAll(func(key, value []byte) {
		count += 1
	})
	assert.Equal(t, 6, count)
	assert.Equal(t, []byte("foo"), rh.UserAgent())
	assert.Equal(t, []byte("127.0.0.1"), rh.Host())
	assert.Equal(t, []byte("100-continue"), rh.Peek("Expect"))
}

// TODO 补全测试
