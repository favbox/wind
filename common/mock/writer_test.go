package mock

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtWriter(t *testing.T) {
	b1 := []byte("abcdef4343")
	buf := new(bytes.Buffer)
	isFinal := false
	w := &ExtWriter{
		Buf:     buf,
		IsFinal: &isFinal,
	}

	// write
	n, err := w.Write(b1)
	assert.Equal(t, nil, err)
	assert.Equal(t, len(b1), n)

	// flush
	err = w.Flush()
	assert.Equal(t, nil, err)
	assert.Equal(t, b1, w.Buf.Bytes())

	// setbody
	b2 := []byte("abc")
	w.SetBody(b2)
	err = w.Flush()
	assert.Equal(t, nil, err)
	assert.Equal(t, b2, w.Buf.Bytes())

	_ = w.Finalize()
	assert.Equal(t, true, *(w.IsFinal))
}
