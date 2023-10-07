package route

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/favbox/wind/app"
	"github.com/favbox/wind/app/server/binding"
	"github.com/favbox/wind/common/config"
	errs "github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/common/mock"
	"github.com/favbox/wind/network"
	"github.com/favbox/wind/network/standard"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/consts"
	"github.com/favbox/wind/route/param"
	"github.com/stretchr/testify/assert"
)

type mockTransporter struct{}

func (m *mockTransporter) ListenAndServe(onData network.OnData) error {
	panic("implement me")
}

func (m *mockTransporter) Close() error {
	panic("implement me")
}

func (m *mockTransporter) Shutdown(ctx context.Context) error {
	panic("implement me")
}

type mockConn struct{}

func (m *mockConn) Handshake() error {
	return nil
}

func (m *mockConn) ConnectionState() tls.ConnectionState {
	return tls.ConnectionState{
		NegotiatedProtocol: "h2",
	}
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	// TODO implement me
	panic("implement me")
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	// TODO implement me
	panic("implement me")
}

func (m *mockConn) Close() error {
	// TODO implement me
	panic("implement me")
}

func (m *mockConn) LocalAddr() net.Addr {
	// TODO implement me
	panic("implement me")
}

func (m *mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.ParseIP("126.0.0.5"),
		Port: 8888,
		Zone: "",
	}
}

func (m *mockConn) SetDeadline(t time.Time) error {
	// TODO implement me
	panic("implement me")
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	// TODO implement me
	panic("implement me")
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	// TODO implement me
	panic("implement me")
}

func (m *mockConn) Len() int {
	// TODO implement me
	panic("implement me")
}

func (m *mockConn) Peek(n int) ([]byte, error) {
	// TODO implement me
	panic("implement me")
}

func (m *mockConn) Skip(n int) error {
	// TODO implement me
	panic("implement me")
}

func (m *mockConn) ReadByte() (byte, error) {
	// TODO implement me
	panic("implement me")
}

func (m *mockConn) ReadBinary(n int) (p []byte, err error) {
	// TODO implement me
	panic("implement me")
}

func (m *mockConn) Release() error {
	// TODO implement me
	panic("implement me")
}

func (m *mockConn) Malloc(n int) (buf []byte, err error) {
	// TODO implement me
	panic("implement me")
}

func (m *mockConn) WriteBinary(b []byte) (n int, err error) {
	// TODO implement me
	panic("implement me")
}

func (m *mockConn) Flush() error {
	// TODO implement me
	panic("implement me")
}

func (m *mockConn) SetReadTimeout(t time.Duration) error {
	return nil
}

func (m *mockConn) SetWriteTimeout(t time.Duration) error {
	// TODO implement me
	panic("implement me")
}

func newMockTransporter(options *config.Options) network.Transporter {
	return &mockTransporter{}
}

func TestNewEngine(t *testing.T) {
	defaultTransporter = standard.NewTransporter
	opt := config.NewOptions([]config.Option{})
	router := NewEngine(opt)
	assert.Equal(t, "standard", router.GetTransporterName())
	assert.Equal(t, "/", router.basePath)
	assert.Equal(t, router.engine, router)
	assert.Equal(t, 0, len(router.Handlers))
}

func TestNewEngine_WithTransporter(t *testing.T) {
	defaultTransporter = newMockTransporter
	opt := config.NewOptions([]config.Option{})
	router := NewEngine(opt)
	assert.Equal(t, "route", router.GetTransporterName())

	defaultTransporter = newMockTransporter
	opt.TransporterNewer = standard.NewTransporter
	router = NewEngine(opt)
	assert.Equal(t, "standard", router.GetTransporterName())
	assert.Equal(t, "route", GetTransporterName())
}

func TestGetTransporterName(t *testing.T) {
	name := getTransporterName(&mockTransporter{})
	assert.Equal(t, "route", name)
}

