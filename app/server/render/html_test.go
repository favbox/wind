package render

import (
	"os"
	"testing"
	"time"
)

func TestHTMLDebug_StartChecker_timer(t *testing.T) {
	render := &HTMLDebug{RefreshInterval: time.Second}
	select {
	case <-render.reloadCh:
		t.Fatalf("不该触发重载")
	default:
	}

	render.startChecker()
	select {
	case <-time.After(render.RefreshInterval + 500*time.Millisecond):
		t.Fatalf("应在1.5秒内触发重载")
	default:

	}
}

func TestHTMLDebug_StartChecker_fsnotify(t *testing.T) {
	f, _ := os.CreateTemp("./", "test.tmpl")
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()
	render := &HTMLDebug{
		Files: []string{f.Name()},
	}
	select {
	case <-render.reloadCh:
		t.Fatalf("不该触发重载")
	default:
	}
	render.startChecker()
	f.Write([]byte("hello"))
	f.Sync()
	select {
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("不该被立即触发")
	case <-render.reloadCh:
	}
	select {
	case <-render.reloadCh:
		t.Fatalf("不该被触发")
	default:
	}
}
