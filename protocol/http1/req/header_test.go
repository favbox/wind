package req

import (
	"testing"

	"github.com/favbox/gosky/wind/pkg/common/test/assert"
	"github.com/favbox/gosky/wind/pkg/common/test/mock"
	"github.com/favbox/gosky/wind/pkg/protocol"
	"github.com/favbox/gosky/wind/pkg/protocol/consts"
)

func TestRequestHeader_Read(t *testing.T) {
	s := "PUT /foo/bar HTTP/1.1\r\nExpect: 100-continue\r\nUser-Agent: foo\r\nHost: 127.0.0.1\r\nConnection: Keep-Alive\r\nContent-Length: 5\r\nContent-Type: foo/bar\r\n\r\nabcdef4343"
	zr := mock.NewZeroCopyReader(s)
	rh := protocol.RequestHeader{}
	ReadHeader(&rh, zr)

	// firstline
	assert.DeepEqual(t, []byte(consts.MethodPut), rh.Method())
	assert.DeepEqual(t, []byte("/foo/bar"), rh.RequestURI())
	assert.True(t, rh.IsHTTP11())

	// headers
	assert.DeepEqual(t, 5, rh.ContentLength())
	assert.DeepEqual(t, []byte("foo/bar"), rh.ContentType())
	count := 0
	rh.VisitAll(func(key, value []byte) {
		count += 1
	})
	assert.DeepEqual(t, 6, count)
	assert.DeepEqual(t, []byte("foo"), rh.UserAgent())
	assert.DeepEqual(t, []byte("127.0.0.1"), rh.Host())
	assert.DeepEqual(t, []byte("100-continue"), rh.Peek("Expect"))
}
