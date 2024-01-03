package route

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/favbox/wind/app"
	"github.com/favbox/wind/app/server/binding"
	"github.com/favbox/wind/app/server/render"
	"github.com/favbox/wind/common/config"
	errs "github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/common/tracer"
	"github.com/favbox/wind/common/tracer/stats"
	"github.com/favbox/wind/common/tracer/traceinfo"
	"github.com/favbox/wind/common/utils"
	"github.com/favbox/wind/common/wlog"
	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/internal/bytestr"
	"github.com/favbox/wind/internal/nocopy"
	internalStats "github.com/favbox/wind/internal/stats"
	"github.com/favbox/wind/network"
	"github.com/favbox/wind/network/standard"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/consts"
	"github.com/favbox/wind/protocol/http1"
	"github.com/favbox/wind/protocol/http1/factory"
	"github.com/favbox/wind/protocol/suite"
)

const unknownTransporterName = "unknown"

const (
	_ uint32 = iota
	statusInitialized
	statusRunning
	statusShutdown
	statusClosed
)

var (
	// 默认网络传输器（基于标准库实现，另外可选 netpoll.NewTransporter）
	defaultTransporter = standard.NewTransporter

	errInitFailed       = errs.NewPrivate("路由引擎已经初始化")
	errAlreadyRunning   = errs.NewPrivate("路由引擎已在运行中")
	errStatusNotRunning = errs.NewPrivate("路由引擎未在运行中")

	default404Body = []byte("404 资源未找到")
	default405Body = []byte("405 方法不允许")
	default400Body = []byte("400 错误请求")

	requiredHostBody = []byte("缺少必需的主机标头")
)

type hijackConn struct {
	network.Conn
	e *Engine
}

// CtxCallback 引擎启动时，依次触发的钩子函数
type CtxCallback func(ctx context.Context)

// CtxErrCallback 引擎关闭时，同时触发的钩子函数
type CtxErrCallback func(ctx context.Context) error

// Deprecated: 仅用于获取全局默认传输器 - 可能并非引擎真正使用的。
// 使用 *Engine.GetTransporterName 获取真实使用的传输器。
func GetTransporterName() (tName string) {
	defer func() {
		err := recover()
		if err != nil || tName == "" {
			tName = unknownTransporterName
		}
	}()
	fName := runtime.FuncForPC(reflect.ValueOf(defaultTransporter).Pointer()).Name()
	fSlice := strings.Split(fName, "/")
	name := fSlice[len(fSlice)-1]
	fSlice = strings.Split(name, ".")
	tName = fSlice[0]
	return
}

// SetTransporter 设置全局默认的网络传输器。
func SetTransporter(transporter func(options *config.Options) network.Transporter) {
	defaultTransporter = transporter
}

// NewEngine 创建给定选项的路由引擎。
func NewEngine(opts *config.Options) *Engine {
	engine := &Engine{
		trees: make(MethodTrees, 0, 9),
		RouterGroup: RouterGroup{
			Handlers: nil,
			basePath: opts.BasePath,
			root:     true,
		},
		transport:             defaultTransporter(opts),
		tracerCtl:             &internalStats.Controller{},
		protocolServers:       make(map[string]protocol.Server),
		protocolStreamServers: make(map[string]protocol.StreamServer),
		enableTrace:           true,
		options:               opts,
	}
	engine.initBinderAndValidator(opts)
	if opts.TransporterNewer != nil {
		engine.transport = opts.TransporterNewer(opts)
	}
	engine.RouterGroup.engine = engine

	traceLevel := initTrace(engine)

	// 定义 RequestContext 上下文池的新建函数
	engine.ctxPool.New = func() any {
		ctx := engine.allocateContext()
		if engine.enableTrace {
			ti := traceinfo.NewTraceInfo()
			ti.Stats().SetLevel(traceLevel)
			ctx.SetTraceInfo(ti)
		}
		return ctx
	}

	// 初始化协议组
	engine.protocolSuite = suite.New()

	return engine
}

