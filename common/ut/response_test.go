package ut

import (
	"testing"

	"github.com/favbox/wind/protocol/consts"
	"github.com/stretchr/testify/assert"
)

func TestResponseRecorder_Result(t *testing.T) {
	r := new(ResponseRecorder)
	result := r.Result()
	assert.Equal(t, consts.StatusOK, result.StatusCode())
}

func TestResponseRecorder_Flush(t *testing.T) {
	r := new(ResponseRecorder)
	r.Flush()
	result := r.Result()
	assert.Equal(t, consts.StatusOK, result.StatusCode())
}

func TestResponseRecorder_WriteHeader(t *testing.T) {
	r := NewRecorder()
	r.WriteHeader(consts.StatusCreated)
	r.WriteHeader(consts.StatusOK)
	result := r.Result()
	assert.Equal(t, consts.StatusCreated, result.StatusCode())
}

func TestResponseRecorder_WriteString(t *testing.T) {
	r := NewRecorder()
	r.WriteString("hello")
	result := r.Result()
	assert.Equal(t, "hello", string(result.Body()))
}

func TestResponseRecorder_Write(t *testing.T) {
	r := NewRecorder()
	r.Write([]byte("hello"))
	result := r.Result()
	assert.Equal(t, "hello", string(result.Body()))
}
