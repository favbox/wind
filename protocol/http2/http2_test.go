package http2

import (
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/favbox/wind/protocol"
)

func init() {
	inTests = true
	DebugGoroutines = true
	flag.BoolVar(&VerboseLogs, "verboseh2", VerboseLogs, "Verbose HTTP/2 debug logging")
}

func TestSettingString(t *testing.T) {
	tests := []struct {
		s    Setting
		want string
	}{
		{Setting{SettingMaxFrameSize, 123}, "[MAX_FRAME_SIZE = 123]"},
		{Setting{1<<16 - 1, 123}, "[UNKNOWN_SETTING_65535 = 123]"},
	}
	for i, tt := range tests {
		got := fmt.Sprint(tt.s)
		if got != tt.want {
			t.Errorf("%d. for %#v, string = %q; want %q", i, tt.s, got, tt.want)
		}
	}
}

type logFilter interface {
	IsFilter(p string) bool
}

type twriter struct {
	t      testing.TB
	filter logFilter
}

func (w twriter) Write(p []byte) (n int, err error) {
	if w.filter != nil {
		ps := string(p)
		if w.filter.IsFilter(ps) {
			return len(p), nil // no logging
		}
	}
	w.t.Logf("%s", p)
	return len(p), nil
}

func cleanDate(res *protocol.Response) {
	if d := res.Header.Get("Date"); len(d) > 0 {
		res.Header.Set("Date", "XXX")
	}
}

// waitCondition reports whether fn eventually returned true,
// checking immediately and then every checkEvery amount,
// until waitFor has elapsed, at which point it returns false.
func waitCondition(waitFor, checkEvery time.Duration, fn func() bool) bool {
	deadline := time.Now().Add(waitFor)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(checkEvery)
	}
	return false
}