// Engine 路由引擎，实现路由及协议服务器。
type Engine struct {
	noCopy nocopy.NoCopy

	// 引擎名称
	Name       string
	serverName atomic.Value

	// 路由器和服务器的选项
	options *config.Options

	// 路由
	RouterGroup
	trees MethodTrees

	// 路由当前最大参数个数
	maxParams uint16

	allNoMethod app.HandlersChain // 框架级方法不允许处理器
	allNoRoute  app.HandlersChain // 框架级路由找不到处理器
	noRoute     app.HandlersChain // 用户级路由找不到处理器
	noMethod    app.HandlersChain // 用户级方法不允许处理器

	delims     render.Delims     // HTML 模板的分隔符
	funcMap    template.FuncMap  // HTML 模板的函数映射
	htmlRender render.HTMLRender // HTML 模板的渲染器

	// 是否不用劫持连接池来获取和释放劫持连接？
	//
	// 如果难以保证劫持连接不会被重复关闭，请设置为 true。
	NoHijackConnPool bool
	hijackConnPool   sync.Pool
	// 是否在处理劫持连接后继续保留该链接？
	// 这可节省协程，例如：当 wind 升级 http 连接 为 websocket 且
	// 连接已经转至另一个处理器，该处理器可按需关闭它。
	KeepHijackedConns bool

	// 底层传输的网络库，现有 go net 和 netpoll l两个选择
	transport network.Transporter

	// 链路追踪
	tracerCtl   tracer.Controller
	enableTrace bool

	// 管理协议层不同协议对应的服务器的创建
	protocolSuite         *suite.Config
	protocolServers       map[string]protocol.Server       // 协议与可用的普通服务器实现
	protocolStreamServers map[string]protocol.StreamServer // 协议与可用的流式服务器实现

	// RequestContext 连接池
	ctxPool sync.Pool

	// 处理从 http 处理器中恢复的 panic 的函数。
	// 用于生成错误页并返回 http 错误代码 500（内部服务器错误）。
	// 该处理器可防止服务器因未回复的 panic 而崩溃。
	PanicHandler app.HandlerFunc

	// 在收到 Expect 100 Continue 标头后调用 ContinueHandler。
	// 使用该处理器，服务器可以基于头信息决定是否读取可能较大的请求体。
	//
	// 默认会自动读取请求体，就像普通请求一样。
	ContinueHandler func(header *protocol.RequestHeader) bool

	// 用于表示引擎状态（Init/Running/Shutdown/Closed）。
	status uint32

	// OnRun 是引擎启动时，依次触发的一组钩子函数。
	OnRun []CtxErrCallback

	// OnShutdown 是引擎关闭时，并行触发的一组钩子函数。
	OnShutdown []CtxCallback

	clientIPFunc  app.ClientIP      // 自定义获取客户端 IP 的函数。
	formValueFunc app.FormValueFunc // 自定义获取表单值的函数。

	binder    binding.Binder          // 自定义请求参数绑定器。
	validator binding.StructValidator // 自定义请求参数验证器。
}

// NewContext 创建一个无请求/无响应信息的纯粹上下文。
//
// 注意，在用于处理器之前设置 Request 请求字段。
func (engine *Engine) NewContext() *app.RequestContext {
	return app.NewContext(engine.maxParams)
}

// Run 初始化并由传输器监听连接并提供 Serve 服务。
func (engine *Engine) Run() (err error) {
	// 初始化引擎：加载协议及其服务器实现
	if err = engine.Init(); err != nil {
		return err
	}

	// 切换引擎状态为运行中
	if err = engine.MarkAsRunning(); err != nil {
		return err
	}

	// 返回监听服务出错后，切换引擎转改至已关闭
	defer atomic.SwapUint32(&engine.status, statusClosed)

	// 依次触发可能存在的启动钩子
	ctx := context.Background()
	for i := range engine.OnRun {
		if err = engine.OnRun[i](ctx); err != nil {
			return err
		}
	}

	return engine.listenAndServe()
}

func (engine *Engine) listenAndServe() error {
	wlog.SystemLogger().Infof("使用网络库=%s", engine.GetTransporterName())
	return engine.transport.ListenAndServe(engine.onData)
}

func (engine *Engine) onData(ctx context.Context, conn any) (err error) {
	switch conn := conn.(type) {
	case network.Conn:
		err = engine.Serve(ctx, conn)
	case network.StreamConn:
		err = engine.ServeStream(ctx, conn)
	}
	return
}

// MarkAsRunning 将引擎状态设为“运行中”。
// 警告：除非你知道自己在做什么，否则勿用此法。
func (engine *Engine) MarkAsRunning() error {
	if !atomic.CompareAndSwapUint32(&engine.status, statusInitialized, statusRunning) {
		return errAlreadyRunning
	}
	return nil
}

// Init 初始化可用协议。 默认内置 HTTP1 协议服务器。
func (engine *Engine) Init() error {
	// 默认内置 HTTP1 协议的服务器实现
	if !engine.HasServer(suite.HTTP1) {
		engine.AddProtocol(suite.HTTP1, factory.NewServerFactory(newHttp1OptionFromEngine(engine)))
	}

	// 加载所有可用的服务器协议及其实现
	serverMap, streamServerMap, err := engine.protocolSuite.LoadAll(engine)
	if err != nil {
		return errs.New(err, errs.ErrorTypePrivate, "加载所有协议组错误")
	}
	engine.protocolServers = serverMap
	engine.protocolStreamServers = streamServerMap

	// 若启用 ALPN，则将 HTTP1 作为 TLS 的备用回退协议。
	if engine.alpnEnable() {
		engine.options.TLS.NextProtos = append(engine.options.TLS.NextProtos, suite.HTTP1)
	}

	// 尝试将引擎状态切至已初始化
	if !atomic.CompareAndSwapUint32(&engine.status, 0, statusInitialized) {
		return errInitFailed
	}

	return nil
}

