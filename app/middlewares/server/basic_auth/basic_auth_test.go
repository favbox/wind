package basic_auth

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/favbox/wind/app"
	"github.com/favbox/wind/internal/bytesconv"
	"github.com/stretchr/testify/assert"
)

func TestPairs(t *testing.T) {
	t1 := Accounts{"test1": "value1"}
	t2 := Accounts{"test2": "value2"}
	p1 := constructPairs(t1)
	p2 := constructPairs(t2)

	u1, ok1 := p1.findValue("Basic dGVzdDE6dmFsdWUx")
	u2, ok2 := p2.findValue("Basic dGVzdDI6dmFsdWUy")
	_, ok3 := p1.findValue("bad header")
	_, ok4 := p2.findValue("bad header")
	assert.True(t, ok1)
	assert.Equal(t, "test1", u1)
	assert.True(t, ok2)
	assert.Equal(t, "test2", u2)
	assert.False(t, ok3)
	assert.False(t, ok4)
}

func TestBasicAuth(t *testing.T) {
	userName1 := "user1"
	password1 := "value1"
	userName2 := "user2"
	password2 := "value2"

	c1 := app.RequestContext{}
	encodeStr := "Basic " + base64.StdEncoding.EncodeToString(bytesconv.S2b(userName1+":"+password1))
	c1.Request.Header.Add("Authorization", encodeStr)

	t1 := Accounts{userName1: password1}
	handler := BasicAuth(t1)
	handler(context.TODO(), &c1)

	user, ok := c1.Get("user")
	assert.Equal(t, userName1, user)
	assert.True(t, ok)

	c2 := app.RequestContext{}
	encodeStr = "Basic " + base64.StdEncoding.EncodeToString(bytesconv.S2b(userName2+":"+password2))
	c2.Request.Header.Add("Authorization", encodeStr)

	handler(context.TODO(), &c2)

	user, ok = c2.Get("user")
	assert.Nil(t, user)
	assert.False(t, ok)
}
