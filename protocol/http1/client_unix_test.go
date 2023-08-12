package http1

import (
	"context"
	"errors"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	errs "github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/network/netpoll"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/consts"
	"github.com/stretchr/testify/assert"
)

func TestGcBodyStream(t *testing.T) {
	srv := &http.Server{
		Addr: "127.0.0.1:11001",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			for range [1024]int{} {
				w.Write([]byte("hello world\n"))
			}
		}),
	}
	go srv.ListenAndServe()
	time.Sleep(100 * time.Millisecond)

	c := &HostClient{
		ClientOptions: &ClientOptions{
			Dialer:             netpoll.NewDialer(),
			ResponseBodyStream: true,
		},
		Addr: "127.0.0.1:11001",
	}

	for i := 0; i < 10; i++ {
		req, resp := protocol.AcquireRequest(), protocol.AcquireResponse()
		req.SetRequestURI("http://127.0.0.1:11001")
		req.SetMethod(consts.MethodPost)
		err := c.Do(context.Background(), req, resp)
		if err != nil {
			t.Errorf("客户端执行错误 = %v", err.Error())
		}
	}

	// time.Sleep(time.Minute)

	runtime.GC()
	// 等待 gc
	time.Sleep(100 * time.Millisecond)
	c.CloseIdleConnections()
	assert.Equal(t, 0, c.ConnPoolState().TotalConnNum)
}

func TestHostClient_SetMaxConns(t *testing.T) {
	srv := &http.Server{
		Addr: "127.0.0.1:11002",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("hello world\n"))
		}),
	}
	go srv.ListenAndServe()
	time.Sleep(100 * time.Millisecond)

	c := &HostClient{
		ClientOptions: &ClientOptions{
			Dialer:             netpoll.NewDialer(),
			ResponseBodyStream: true,
			MaxConnWaitTimeout: time.Millisecond * 100,
			MaxConns:           5,
		},
		Addr: "127.0.0.1:11002",
	}

	var successCount int32
	var noFreeCount int32
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, resp := protocol.AcquireRequest(), protocol.AcquireResponse()
			req.SetRequestURI("http://127.0.0.1:11002")
			req.SetMethod(consts.MethodPost)
			err := c.Do(context.Background(), req, resp)
			if err != nil {
				if errors.Is(err, errs.ErrNoFreeConns) {
					atomic.AddInt32(&noFreeCount, 1)
					return
				}
				t.Errorf("客户端执行错误 = %v", err.Error())
			}
			atomic.AddInt32(&successCount, 1)
		}()
	}
	wg.Wait()

	assert.True(t, atomic.LoadInt32(&successCount) == 5)
	assert.True(t, atomic.LoadInt32(&noFreeCount) == 5)
	assert.Equal(t, 0, c.ConnectionCount())
	assert.Equal(t, 5, c.WantConnectionCount())

	// GC 会触发设置 runtime.SetFinalizer
	runtime.GC()
	// 等待 gc
	time.Sleep(100 * time.Millisecond)

	c.CloseIdleConnections()
	assert.Equal(t, 0, c.WantConnectionCount())
}