// HasServer 报告是否有给定协议的服务器实现。
func (engine *Engine) HasServer(protocol string) bool {
	return engine.protocolSuite.Get(protocol) != nil
}

// AddProtocol 添加给定协议的服务器工厂。
func (engine *Engine) AddProtocol(protocol string, factory any) {
	engine.protocolSuite.Add(protocol, factory)
}

// SetAltHeader 设置目标协议 targetProtocol 以外协议的 "Alt-Svc" 标头值。
func (engine *Engine) SetAltHeader(targetProtocol, altHeaderValue string) {
	engine.protocolSuite.SetAltHeader(targetProtocol, altHeaderValue)
}

// Shutdown 优雅退出服务器，步骤如下：
//
//  1. 依次触发 Engine.OnShutdown 钩子函数，直至完成或超时；
//  2. 关闭网络监听器，不再接受新连接；
//  3. 等待所有连接关闭：
//     短时可处理的连接处理完就关闭
//     手中请求或下个请求，则等待触达空闲超时(IdleTimeout)或优雅退出超时(ExitWaitTime)。
//  4. 退出
func (engine *Engine) Shutdown(ctx context.Context) (err error) {
	if atomic.LoadUint32(&engine.status) != statusRunning {
		return errStatusNotRunning
	}
	if !atomic.CompareAndSwapUint32(&engine.status, statusRunning, statusShutdown) {
		return
	}

	ch := make(chan struct{})
	// 触发可能的钩子
	go engine.executeOnShutdownHooks(ctx, ch)
	defer func() {
		// 确保钩子执行完成或超时
		select {
		case <-ctx.Done():
			wlog.SystemLogger().Infof("执行 OnShutdownHooks 超时：错误=%v", ctx.Err())
			return
		case <-ch:
			wlog.SystemLogger().Info("执行 OnShutdownHooks 完成")
			return
		}
	}()

	// 注销服务
	if opt := engine.options; opt != nil && opt.Registry != nil {
		if err = opt.Registry.Deregister(opt.RegistryInfo); err != nil {
			wlog.SystemLogger().Errorf("服务注销出错 error=%v", err)
			return err
		}
	}

	// 关闭传输器
	if err := engine.transport.Shutdown(ctx); err != ctx.Err() {
		return err
	}

	return
}

// Close 关闭路由引擎。
//
// 包括传输器及渲染器可能用到的文件监视器。
func (engine *Engine) Close() error {
	if engine.htmlRender != nil {
		engine.htmlRender.Close()
	}
	return engine.transport.Close()
}

// Serve 提供普通连接服务。在可用协议的服务过程中，会自动调用请求服务 ServeHTTP。
func (engine *Engine) Serve(ctx context.Context, conn network.Conn) (err error) {
	defer func() {
		errProcess(conn, err)
	}()

	// H2C 即 HTTP/2 的明文协议，无需使用TLS，常用于开发或测试环境
	if engine.options.H2C {
		// 协议嗅探器
		buf, _ := conn.Peek(len(bytestr.StrClientPreface))
		if bytes.Equal(buf, bytestr.StrClientPreface) && engine.protocolServers[suite.HTTP2] != nil {
			return engine.protocolServers[suite.HTTP2].Serve(ctx, conn)
		}
		wlog.SystemLogger().Warn("HTTP2 服务器未加载，请求正在回退到 HTTP1 服务器")
	}

	// ALPN 协议
	if engine.options.ALPN && engine.options.TLS != nil {
		proto, err1 := engine.getNextProto(conn)
		if err1 != nil {
			// 握手时，客户端关闭了连接，关闭即可。
			if err1 == io.EOF {
				return nil
			}
			// 向 HTTPS 发送 HTTP
			if re, ok := err1.(tls.RecordHeaderError); ok && re.Conn != nil && utils.TLSRecordHeaderLooksLikeHTTP(re.RecordHeader) {
				io.WriteString(re.Conn, "HTTP/1.0 400 Bad Request\r\n\r\n客户端向 HTTPS 服务器发送了 HTTP 请求。\n")
				re.Conn.Close()
				return re
			}
			return err1
		}
		if server, ok := engine.protocolServers[proto]; ok {
			return server.Serve(ctx, conn)
		}
	}

	// HTTP1 协议
	err = engine.protocolServers[suite.HTTP1].Serve(ctx, conn)

	return
}

