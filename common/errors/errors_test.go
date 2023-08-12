package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError(t *testing.T) {
	baseError := errors.New("test error")
	err := &Error{
		Err:  baseError,
		Type: ErrorTypePrivate,
	}
	assert.Equal(t, err.Error(), baseError.Error())
	assert.Equal(t, map[string]any{"error": baseError.Error()}, err.JSON())

	assert.Equal(t, err.SetType(ErrorTypePublic), err)
	assert.Equal(t, ErrorTypePublic, err.Type)

	assert.Equal(t, err.SetMeta("some data"), err)
	assert.Equal(t, "some data", err.Meta)
	assert.Equal(t, map[string]any{
		"error": baseError.Error(),
		"meta":  "some data",
	}, err.JSON())

	err.SetMeta(map[string]any{
		"status": 200,
		"data":   "some data",
	})
	assert.Equal(t, map[string]any{
		"error":  baseError.Error(),
		"status": 200,
		"data":   "some data",
	}, err.JSON())

	err.SetMeta(map[string]any{
		"error":  "custom error",
		"status": 200,
		"data":   "some data",
	})
	assert.Equal(t, map[string]any{
		"error":  "custom error",
		"status": 200,
		"data":   "some data",
	}, err.JSON())

	type customError struct {
		status string
		data   string
	}
	err.SetMeta(customError{status: "200", data: "other data"})
	assert.Equal(t, customError{
		status: "200",
		data:   "other data",
	}, err.JSON())
	fmt.Printf("%#v\n", err.JSON())
}

func TestErrorSlice(t *testing.T) {
	errs := ErrorChain{
		{Err: errors.New("first"), Type: ErrorTypePrivate},
		{Err: errors.New("second"), Type: ErrorTypePrivate, Meta: "some data"},
		{Err: errors.New("third"), Type: ErrorTypePublic, Meta: map[string]any{"status": "400"}},
	}
	assert.Equal(t, errs, errs.ByType(ErrorTypeAny))
	assert.Equal(t, "third", errs.Last().Error())
	assert.Equal(t, []string{"first", "second", "third"}, errs.Errors())
	assert.Equal(t, []string{"third"}, errs.ByType(ErrorTypePublic).Errors())
	assert.Equal(t, []string{"first", "second"}, errs.ByType(ErrorTypePrivate).Errors())
	assert.Equal(t, []string{"first", "second", "third"}, errs.ByType(ErrorTypePublic|ErrorTypePrivate).Errors())
	assert.Equal(t, "", errs.ByType(ErrorTypeBind).String())
	assert.Equal(t, `Error #01: first
Error #02: second
     Meta: some data
Error #03: third
     Meta: map[status:400]
`, errs.String())
	assert.Equal(t, []any{
		map[string]any{"error": "first"},
		map[string]any{"error": "second", "meta": "some data"},
		map[string]any{"error": "third", "status": "400"},
	}, errs.JSON())

	errs = ErrorChain{
		{Err: errors.New("first"), Type: ErrorTypePrivate},
	}
	assert.Equal(t, map[string]any{"error": "first"}, errs.JSON())

	errs = ErrorChain{}
	assert.Equal(t, true, errs.Last() == nil)
	assert.Nil(t, errs.JSON())
	assert.Equal(t, "", errs.String())
}

func TestErrorFormat(t *testing.T) {
	err := Newf(ErrorTypeAny, nil, "caused by %s", "reason")
	assert.Equal(t, New(errors.New("caused by reason"), ErrorTypeAny, nil), err)
	publicErr := NewPublicf("caused by %s", "reason")
	assert.Equal(t, New(errors.New("caused by reason"), ErrorTypePublic, nil), publicErr)
	privateErr := NewPrivatef("caused by %s", "reason")
	assert.Equal(t, New(errors.New("caused by reason"), ErrorTypePrivate, nil), privateErr)
}
