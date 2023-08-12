package ext

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/favbox/wind/common/bytebufferpool"
	"github.com/favbox/wind/common/mock"
	"github.com/favbox/wind/protocol"
	"github.com/stretchr/testify/assert"
)

func createChunkedBody(body, rest []byte, trailer map[string]string, hasTrailer bool) []byte {
	var b []byte
	chunkSize := 1
	for len(body) > 0 {
		if chunkSize > len(body) {
			chunkSize = len(body)
		}
		b = append(b, []byte(fmt.Sprintf("%x\r\n", chunkSize))...)
		b = append(b, body[:chunkSize]...)
		b = append(b, []byte("\r\n")...)
		body = body[chunkSize:]
		chunkSize++
	}
	if hasTrailer {
		b = append(b, "0\r\n"...)
		for k, v := range trailer {
			b = append(b, k...)
			b = append(b, ": "...)
			b = append(b, v...)
			b = append(b, "\r\n"...)
		}
		b = append(b, "\r\n"...)
	}
	return append(b, rest...)
}

func testChunkedSkipRest(t *testing.T, data, rest string) {
	var pool bytebufferpool.Pool
	reader := mock.NewZeroCopyReader(data)

	bs := AcquireBodyStream(pool.Get(), reader, &protocol.Trailer{}, -1)
	err := bs.(*bodyStream).skipRest()
	assert.Nil(t, err)

	restData, err := io.ReadAll(reader)
	assert.Nil(t, err)
	assert.Equal(t, rest, string(restData))
}

func testChunkedSkipRestWithBodySize(t *testing.T, bodySize int) {
	body := mock.CreateFixedBody(bodySize)
	rest := mock.CreateFixedBody(bodySize)
	data := createChunkedBody(body, rest, map[string]string{"foo": "bar"}, true)

	testChunkedSkipRest(t, string(data), string(rest))
}

func TestChunkedSkipRest(t *testing.T) {
	t.Parallel()

	testChunkedSkipRest(t, "0\r\n\r\n", "")
	testChunkedSkipRest(t, "0\r\n\r\nHTTP/1.1 / POST", "HTTP/1.1 / POST")
	testChunkedSkipRest(t, "0\r\nWind: test\r\nfoo: bar\r\n\r\nHTTP/1.1 / POST", "HTTP/1.1 / POST")

	testChunkedSkipRestWithBodySize(t, 5)

	// medium-size body
	testChunkedSkipRestWithBodySize(t, 43488)

	// big body
	testChunkedSkipRestWithBodySize(t, 3*1024*1024)
}

func TestBodyStream_Reset(t *testing.T) {
	t.Parallel()
	bs := bodyStream{
		prefetchedBytes: bytes.NewReader([]byte("aaa")),
		reader:          mock.NewZeroCopyReader("bbb"),
		trailer:         &protocol.Trailer{},
		offset:          10,
		contentLength:   20,
		chunkLeft:       50,
		chunkEOF:        true,
	}

	bs.reset()

	assert.Nil(t, bs.prefetchedBytes)
	assert.Nil(t, bs.reader)
	assert.Nil(t, bs.trailer)
	assert.Equal(t, 0, bs.offset)
	assert.Equal(t, 0, bs.contentLength)
	assert.Equal(t, 0, bs.chunkLeft)
	assert.False(t, bs.chunkEOF)
}

func TestReadBodyWithStreaming(t *testing.T) {
	t.Run("TestBodyFixedSize", func(t *testing.T) {
		bodySize := 1024
		body := mock.CreateFixedBody(bodySize)
		reader := mock.NewZeroCopyReader(string(body))
		dst, err := ReadBodyWithStreaming(reader, bodySize, -1, nil)
		assert.Nil(t, err)
		assert.Equal(t, body, dst)
	})

	t.Run("TestBodyFixedSizeMaxContentLength", func(t *testing.T) {
		bodySize := 8 * 1024 * 2
		body := mock.CreateFixedBody(bodySize)
		reader := mock.NewZeroCopyReader(string(body))
		dst, err := ReadBodyWithStreaming(reader, bodySize, 8*1024*10, nil)
		assert.Nil(t, err)
		assert.Equal(t, body[:maxContentLengthInStream], dst)
	})

	t.Run("TestBodyIdentity", func(t *testing.T) {
		bodySize := 1024
		body := mock.CreateFixedBody(bodySize)
		reader := mock.NewZeroCopyReader(string(body))
		dst, err := ReadBodyWithStreaming(reader, -2, 512, nil)
		assert.Nil(t, err)
		assert.Equal(t, body, dst)
	})

	t.Run("TestErrBodyTooLarge", func(t *testing.T) {
		bodySize := 2048
		body := mock.CreateFixedBody(bodySize)
		reader := mock.NewZeroCopyReader(string(body))
		dst, err := ReadBodyWithStreaming(reader, bodySize, 1024, nil)
		assert.True(t, errors.Is(err, errBodyTooLarge))
		assert.Equal(t, body[:len(dst)], dst)
	})

	t.Run("TestErrChunkedStream", func(t *testing.T) {
		bodySize := 1024
		body := mock.CreateFixedBody(bodySize)
		reader := mock.NewZeroCopyReader(string(body))
		dst, err := ReadBodyWithStreaming(reader, -1, bodySize, nil)
		assert.True(t, errors.Is(err, errChunkedStream))
		assert.Nil(t, dst)
	})
}