// ServeStream 提供流式连接服务。在可用协议的服务过程中，会自动调用请求服务 ServeHTTP。
func (engine *Engine) ServeStream(ctx context.Context, conn network.StreamConn) (err error) {
	// ALPN 协议
	if engine.options.ALPN && engine.options.TLS != nil {
		version := conn.GetVersion()
		nextProtocol := versionToALPN(version)
		if server, ok := engine.protocolStreamServers[nextProtocol]; ok {
			return server.Serve(ctx, conn)
		}
	}

	// 默认协议 HTTP3
	if server, ok := engine.protocolStreamServers[suite.HTTP3]; ok {
		return server.Serve(ctx, conn)
	}

	return errs.ErrNotSupportProtocol
}

func (engine *Engine) initBinderAndValidator(opt *config.Options) {
	// 初始化请求参数验证器
	if opt.CustomValidator != nil {
		customValidator, ok := opt.CustomValidator.(binding.StructValidator)
		if !ok {
			panic("自定义验证器未实现 binding.StructValidator 接口")
		}
		engine.validator = customValidator
	} else {
		engine.validator = binding.NewValidator(binding.NewValidateConfig())
		if opt.ValidateConfig != nil {
			vConf, ok := opt.ValidateConfig.(*binding.ValidateConfig)
			if !ok {
				panic("opt.ValidateConfig 不是 '*binding.ValidateConfig' 类型")
			}
			engine.validator = binding.NewValidator(vConf)
		}
	}

	// 初始化请求参数绑定器
	if opt.CustomBinder != nil {
		customBinder, ok := opt.CustomBinder.(binding.Binder)
		if !ok {
			panic("自定义绑定器未实现 binding.Binder 接口")
		}
		engine.binder = customBinder
		return
	}

	// 初始化绑定器。由于存在 "BindAndValidate" 接口，此处需注入 Validator。
	defaultBindConfig := binding.NewBindConfig()
	defaultBindConfig.Validator = engine.validator
	engine.binder = binding.NewBinder(defaultBindConfig)
	if opt.BindConfig != nil {
		bConf, ok := opt.BindConfig.(*binding.BindConfig)
		if !ok {
			panic("opt.BindConfig 不是 '*binding.BindConfig' 类型")
		}
		if bConf.Validator == nil {
			bConf.Validator = engine.validator
		}
		engine.binder = binding.NewBinder(bConf)
	}
}

// ↓ ↓ ↓ ↓ ↓ suite.Core 接口的具体实现  ↓ ↓ ↓ ↓ ↓

// IsRunning 报告引擎是否正在运行。
func (engine *Engine) IsRunning() bool {
	return atomic.LoadUint32(&engine.status) == statusRunning
}

// GetCtxPool 返回引擎的请求上下文池子。
func (engine *Engine) GetCtxPool() *sync.Pool {
	return &engine.ctxPool
}

// ServeHTTP 提供请求服务。在服务过程中，会自动调用用户扩展的 app.HandlerFunc。
func (engine *Engine) ServeHTTP(c context.Context, ctx *app.RequestContext) {
	ctx.SetBinder(engine.binder)
	ctx.SetValidator(engine.validator)
	if engine.PanicHandler != nil {
		defer engine.recover(ctx)
	}

	rPath := string(ctx.Request.URI().Path())

	// 对齐 https://datatracker.ietf.org/doc/html/rfc2616#section-5.2
	if len(ctx.Request.Host()) == 0 && ctx.Request.Header.IsHTTP11() && bytesconv.B2s(ctx.Request.Method()) != consts.MethodConnect {
		serveError(c, ctx, consts.StatusBadRequest, requiredHostBody)
		return
	}

	httpMethod := bytesconv.B2s(ctx.Request.Header.Method())
	unescape := false
	if engine.options.UseRawPath {
		rPath = string(ctx.Request.URI().PathOriginal())
		unescape = engine.options.UnescapePathValues
	}

	if engine.options.RemoveExtraSlash {
		rPath = utils.CleanPath(rPath)
	}

	// 若路由路径为空或未以 '/' 开头，需遵循 RFC7230#section-5.3
	if rPath == "" || rPath[0] != '/' {
		serveError(c, ctx, consts.StatusBadRequest, default400Body)
		return
	}

	// 若路由方法存在，则通过 Next 调用处理链
	t := engine.trees
	paramsPointer := &ctx.Params
	for i, tl := 0, len(t); i < tl; i++ {
		if t[i].method != httpMethod {
			continue
		}
		// 在树中查找路由
		value := t[i].find(rPath, paramsPointer, unescape)

		if value.handlers != nil {
			ctx.SetHandlers(value.handlers)
			ctx.SetFullPath(value.fullPath)
			ctx.Next(c)
			return
		}
		if httpMethod != consts.MethodConnect && rPath != "/" {
			if value.tsr && engine.options.RedirectTrailingSlash {
				redirectTrailingSlash(ctx)
				return
			}
			if engine.options.RedirectFixedPath && redirectFixedPath(ctx, t[i].root, engine.options.RedirectFixedPath) {
				return
			}
		}
		break
	}

	// 若方法不允许，则尝试替代方法的处理链
	if engine.options.HandleMethodNotAllowed {
		for _, tree := range engine.trees {
			if tree.method == httpMethod {
				continue
			}
			if value := tree.find(rPath, paramsPointer, unescape); value.handlers != nil {
				ctx.SetHandlers(engine.allNoMethod)
				serveError(c, ctx, consts.StatusMethodNotAllowed, default405Body)
				return
			}
		}
	}

	// 请求至此，说明无用户处理器则用
	ctx.SetHandlers(engine.allNoRoute)

	// 然后处理 404 错误的路由
	serveError(c, ctx, consts.StatusNotFound, default404Body)
}

