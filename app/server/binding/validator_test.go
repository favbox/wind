package binding

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidator_ValidateStruct(t *testing.T) {
	type User struct {
		Age int `vd:"$>=0 && $<=130"`
	}

	user := &User{Age: 135}
	err := DefaultValidator().ValidateStruct(user)
	assert.NotNil(t, err)
}

func TestValidator_ValidateTag(t *testing.T) {
	type User struct {
		Age int `query:"age" vt:"$>=0&&$<=130"`
	}

	user := &User{Age: 135}
	validateConfig := NewValidateConfig()
	validateConfig.ValidateTag = "vt"
	vd := NewValidator(validateConfig)
	err := vd.ValidateStruct(user)
	assert.NotNil(t, err)

	bindConfig := NewBindConfig()
	bindConfig.Validator = vd
	binder := NewBinder(bindConfig)
	user = &User{}
	req := newMockRequest().
		SetRequestURI("http://foobar.com?age=135").
		SetHeader("h", "header")
	err = binder.BindAndValidate(req.Req, user, nil)
	assert.NotNil(t, err)
	fmt.Println(user.Age)
}