func TestEngine_Unescape(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))

	routes := []string{
		"/*all",
		"/cmd/:tool",
		"/src/*filepath",
		"/search/:query",
		"/info/:user/project/:project",
		"/info/:user",
	}

	for _, r := range routes {
		e.GET(r, func(c context.Context, ctx *app.RequestContext) {
			ctx.String(consts.StatusOK, ctx.Param(ctx.Query("key")))
		})
	}

	testRoutes := []struct {
		route string
		key   string
		want  string
	}{
		{"/", "", ""},
		{"/cmd/%E4%BD%A0%E5%A5%BD/", "tool", "你好"},
		{"/src/some/%E4%B8%96%E7%95%8C.png", "filepath", "some/世界.png"},
		{"/info/%E4%BD%A0%E5%A5%BD/project/%E4%B8%96%E7%95%8C", "user", "你好"},
		{"/info/%E4%BD%A0%E5%A5%BD/project/%E4%B8%96%E7%95%8C", "project", "世界"},
	}

	for _, tr := range testRoutes {
		w := performRequest(e, consts.MethodGet, tr.route+"?key="+tr.key)
		assert.Equal(t, consts.StatusOK, w.Code)
	}
}

func TestEngine_UnescapeRaw(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	e.options.UseRawPath = true

	routes := []string{
		"/*all",
		"/cmd/:tool/",
		"/src/*filepath",
		"/search/:query",
		"/info/:user/project/:project",
		"/info/:user",
	}

	for _, r := range routes {
		e.GET(r, func(c context.Context, ctx *app.RequestContext) {
			ctx.String(consts.StatusOK, ctx.Param(ctx.Query("key")))
		})
	}

	testRoutes := []struct {
		route string
		key   string
		want  string
	}{
		{"/", "", ""},
		{"/cmd/test/", "tool", "test"},
		{"/src/some/file.png", "filepath", "some/file.png"},
		{"/src/some/file+test.png", "filepath", "some/file test.png"},
		{"/src/some/file++++%%%%test.png", "filepath", "some/file++++%%%%test.png"},
		{"/src/some/file%2Ftest.png", "filepath", "some/file/test.png"},
		{"/search/someth!ng+in+ünìcodé", "query", "someth!ng in ünìcodé"},
		{"/info/gordon/project/go", "user", "gordon"},
		{"/info/gordon/project/go", "project", "go"},
		{"/info/slash%2Fgordon", "user", "slash/gordon"},
		{"/info/slash%2Fgordon/project/Project%20%231", "user", "slash/gordon"},
		{"/info/slash%2Fgordon/project/Project%20%231", "project", "Project #1"},
		{"/info/slash%%%%", "user", "slash%%%%"},
		{"/info/slash%%%%2Fgordon/project/Project%%%%20%231", "user", "slash%%%%2Fgordon"},
		{"/info/slash%%%%2Fgordon/project/Project%%%%20%231", "project", "Project%%%%20%231"},
	}
	for _, tr := range testRoutes {
		w := performRequest(e, http.MethodGet, tr.route+"?key="+tr.key)
		assert.Equal(t, consts.StatusOK, w.Code)
		assert.Equal(t, tr.want, w.Body.String())
	}
}

func TestConnectionClose(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	atomic.StoreUint32(&e.status, statusRunning)
	e.Init()
	e.GET("/foo", func(c context.Context, ctx *app.RequestContext) {
		ctx.String(consts.StatusOK, "ok")
	})
	conn := mock.NewConn("GET /foo HTTP/1.1\r\nHost: google.com\r\nConnection: close\r\n\r\n")
	err := e.Serve(context.Background(), conn)
	assert.True(t, errors.Is(err, errs.ErrShortConnection))
}

func TestConnectionClose1(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	atomic.StoreUint32(&e.status, statusRunning)
	e.Init()
	e.GET("/foo", func(c context.Context, ctx *app.RequestContext) {
		ctx.SetConnectionClose()
		ctx.String(consts.StatusOK, "ok")
	})

	conn := mock.NewConn("GET /foo HTTP/1.1\r\nHost: google.com\r\n\r\n")
	err := e.Serve(context.Background(), conn)
	assert.True(t, errors.Is(err, errs.ErrShortConnection))
}

