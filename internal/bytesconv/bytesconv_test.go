package bytesconv

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/favbox/wind/common/bytebufferpool"
	"github.com/favbox/wind/common/mock"
	"github.com/favbox/wind/network"
	"github.com/stretchr/testify/assert"
)

func TestAppendQuotedArg(t *testing.T) {
	t.Parallel()

	// 与 url.QueryEscape 同步
	allCases := make([]byte, 256)
	for i := 0; i < 256; i++ {
		allCases[i] = byte(i)
	}
	res := B2s(AppendQuotedArg(nil, allCases))
	expect := url.QueryEscape(B2s(allCases))
	assert.Equal(t, expect, res)
}

func TestAppendQuotedPath(t *testing.T) {
	t.Parallel()

	// 测试所有字符
	pathSegment := make([]byte, 256)
	for i := 0; i < 256; i++ {
		pathSegment[i] = byte(i)
	}
	for _, s := range []struct {
		path string
	}{
		{"/"},
		{"//"},
		{"/foo/bar"},
		{"*"},
		{"/foo/" + B2s(pathSegment)},
	} {
		u := url.URL{Path: s.path}
		expectedS := u.EscapedPath()
		res := B2s(AppendQuotedPath(nil, S2b(s.path)))
		assert.Equal(t, expectedS, res)
	}
}

func TestLowercaseBytes(t *testing.T) {
	t.Parallel()

	for _, v := range []struct {
		b1, b2 []byte
	}{
		{[]byte("wind-HTTP"), []byte("wind-http")},
		{[]byte("wind"), []byte("wind")},
		{[]byte("HTTP"), []byte("http")},
	} {
		LowercaseBytes(v.b1)
		assert.Equal(t, v.b2, v.b1)
	}
}

func TestB2s(t *testing.T) {
	t.Parallel()

	for _, v := range []struct {
		s string
		b []byte
	}{
		{"wind-http", []byte("wind-http")},
		{"wind", []byte("wind")},
		{"http", []byte("http")},
	} {
		assert.Equal(t, v.s, B2s(v.b))
	}
}

func TestS2b(t *testing.T) {
	t.Parallel()

	for _, v := range []struct {
		s string
		b []byte
	}{
		{"wind-http", []byte("wind-http")},
		{"wind", []byte("wind")},
		{"http", []byte("http")},
	} {
		assert.Equal(t, v.b, S2b(v.s))
	}
}

func TestAppendUint(t *testing.T) {
	t.Parallel()

	for _, s := range []struct {
		n int
	}{
		{0},
		{123},
		{0x7fffffff},
	} {
		expectedS := fmt.Sprintf("%d", s.n)
		s := AppendUint(nil, s.n)
		assert.Equal(t, expectedS, B2s(s))
	}
}

func BenchmarkB2s(b *testing.B) {
	s := "hi"
	bs := []byte("hi")

	b.Run("std/string", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = string(bs)
		}
	})

	b.Run("bytesconv/B2s", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = B2s(bs)
		}
	})

	b.Run("std/[]byte", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = []byte(s)
		}
	})

	b.Run("bytesconv/S2b", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = S2b(s)
		}
	})

	b.Run("bytesconv/B2s", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = B2s(bs)
		}
	})

	b.Run("std/string multicore", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = string(bs)
			}
		})
	})

	b.Run("bytesconv/B2s multicore", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = B2s(bs)
			}
		})
	})
}

func BenchmarkAppendQuotedArg(b *testing.B) {
	allCases := make([]byte, 256)
	for i := 0; i < 256; i++ {
		allCases[i] = byte(i)
	}

	b.Run("AppendQuotedArg", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = B2s(AppendQuotedArg(nil, allCases))
			}
		})
	})

	b.Run("url.QueryEscape", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = url.QueryEscape(B2s(allCases))
			}
		})
	})
}

func BenchmarkAppendQuotedPath(b *testing.B) {
	allCases := make([]byte, 256)
	for i := 0; i < 256; i++ {
		allCases[i] = byte(i)
	}
	u := url.URL{Path: B2s(allCases)}

	b.Run("AppendQuotedPath", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = B2s(AppendQuotedPath(nil, allCases))
			}
		})
	})

	b.Run("url.QueryEscape", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = u.EscapedPath()
			}
		})
	})
}

// 用于 32 位和 64 位的通用测试函数。
func testWriteHexInt(t *testing.T, n int, expectedS string) {
	w := bytebufferpool.Get()
	zw := network.NewWriter(w)
	if err := WriteHexInt(zw, n); err != nil {
		t.Errorf("异常发生在写入十六进制值 %x: %v", n, err)
	}
	if err := zw.Flush(); err != nil {
		t.Fatalf("异常发生于冲刷十六进制值 %x: %v", n, err)
	}
	s := B2s(w.B)
	assert.Equal(t, s, expectedS)
}

// 用于 32 位和 64 位的通用测试函数。
func testReadHexInt(t *testing.T, s string, expectedN int) {
	zr := mock.NewZeroCopyReader(s)
	n, err := ReadHexInt(zr)
	if err != nil {
		t.Errorf("异常错误：%q. s=%s", err, s)
	}
	assert.Equal(t, n, expectedN)
}
