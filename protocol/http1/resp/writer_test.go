package resp

import (
	"testing"

	"github.com/favbox/wind/common/mock"
	"github.com/favbox/wind/internal/bytestr"
	"github.com/favbox/wind/protocol"
	"github.com/stretchr/testify/assert"
)

func TestNewChunkedBodyWriter(t *testing.T) {
	resp := protocol.AcquireResponse()
	mockConn := mock.NewConn("")
	w := NewChunkedBodyWriter(resp, mockConn)
	w.Write([]byte("hello"))
	w.Flush()

	out, _ := mockConn.WriterRecorder().ReadBinary(mockConn.WriterRecorder().WroteLen())
	assert.Contains(t, string(out), "Transfer-Encoding: chunked")
	assert.Contains(t, string(out), "5"+string(bytestr.StrCRLF)+"hello")
	assert.NotContains(t, string(out), "0"+string(bytestr.StrCRLF)+string(bytestr.StrCRLF))
}

func TestNewChunkedBodyWriter1(t *testing.T) {
	resp := protocol.AcquireResponse()
	mockConn := mock.NewConn("")
	w := NewChunkedBodyWriter(resp, mockConn)
	w.Write([]byte("hello"))
	w.Flush()
	w.Finalize()
	w.Flush()

	out, _ := mockConn.WriterRecorder().ReadBinary(mockConn.WriterRecorder().WroteLen())
	assert.Contains(t, string(out), "Transfer-Encoding: chunked")
	assert.Contains(t, string(out), "5"+string(bytestr.StrCRLF)+"hello")
	assert.Contains(t, string(out), "0"+string(bytestr.StrCRLF)+string(bytestr.StrCRLF))
}

func TestNewChunkedBodyWriterNoData(t *testing.T) {
	resp := protocol.AcquireResponse()
	resp.Header.Set("Foo", "Bar")
	mockConn := mock.NewConn("")
	w := NewChunkedBodyWriter(resp, mockConn)
	w.Finalize()
	w.Flush()
	out, _ := mockConn.WriterRecorder().ReadBinary(mockConn.WriterRecorder().WroteLen())
	assert.Contains(t, string(out), "Transfer-Encoding: chunked")
	assert.Contains(t, string(out), "Foo: Bar")
	assert.Contains(t, string(out), "0"+string(bytestr.StrCRLF)+string(bytestr.StrCRLF))
}
