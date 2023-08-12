package http1

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/favbox/wind/app"
	"github.com/favbox/wind/common/mock"
	"github.com/favbox/wind/common/tracer/stats"
	"github.com/favbox/wind/common/tracer/traceinfo"
	internalStats "github.com/favbox/wind/internal/stats"
)

func BenchmarkServer_Serve(b *testing.B) {
	server := NewServer()
	server.EnableTrace = true
	reqCtx := &app.RequestContext{}
	server.Core = &mockCore{
		ctxPool: &sync.Pool{
			New: func() any {
				ti := traceinfo.NewTraceInfo()
				ti.Stats().SetLevel(stats.LevelDetailed)
				reqCtx.SetTraceInfo(&mockTraceInfo{ti})
				return reqCtx
			},
		},
		controller: &internalStats.Controller{},
	}

	err := server.Serve(context.TODO(), mock.NewConn("GET /aaa HTTP/1.1\nHost: foobar.com\n\n"))
	if err != nil {
		fmt.Println(err.Error())
	}

	for i := 0; i < b.N; i++ {
		server.Serve(context.TODO(), mock.NewConn("GET /aaa HTTP/1.1\nHost: foobar.com\n\n"))
	}
}
