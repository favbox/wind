package protocol

import (
	"strings"
	"testing"

	"github.com/favbox/wind/internal/bytestr"
	"github.com/favbox/wind/protocol/consts"
	"github.com/stretchr/testify/assert"
)

func TestTrailerAdd(t *testing.T) {
	var tr Trailer
	assert.Nil(t, tr.Add("foo", "value1"))
	assert.Nil(t, tr.Add("foo", "value2"))
	assert.Nil(t, tr.Add("bar", "value3"))
	assert.True(t, strings.Contains(string(tr.Header()), "Foo: value1"))
	assert.True(t, strings.Contains(string(tr.Header()), "Foo: value2"))
	assert.True(t, strings.Contains(string(tr.Header()), "Bar: value3"))
}

func TestHeaderTrailerSet(t *testing.T) {
	h := &RequestHeader{}

	// only one trailer
	h.Set("Trailer", "Foo")
	assert.True(t, strings.Contains(string(h.Trailer().Header()), "Foo:"))

	// multi trailer
	h.Set("Trailer", "Foo, bar, wind")
	assert.True(t, strings.Contains(string(h.Trailer().Header()), "Foo:"))
	assert.True(t, strings.Contains(string(h.Trailer().Header()), "Bar:"))
	assert.True(t, strings.Contains(string(h.Trailer().Header()), "Wind:"))

	// all lowercase
	h.Set("Trailer", "foo,wind,aaa")
	assert.True(t, strings.Contains(string(h.Trailer().Header()), "Foo:"))
	assert.True(t, strings.Contains(string(h.Trailer().Header()), "Wind:"))
	assert.True(t, strings.Contains(string(h.Trailer().Header()), "Aaa:"))

	// all uppercase
	h.Set("Trailer", "FOO,wind,AAA")
	assert.True(t, strings.Contains(string(h.Trailer().Header()), "Foo:"))
	assert.True(t, strings.Contains(string(h.Trailer().Header()), "Wind:"))
	assert.True(t, strings.Contains(string(h.Trailer().Header()), "Aaa:"))

	// with '-'
	h.Set("Trailer", "FOO-wind-AAA")
	assert.True(t, strings.Contains(string(h.Trailer().Header()), "Foo-Wind-Aaa:"))

	// more space
	h.Set("Trailer", "      foo,      wind       ,        aaa      ")
	assert.True(t, strings.Contains(string(h.Trailer().Header()), "Foo:"))
	assert.True(t, strings.Contains(string(h.Trailer().Header()), "Wind:"))
	assert.True(t, strings.Contains(string(h.Trailer().Header()), "Aaa:"))
}

func TestTrailerAddError(t *testing.T) {
	var tr Trailer
	err := tr.Add(consts.HeaderContentType, "")
	assert.NotNil(t, err)
	assert.NotNil(t, tr.Set(consts.HeaderProxyConnection, ""))
}

func TestTrailerDel(t *testing.T) {
	var tr Trailer
	assert.Nil(t, tr.Add("foo", "value1"))
	assert.Nil(t, tr.Add("foo", "value2"))
	assert.Nil(t, tr.Add("bar", "value3"))
	tr.Del("foo")
	assert.False(t, strings.Contains(string(tr.Header()), "Foo: value1"))
	assert.False(t, strings.Contains(string(tr.Header()), "Foo: value2"))
	assert.True(t, strings.Contains(string(tr.Header()), "Bar: value3"))
}

func TestTrailerSet(t *testing.T) {
	var tr Trailer
	assert.Nil(t, tr.Set("foo", "value1"))
	assert.Nil(t, tr.Set("foo", "value2"))
	assert.Nil(t, tr.Set("bar", "value3"))
	assert.False(t, strings.Contains(string(tr.Header()), "Foo: value1"))
	assert.True(t, strings.Contains(string(tr.Header()), "Foo: value2"))
	assert.True(t, strings.Contains(string(tr.Header()), "Bar: value3"))
}

func TestTrailerGet(t *testing.T) {
	var tr Trailer
	assert.Nil(t, tr.Add("foo", "value1"))
	assert.Nil(t, tr.Add("bar", "value3"))
	assert.Equal(t, tr.Get("foo"), "value1")
	assert.Equal(t, tr.Get("bar"), "value3")
}

func TestTrailerUpdateArgBytes(t *testing.T) {
	var tr Trailer
	assert.Nil(t, tr.addArgBytes([]byte("Foo"), []byte("value0"), argsNoValue))
	assert.Nil(t, tr.UpdateArgBytes([]byte("Foo"), []byte("value1")))
	assert.Nil(t, tr.UpdateArgBytes([]byte("Foo"), []byte("value2")))
	assert.Nil(t, tr.UpdateArgBytes([]byte("Bar"), []byte("value3")))
	assert.True(t, strings.Contains(string(tr.Header()), "Foo: value1"))
	assert.False(t, strings.Contains(string(tr.Header()), "Foo: value2"))
	assert.False(t, strings.Contains(string(tr.Header()), "Bar: value3"))
}

func TestTrailerEmpty(t *testing.T) {
	var tr Trailer
	assert.Equal(t, tr.Empty(), true)
	assert.Nil(t, tr.Set("foo", ""))
	assert.Equal(t, tr.Empty(), false)
}

func TestTrailerVisitAll(t *testing.T) {
	var tr Trailer
	assert.Nil(t, tr.Add("foo", "value1"))
	assert.Nil(t, tr.Add("bar", "value2"))
	tr.VisitAll(
		func(k, v []byte) {
			key := string(k)
			value := string(v)
			if (key != "Foo" || value != "value1") && (key != "Bar" || value != "value2") {
				t.Fatalf("Unexpected (%v, %v). Expected %v", key, value, "(foo, value1) or (bar, value2)")
			}
		})
}

func TestIsBadTrailer(t *testing.T) {
	assert.True(t, IsBadTrailer(bytestr.StrAuthorization))
	assert.True(t, IsBadTrailer(bytestr.StrContentEncoding))
	assert.True(t, IsBadTrailer(bytestr.StrContentLength))
	assert.True(t, IsBadTrailer(bytestr.StrContentType))
	assert.True(t, IsBadTrailer(bytestr.StrContentRange))
	assert.True(t, IsBadTrailer(bytestr.StrConnection))
	assert.True(t, IsBadTrailer(bytestr.StrExpect))
	assert.True(t, IsBadTrailer(bytestr.StrHost))
	assert.True(t, IsBadTrailer(bytestr.StrKeepAlive))
	assert.True(t, IsBadTrailer(bytestr.StrMaxForwards))
	assert.True(t, IsBadTrailer(bytestr.StrProxyConnection))
	assert.True(t, IsBadTrailer(bytestr.StrProxyAuthenticate))
	assert.True(t, IsBadTrailer(bytestr.StrProxyAuthorization))
	assert.True(t, IsBadTrailer(bytestr.StrRange))
	assert.True(t, IsBadTrailer(bytestr.StrTE))
	assert.True(t, IsBadTrailer(bytestr.StrTrailer))
	assert.True(t, IsBadTrailer(bytestr.StrTransferEncoding))
	assert.True(t, IsBadTrailer(bytestr.StrWWWAuthenticate))
}