func TestIdleTimeout(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	e.options.IdleTimeout = 0
	atomic.StoreUint32(&e.status, statusRunning)
	e.Init()
	e.GET("/foo", func(c context.Context, ctx *app.RequestContext) {
		time.Sleep(100 * time.Millisecond)
		ctx.String(consts.StatusOK, "ok")
	})

	conn := mock.NewConn("GET /foo HTTP/1.1\r\nHost: google.com\r\n\r\n")

	ch := make(chan error)
	startCh := make(chan error)
	go func() {
		<-startCh
		ch <- e.Serve(context.Background(), conn)
	}()
	close(startCh)

	select {
	case err := <-ch:
		if err != nil {
			t.Errorf("出错：%s", err)
		}
		return
	case <-time.Tick(120 * time.Millisecond):
		t.Errorf("超时！应在 120 毫秒内完成")
	}
}

func TestIdleTimeout01(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	e.options.IdleTimeout = 1 * time.Second
	atomic.StoreUint32(&e.status, statusRunning)
	e.Init()
	atomic.StoreUint32(&e.status, statusRunning)
	e.GET("/foo", func(c context.Context, ctx *app.RequestContext) {
		time.Sleep(10 * time.Millisecond)
		ctx.String(consts.StatusOK, "ok")
	})

	conn := mock.NewConn("GET /foo HTTP/1.1\r\nHost: google.com\r\n\r\n")

	ch := make(chan error)
	startCh := make(chan error)
	go func() {
		<-startCh
		ch <- e.Serve(context.Background(), conn)
	}()
	close(startCh)
	select {
	case <-ch:
		t.Errorf("不能这么早返回！应该等待至少1秒。。。")
	case <-time.Tick(1 * time.Second):
		fmt.Println("让 mock 连接闲置 1 秒钟啦，所以不会触发 Serve，也不会有结果发送到 ch 中啦")
		return
	}
}

func TestIdleTimeout03(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	e.options.IdleTimeout = 0
	e.transport = standard.NewTransporter(e.options)
	atomic.StoreUint32(&e.status, statusRunning)
	e.Init()
	atomic.StoreUint32(&e.status, statusRunning)
	e.GET("/foo", func(c context.Context, ctx *app.RequestContext) {
		time.Sleep(50 * time.Millisecond)
		ctx.String(consts.StatusOK, "ok")
	})

	conn := mock.NewConn("GET /foo HTTP/1.1\r\nHost: google.com\r\n\r\n" +
		"GET /foo HTTP/1.1\r\nHost: google.com\r\nConnection: close\r\n\r\n")

	ch := make(chan error)
	startCh := make(chan error)
	go func() {
		<-startCh
		ch <- e.Serve(context.Background(), conn)
	}()
	close(startCh)
	select {
	case err := <-ch:
		if !errors.Is(err, errs.ErrShortConnection) {
			t.Errorf("错误应为 ErrShortConnection，但得到了 %s", err)
		}
	case <-time.Tick(200 * time.Millisecond):
		t.Errorf("超时！应在 200ms 内完成")
	}
}

