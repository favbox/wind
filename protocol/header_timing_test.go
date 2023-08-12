package protocol

import (
	"net/http"
	"strconv"
	"testing"
)

func BenchmarkHTTPHeaderGet(b *testing.B) {
	b.ReportAllocs()
	hh := make(http.Header)
	hh.Set("x-logid", "abc")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hh.Get("x-logid")
	}
}

func BenchmarkWindHeaderGet(b *testing.B) {
	b.ReportAllocs()
	wh := new(ResponseHeader)
	wh.Set("x-logid", "abc")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wh.Get("x-logid")
	}
}

func BenchmarkHTTPHeaderSet(b *testing.B) {
	hh := make(http.Header)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hh.Set("X-tt-logid", "abc123456789")
	}
}

func BenchmarkWindHeaderSet(b *testing.B) {
	wh := new(ResponseHeader)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wh.Set("X-tt-logid", "abc123456789")
	}
}

func BenchmarkHTTPHeaderAdd(b *testing.B) {
	hh := make(http.Header)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hh.Add("X-tt-"+strconv.Itoa(i), "abc123456789")
	}
}

func BenchmarkWindHeaderAdd(b *testing.B) {
	wh := new(ResponseHeader)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wh.Add("X-tt-"+strconv.Itoa(i), "abc123456789")
	}
}

func BenchmarkRefreshServerDate(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		refreshServerDate()
	}
}