// GetTracer 获取链路跟踪控制器。
func (engine *Engine) GetTracer() tracer.Controller {
	return engine.tracerCtl
}

// ↑ ↑ ↑ ↑ ↑ suite.Core 接口的具体实现  ↑ ↑ ↑ ↑ ↑

// Use 添加全局中间件。
//
// 将中间件包含在每个请求的处理链中，甚至 404, 405, 静态文件...
//
// 常用场景：日志记录、错误管理等。
func (engine *Engine) Use(middleware ...app.HandlerFunc) Router {
	engine.RouterGroup.Use(middleware...)
	engine.rebuild404Handlers()
	engine.rebuild405Handlers()
	return engine
}

// GetOptions 返回路由器和协议服务器的配置项。
func (engine *Engine) GetOptions() *config.Options {
	return engine.options
}

// GetServerName 获取服务器名称。
func (engine *Engine) GetServerName() []byte {
	v := engine.serverName.Load()
	var serverName []byte
	if v == nil {
		serverName = []byte(engine.Name)
		if len(serverName) == 0 {
			serverName = bytestr.DefaultServerName
		}
		engine.serverName.Store(serverName)
	} else {
		serverName = v.([]byte)
	}
	return serverName
}

// GetTransporterName 获取底层网络传输器的名称。
func (engine *Engine) GetTransporterName() string {
	return getTransporterName(engine.transport)
}

// IsStreamRequestBody 是否流式处理请求体？
func (engine *Engine) IsStreamRequestBody() bool {
	return engine.options.StreamRequestBody
}

// IsTraceEnable 是否启用链路跟踪？
func (engine *Engine) IsTraceEnable() bool {
	return engine.enableTrace
}

// NoRoute 设置 404 请求方法未找到时对应的处理链，默认返回 404 状态码。
func (engine *Engine) NoRoute(handlers ...app.HandlerFunc) {
	engine.noRoute = handlers
	engine.rebuild404Handlers()
}

// NoMethod 设置405请求方法不允许时对应的处理链。
func (engine *Engine) NoMethod(handlers ...app.HandlerFunc) {
	engine.noMethod = handlers
	engine.rebuild405Handlers()
}

// PrintRoute 递归打印给定方法的路由节点信息。
func (engine *Engine) PrintRoute(method string) {
	root := engine.trees.get(method)
	printNode(root.root, 0)
}

// Routes 返回已注册的路由切片，及关键信息，如： HTTP 方法、路径和处理器名称。
func (engine *Engine) Routes() (routes Routes) {
	for _, tree := range engine.trees {
		routes = iterate(tree.method, routes, tree.root)
	}
	return routes
}

// Delims 设置 HTML 模板的左右分隔符并返回引擎。
func (engine *Engine) Delims(left, right string) *Engine {
	engine.delims = render.Delims{
		Left:  left,
		Right: right,
	}
	return engine
}

// LoadHTMLFiles 加载一组 HTML 文件，并关联到 HTML 渲染器。
func (engine *Engine) LoadHTMLFiles(files ...string) {
	tmpl := template.Must(template.New("").
		Delims(engine.delims.Left, engine.delims.Right).
		Funcs(engine.funcMap).
		ParseFiles(files...))

	if engine.options.AutoReloadRender {
		engine.SetAutoReloadHTMLTemplate(tmpl, files)
		return
	}

	engine.SetHTMLTemplate(tmpl)
}

