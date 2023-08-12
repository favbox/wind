package mock

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateCreateFixedBody(t *testing.T) {
	bodySize := 10
	resFixedBody := "0123456789"
	b := CreateFixedBody(bodySize)
	if string(b) != resFixedBody {
		t.Fatalf("Unexpected %s. Expecting %s.", b, resFixedBody)
	}

	nilFixedBody := CreateFixedBody(0)
	if nilFixedBody != nil {
		t.Fatalf("Unexpected %s. Expecting a nil", nilFixedBody)
	}
}

func TestGenerateCreateChunkedBody(t *testing.T) {
	bodySize := 10
	b := CreateFixedBody(bodySize)
	trailer := map[string]string{"Foo": "chunked shit"}
	expectCb := "1\r\n0\r\n2\r\n12\r\n3\r\n345\r\n4\r\n6789\r\n0\r\nFoo: chunked shit\r\n\r\n"

	cb := CreateChunkedBody(b, trailer, true)
	assert.Equal(t, expectCb, string(cb))
}