func TestEngine_Routes(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	e.GET("/", handlerTest1)
	e.GET("/user", handlerTest2)
	e.GET("/user/:name/*action", handlerTest1)
	e.GET("/anonymous1", func(c context.Context, ctx *app.RequestContext) {})
	e.POST("/user", handlerTest2)
	e.POST("/user/:name/*action", handlerTest2)
	e.POST("/anonymous2", func(c context.Context, ctx *app.RequestContext) {})
	group := e.Group("/v1")
	{
		group.GET("/user", handlerTest1)
		group.POST("/login", handlerTest2)
	}
	e.Static("/static", ".")

	list := e.Routes()

	assert.Equal(t, 11, len(list))

	assertRoutePresent(t, list, Route{
		Method:  "GET",
		Path:    "/",
		Handler: "github.com/favbox/wind/route.handlerTest1",
	})
	assertRoutePresent(t, list, Route{
		Method:  "GET",
		Path:    "/user",
		Handler: "github.com/favbox/wind/route.handlerTest2",
	})
	assertRoutePresent(t, list, Route{
		Method:  "GET",
		Path:    "/user/:name/*action",
		Handler: "github.com/favbox/wind/route.handlerTest1",
	})
	assertRoutePresent(t, list, Route{
		Method:  "GET",
		Path:    "/v1/user",
		Handler: "github.com/favbox/wind/route.handlerTest1",
	})
	assertRoutePresent(t, list, Route{
		Method:  "GET",
		Path:    "/static/*filepath",
		Handler: "github.com/favbox/wind/app.(*fsHandler).handleRequest-fm",
	})
	assertRoutePresent(t, list, Route{
		Method:  "GET",
		Path:    "/anonymous1",
		Handler: "github.com/favbox/wind/route.TestEngine_Routes.func1",
	})
	assertRoutePresent(t, list, Route{
		Method:  "POST",
		Path:    "/user",
		Handler: "github.com/favbox/wind/route.handlerTest2",
	})
	assertRoutePresent(t, list, Route{
		Method:  "POST",
		Path:    "/user/:name/*action",
		Handler: "github.com/favbox/wind/route.handlerTest2",
	})
	assertRoutePresent(t, list, Route{
		Method:  "POST",
		Path:    "/anonymous2",
		Handler: "github.com/favbox/wind/route.TestEngine_Routes.func2",
	})
	assertRoutePresent(t, list, Route{
		Method:  "POST",
		Path:    "/v1/login",
		Handler: "github.com/favbox/wind/route.handlerTest2",
	})
	assertRoutePresent(t, list, Route{
		Method:  "HEAD",
		Path:    "/static/*filepath",
		Handler: "github.com/favbox/wind/app.(*fsHandler).handleRequest-fm",
	})
}

func assertRoutePresent(t *testing.T, gets Routes, want Route) {
	for _, get := range gets {
		if get.Path == want.Path && get.Method == want.Method && get.Handler == want.Handler {
			return
		}
	}

	t.Errorf("路由未找到: %v", want)
}

func handlerTest1(c context.Context, ctx *app.RequestContext) {}

func handlerTest2(c context.Context, ctx *app.RequestContext) {}

func TestSetEngineRun(t *testing.T) {
	e := NewEngine(config.NewOptions(nil))
	e.Init()
	assert.True(t, !e.IsRunning())
	e.MarkAsRunning()
	assert.True(t, e.IsRunning())
}

type mockBinder struct{}

func (m *mockBinder) Name() string {
	return "test binder"
}

func (m *mockBinder) Bind(request *protocol.Request, i interface{}, params param.Params) error {
	return nil
}

func (m *mockBinder) BindAndValidate(request *protocol.Request, i interface{}, params param.Params) error {
	return nil
}

func (m *mockBinder) BindQuery(request *protocol.Request, i interface{}) error {
	return nil
}

func (m *mockBinder) BindHeader(request *protocol.Request, i interface{}) error {
	return nil
}

func (m *mockBinder) BindPath(request *protocol.Request, i interface{}, params param.Params) error {
	return nil
}

func (m *mockBinder) BindForm(request *protocol.Request, i interface{}) error {
	return nil
}

func (m *mockBinder) BindJSON(request *protocol.Request, i interface{}) error {
	return nil
}

func (m *mockBinder) BindProtobuf(request *protocol.Request, i interface{}) error {
	return nil
}

type mockValidator struct{}

func (m *mockValidator) ValidateStruct(interface{}) error {
	return fmt.Errorf("test mock")
}

func (m *mockValidator) Engine() interface{} {
	return nil
}

func (m *mockValidator) ValidateTag() string {
	return "vd"
}

type mockNonValidator struct{}

func (m *mockNonValidator) ValidateStruct(interface{}) error {
	return fmt.Errorf("test mock")
}

func TestInitBinderAndValidator(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("意外的恐慌，%v", r)
		}
	}()
	opt := config.NewOptions([]config.Option{})
	bindConfig := binding.NewBindConfig()
	bindConfig.LooseZeroMode = true
	opt.BindConfig = bindConfig
	binder := &mockBinder{}
	opt.CustomBinder = binder
	validator := &mockValidator{}
	opt.CustomValidator = validator
	NewEngine(opt)
}