// LoadHTMLGlob 加载给定 pattern 模式的 HTML 文件，并关联到 HTML 渲染器。
func (engine *Engine) LoadHTMLGlob(pattern string) {
	tmpl := template.Must(template.New("").
		Delims(engine.delims.Left, engine.delims.Right).
		Funcs(engine.funcMap).
		ParseGlob(pattern))

	if engine.options.AutoReloadRender {
		files, err := filepath.Glob(pattern)
		if err != nil {
			wlog.SystemLogger().Errorf("LoadHTMLGlob: %v", err)
			return
		}
		engine.SetAutoReloadHTMLTemplate(tmpl, files)
		return
	}

	engine.SetHTMLTemplate(tmpl)
}

// SetAutoReloadHTMLTemplate 关联模板与调试环境的 HTML 模板渲染器。
func (engine *Engine) SetAutoReloadHTMLTemplate(tmpl *template.Template, files []string) {
	engine.htmlRender = &render.HTMLDebug{
		Template:        tmpl,
		Files:           files,
		FuncMap:         engine.funcMap,
		Delims:          engine.delims,
		RefreshInterval: engine.options.AutoReloadInterval,
	}
}

// SetHTMLTemplate 关联模板与生产环境的 HTML 渲染器。
func (engine *Engine) SetHTMLTemplate(tmpl *template.Template) {
	engine.htmlRender = render.HTMLProduction{
		Template: tmpl.Funcs(engine.funcMap),
	}
}

// SetFuncMap 设置用于 template.FuncMap 的模板函数映射。
func (engine *Engine) SetFuncMap(funcMap template.FuncMap) {
	engine.funcMap = funcMap
}

// SetClientIPFunc 设置获取客户端 IP 的自定义函数。
func (engine *Engine) SetClientIPFunc(f app.ClientIP) {
	engine.clientIPFunc = f
}

// SetFormValueFunc 设置获取表单值的自定义函数。
func (engine *Engine) SetFormValueFunc(f app.FormValueFunc) {
	engine.formValueFunc = f
}

// HijackConnHandle 处理给定的劫持连接。
func (engine *Engine) HijackConnHandle(c network.Conn, h app.HijackHandler) {
	engine.hijackConnHandle(c, h)
}

func (engine *Engine) addRoute(method, path string, handlers app.HandlersChain) {
	if len(path) == 0 {
		panic("路径不能为空")
	}
	utils.Assert(path[0] == '/', "路径必须以 / 开头")
	utils.Assert(method != "", "HTTP 方法不能为空")
	utils.Assert(len(handlers) > 0, "至少要对应一个处理器")

	if !engine.options.DisablePrintRoute {
		debugPrintRoute(method, path, handlers)
	}

	methodRouter := engine.trees.get(method)
	if methodRouter == nil {
		methodRouter = &router{
			method:        method,
			root:          &node{},
			hasTsrHandler: make(map[string]bool),
		}
		engine.trees = append(engine.trees, methodRouter)
	}
	methodRouter.addRoute(path, handlers)

	// 更新 maxParams
	if paramsCount := countParams(path); paramsCount > engine.maxParams {
		engine.maxParams = paramsCount
	}
}

// 汇报是否启用了 ALPN 以获取备用的回退协议。
func (engine *Engine) alpnEnable() bool {
	return engine.options.TLS != nil && engine.options.ALPN
}

// 分配一个新的请求上下文，并设定最大保留字节数、获取客户端 IP 和表单值的自定义函数。
func (engine *Engine) allocateContext() *app.RequestContext {
	ctx := engine.NewContext()
	ctx.Request.SetMaxKeepBodySize(engine.options.MaxKeepBodySize)
	ctx.Response.SetMaxKeepBodySize(engine.options.MaxKeepBodySize)
	ctx.SetClientIPFunc(engine.clientIPFunc)
	ctx.SetFormValueFunc(engine.formValueFunc)
	return ctx
}

// 获取 TLS 连接的下一个协商协议。
func (engine *Engine) getNextProto(conn network.Conn) (proto string, err error) {
	if tlsConn, ok := conn.(network.ConnTLSer); ok {
		if engine.options.ReadTimeout > 0 {
			if err := conn.SetReadTimeout(engine.options.ReadTimeout); err != nil {
				wlog.SystemLogger().Errorf("BUG: 设置连接的读取超时时长=%s 错误=%s", engine.options.ReadTimeout, err)
			}
		}
		err = tlsConn.Handshake()
		if err == nil {
			proto = tlsConn.ConnectionState().NegotiatedProtocol
		}
	}
	return
}

