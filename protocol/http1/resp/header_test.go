package resp

import (
	"bytes"
	"testing"

	"github.com/cloudwego/netpoll"
	"github.com/favbox/gosky/wind/pkg/protocol"
)

func TestResponseHeaderCookie(t *testing.T) {
	t.Parallel()

	var h protocol.ResponseHeader
	var c protocol.Cookie

	c.SetKey("foobar")
	c.SetValue("aaa")
	h.SetCookie(&c)

	c.SetKey("йцук")
	c.SetDomain("foobar.com")
	h.SetCookie(&c)

	c.Reset()
	c.SetKey("foobar")
	if !h.Cookie(&c) {
		t.Fatalf("Cannot find cookie %q", c.Key())
	}

	var expectedC1 protocol.Cookie
	expectedC1.SetKey("foobar")
	expectedC1.SetValue("aaa")
	if !equalCookie(&expectedC1, &c) {
		t.Fatalf("unexpected cookie\n%#v\nExpected\n%#v\n", &c, &expectedC1)
	}

	c.SetKey("йцук")
	if !h.Cookie(&c) {
		t.Fatalf("cannot find cookie %q", c.Key())
	}

	var expectedC2 protocol.Cookie
	expectedC2.SetKey("йцук")
	expectedC2.SetValue("aaa")
	expectedC2.SetDomain("foobar.com")
	if !equalCookie(&expectedC2, &c) {
		t.Fatalf("unexpected cookie\n%v\nExpected\n%v\n", &c, &expectedC2)
	}

	h.VisitAllCookie(func(key, value []byte) {
		var cc protocol.Cookie
		if err := cc.ParseBytes(value); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(key, cc.Key()) {
			t.Fatalf("Unexpected cookie key %q. Expected %q", key, cc.Key())
		}
		switch {
		case bytes.Equal(key, []byte("foobar")):
			if !equalCookie(&expectedC1, &cc) {
				t.Fatalf("unexpected cookie\n%v\nExpected\n%v\n", &cc, &expectedC1)
			}
		case bytes.Equal(key, []byte("йцук")):
			if !equalCookie(&expectedC2, &cc) {
				t.Fatalf("unexpected cookie\n%v\nExpected\n%v\n", &cc, &expectedC2)
			}
		default:
			t.Fatalf("unexpected cookie key %q", key)
		}
	})

	w := &bytes.Buffer{}
	zw := netpoll.NewWriter(w)
	if err := WriteHeader(&h, zw); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if err := zw.Flush(); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	h.DelAllCookies()

	var h1 protocol.ResponseHeader
	zr := netpoll.NewReader(w)
	if err := ReadHeader(&h1, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	c.SetKey("foobar")
	if !h1.Cookie(&c) {
		t.Fatalf("Cannot find cookie %q", c.Key())
	}
	if !equalCookie(&expectedC1, &c) {
		t.Fatalf("unexpected cookie\n%v\nExpected\n%v\n", &c, &expectedC1)
	}

	h1.DelCookie("foobar")
	if h.Cookie(&c) {
		t.Fatalf("Unexpected cookie found: %v", &c)
	}
	if h1.Cookie(&c) {
		t.Fatalf("Unexpected cookie found: %v", &c)
	}

	c.SetKey("йцук")
	if !h1.Cookie(&c) {
		t.Fatalf("cannot find cookie %q", c.Key())
	}
	if !equalCookie(&expectedC2, &c) {
		t.Fatalf("unexpected cookie\n%v\nExpected\n%v\n", &c, &expectedC2)
	}

	h1.DelCookie("йцук")
	if h.Cookie(&c) {
		t.Fatalf("Unexpected cookie found: %v", &c)
	}
	if h1.Cookie(&c) {
		t.Fatalf("Unexpected cookie found: %v", &c)
	}
}

func equalCookie(c1, c2 *protocol.Cookie) bool {
	if !bytes.Equal(c1.Key(), c2.Key()) {
		return false
	}
	if !bytes.Equal(c1.Value(), c2.Value()) {
		return false
	}
	if !c1.Expire().Equal(c2.Expire()) {
		return false
	}
	if !bytes.Equal(c1.Domain(), c2.Domain()) {
		return false
	}
	if !bytes.Equal(c1.Path(), c2.Path()) {
		return false
	}
	return true
}