// 处理恐慌。
func (engine *Engine) recover(ctx *app.RequestContext) {
	if r := recover(); r != nil {
		engine.PanicHandler(context.Background(), ctx)
	}
}

// 处理劫持连接。
func (engine *Engine) hijackConnHandle(c network.Conn, h app.HijackHandler) {
	hjc := engine.acquireHijackConn(c)
	h(hjc)

	if !engine.KeepHijackedConns {
		c.Close()
		engine.releaseHijackConn(hjc)
	}
}

// 获取劫持连接。
func (engine *Engine) acquireHijackConn(c network.Conn) *hijackConn {
	// 不用劫持连接池
	if engine.NoHijackConnPool {
		return &hijackConn{
			Conn: c,
			e:    engine,
		}
	}

	// 用连接池
	v := engine.hijackConnPool.Get()

	// 但是还没有可用实例，返回一个新实例
	if v == nil {
		return &hijackConn{
			Conn: c,
			e:    engine,
		}
	}

	// 池中有可用实例，则更新连接
	hjc := v.(*hijackConn)
	hjc.Conn = c
	return hjc
}

// 释放劫持连接。
func (engine *Engine) releaseHijackConn(hjc *hijackConn) {
	if engine.NoHijackConnPool {
		return
	}
	hjc.Conn = nil
	engine.hijackConnPool.Put(hjc)
}

// 重建 404 方法未找到处理器。
func (engine *Engine) rebuild404Handlers() {
	engine.allNoRoute = engine.combineHandlers(engine.noRoute)
}

// 重建 405 方法不允许处理器。
func (engine *Engine) rebuild405Handlers() {
	engine.allNoMethod = engine.combineHandlers(engine.noMethod)
}

// 执行引擎退出的回调钩子。
func (engine *Engine) executeOnShutdownHooks(ctx context.Context, ch chan struct{}) {
	wg := sync.WaitGroup{}
	for i := range engine.OnShutdown {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			engine.OnShutdown[index](ctx)
		}(i)
	}
	wg.Wait()
	ch <- struct{}{}
}

func newHttp1OptionFromEngine(engine *Engine) *http1.Option {
	opt := &http1.Option{
		StreamRequestBody:             engine.options.StreamRequestBody,
		GetOnly:                       engine.options.GetOnly,
		DisablePreParseMultipartForm:  engine.options.DisablePreParseMultipartForm,
		DisableKeepalive:              engine.options.DisableKeepalive,
		NoDefaultServerHeader:         engine.options.NoDefaultServerHeader,
		MaxRequestBodySize:            engine.options.MaxRequestBodySize,
		IdleTimeout:                   engine.options.IdleTimeout,
		ReadTimeout:                   engine.options.ReadTimeout,
		ServerName:                    engine.GetServerName(),
		TLS:                           engine.options.TLS,
		EnableTrace:                   engine.IsTraceEnable(),
		HTMLRender:                    engine.htmlRender,
		ContinueHandler:               engine.ContinueHandler,
		HijackConnHandle:              engine.HijackConnHandle,
		DisableHeaderNamesNormalizing: engine.options.DisableHeaderNamesNormalizing,
		NoDefaultDate:                 engine.options.NoDefaultDate,
		NoDefaultContentType:          engine.options.NoDefaultContentType,
	}
	// 标准库的空闲超时必不能为零，若为 0 则置为 -1。
	// 由于网络库的触发方式不同，具体原因请参阅该值的实际使用情况。
	if opt.IdleTimeout == 0 && engine.GetTransporterName() == "standard" {
		opt.IdleTimeout = -1
	}
	return opt
}

func initTrace(engine *Engine) stats.Level {
	for _, t := range engine.options.Tracers {
		if col, ok := t.(tracer.Tracer); ok {
			engine.tracerCtl.Append(col)
		}
	}

	if !engine.tracerCtl.HasTracer() {
		engine.enableTrace = false
	}

	traceLevel := stats.LevelDetailed
	if tl, ok := engine.options.TraceLevel.(stats.Level); ok {
		traceLevel = tl
	}
	return traceLevel
}

func debugPrintRoute(httpMethod, absolutePath string, handlers app.HandlersChain) {
	nHandlers := len(handlers)
	handlerName := app.GetHandlerName(handlers.Last())
	if handlerName == "" {
		handlerName = utils.NameOfFunction(handlers.Last())
	}
	wlog.SystemLogger().Debugf("方法=%-6s 绝对路径=%-25s --> 处理器名称=%s (%d 个处理器)", httpMethod, absolutePath, handlerName, nHandlers)
}

func getTransporterName(transporter network.Transporter) (tName string) {
	defer func() {
		err := recover()
		if err != nil || tName == "" {
			tName = unknownTransporterName
		}
	}()
	t := reflect.ValueOf(transporter).Type().String()
	tName = strings.Split(strings.TrimPrefix(t, "*"), ".")[0]
	return tName
}

func iterate(method string, routes Routes, root *node) Routes {
	if len(root.handlers) > 0 {
		handlerFunc := root.handlers.Last()
		routes = append(routes, Route{
			Method:      method,
			Path:        root.ppath,
			Handler:     utils.NameOfFunction(handlerFunc),
			HandlerFunc: handlerFunc,
		})
	}

	for _, child := range root.children {
		routes = iterate(method, routes, child)
	}

	if root.paramChild != nil {
		routes = iterate(method, routes, root.paramChild)
	}

	if root.anyChild != nil {
		routes = iterate(method, routes, root.anyChild)
	}

	return routes
}

func printNode(node *node, level int) {
	fmt.Println("node.prefix: " + node.prefix)
	fmt.Println("node.ppath: " + node.ppath)
	fmt.Printf("level: %#v\n\n", level)
	for i := 0; i < len(node.children); i++ {
		printNode(node.children[i], level+1)
	}
}

func redirectFixedPath(ctx *app.RequestContext, root *node, fixTrailingSlash bool) bool {
	rPath := bytesconv.B2s(ctx.Request.URI().Path())
	if fixedPath, ok := root.findCaseInsensitivePath(utils.CleanPath(rPath), fixTrailingSlash); ok {
		ctx.Request.SetRequestURI(bytesconv.B2s(fixedPath))
		redirectRequest(ctx)
		return true
	}
	return false
}

func redirectTrailingSlash(ctx *app.RequestContext) {
	p := bytesconv.B2s(ctx.Request.URI().Path())
	if prefix := utils.CleanPath(bytesconv.B2s(ctx.Request.Header.Peek("X-Forwarded-Prefix"))); prefix != "." {
		p = prefix + "/" + p
	}

	tmpURI := trailingSlashURL(p)

	query := ctx.Request.URI().QueryString()

	if len(query) > 0 {
		tmpURI = tmpURI + "?" + bytesconv.B2s(query)
	}

	ctx.Request.SetRequestURI(tmpURI)
	redirectRequest(ctx)
}

func redirectRequest(ctx *app.RequestContext) {
	code := consts.StatusMovedPermanently // 永久跳转，GET 请求
	if bytesconv.B2s(ctx.Request.Header.Method()) != consts.MethodGet {
		code = consts.StatusTemporaryRedirect
	}

	ctx.Redirect(code, ctx.Request.URI().RequestURI())
}

func trailingSlashURL(ts string) string {
	tmpURI := ts + "/"
	if length := len(ts); length > 1 && ts[length-1] == '/' {
		tmpURI = ts[:length-1]
	}
	return tmpURI
}

func serveError(c context.Context, ctx *app.RequestContext, code int, defaultMessage []byte) {
	ctx.SetStatusCode(code)
	ctx.Next(c) // TODO 无此路由为啥还继续 Next?
	if ctx.Response.StatusCode() == code {
		// 若正文存在（或由用户定制），别管他。
		if ctx.Response.HasBodyBytes() || ctx.Response.IsBodyStream() {
			return
		}
		ctx.Response.Header.Set("Content-Type", "text/plain; charset=utf-8")
		ctx.Response.SetBody(defaultMessage)
	}
}

func errProcess(conn io.Closer, err error) {
	if err == nil {
		return
	}

	defer func() {
		if err != nil {
			conn.Close()
		}
	}()

	// 静默关闭连接
	if errors.Is(err, errs.ErrShortConnection) || errors.Is(err, errs.ErrIdleTimeout) {
		return
	}

	// 不处理劫持连接的错误
	if errors.Is(err, errs.ErrHijacked) {
		err = nil
		return
	}

	// 获取供外部使用的远程地址
	rip := getRemoteAddrFromCloser(conn)

	// 处理特定错误
	if hse, ok := conn.(network.HandleSpecificError); ok {
		if hse.HandleSpecificError(err, rip) {
			return
		}
	}

	// 处理其他错误
	wlog.SystemLogger().Errorf(wlog.EngineErrorFormat, err.Error(), rip)
}

func getRemoteAddrFromCloser(conn io.Closer) string {
	if c, ok := conn.(network.Conn); ok {
		if addr := c.RemoteAddr(); addr != nil {
			return addr.String()
		}
	}
	return ""
}

func versionToALPN(v uint32) string {
	if v == network.Version1 || v == network.Version2 {
		return suite.HTTP3
	}
	if v == network.VersionTLS || v == network.VersionDraft29 {
		return suite.HTTP3Draft29
	}
	return ""
}
