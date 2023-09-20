package app

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/favbox/wind/app/server/binding"
	"github.com/favbox/wind/app/server/render"
	"github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/common/tracer/traceinfo"
	"github.com/favbox/wind/common/utils"
	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/internal/bytestr"
	"github.com/favbox/wind/network"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/consts"
	rConsts "github.com/favbox/wind/route/consts"
	"github.com/favbox/wind/route/param"
)

var zeroTCPAddr = &net.TCPAddr{IP: net.IPv4zero}

type Handler interface {
	ServeHTTP()
}

// RequestContext 表示一个请求上下文。
type RequestContext struct {
	conn     network.Conn
	Request  protocol.Request
	Response protocol.Response

	// 是附加到所有使用该上下文的处理器/中间件的错误列表。
	Errors errors.ErrorChain

	Params     param.Params      // 路由参数切片
	fullPath   string            // 完整请求路径
	handlers   HandlersChain     // 上下文的处理链
	index      int8              // 处理链的当前索引
	HTMLRender render.HTMLRender //  HTML 渲染器

	mu   sync.RWMutex   // 上下文键值对的互斥保护锁
	Keys map[string]any // 上下文键值对

	hijackHandler HijackHandler // 劫持连接的处理器

	finishedMu sync.Mutex    // 请求结束互斥锁
	finished   chan struct{} // 请求是否结束的信道

	// 跟踪信息
	traceInfo traceinfo.TraceInfo

	// 是否启用跟踪
	enableTrace bool

	// 通过自定义函数获取客户端 IP
	clientIPFunc ClientIP

	// 通过自定义函数获取表单值
	formValueFunc FormValueFunc
}

// NewContext 创建一个指定初始最大路由参数的无请求/响应信息的纯粹上下文。
func NewContext(maxParams uint16) *RequestContext {
	v := make(param.Params, 0, maxParams)
	ctx := &RequestContext{Params: v, index: -1}
	return ctx
}

// Abort 中止处理，并防止调用挂起的处理器。
//
// 注意该函数不会停止当前处理器。
// 假设身份鉴权中间件鉴权失败（如密码不匹配），调用 Abort 可确保该请求的后续处理器不被调用。
func (ctx *RequestContext) Abort() {
	ctx.index = rConsts.AbortIndex
}

// AbortWithStatus 设置状态码并中止处理。
//
// 例如，对于身份鉴权失败的请求可使用：ctx.AbortWithStatus(401)
func (ctx *RequestContext) AbortWithStatus(code int) {
	ctx.SetStatusCode(code)
	ctx.Abort()
}

// AbortWithStatusJSON 内部调用 Abort 和 JSON。
//
// 此方法停止处理链，写入状态代码，并返回 JSON 正文。
// 也会自动设置 Content-Type 为 "application/json"。
func (ctx *RequestContext) AbortWithStatusJSON(code int, jsonObj any) {
	ctx.Abort()
	ctx.JSON(code, jsonObj)
}

// AbortWithMsg 设置响应体和状态码，并中止响应。
//
// 警告：将重置先前的响应。
func (ctx *RequestContext) AbortWithMsg(msg string, statusCode int) {
	ctx.Response.Reset()
	ctx.SetStatusCode(statusCode)
	ctx.SetContentTypeBytes(bytestr.DefaultContentType)
	ctx.SetBodyString(msg)
	ctx.Abort()
}

// AbortWithError 内部调用 AbortWithStatus 和 Error。
//
// 此方法停止处理链，写入状态代码，并将指定的错误推送到 Errors。
// 查看 Error 了解更多详情。
func (ctx *RequestContext) AbortWithError(code int, err error) *errors.Error {
	ctx.AbortWithStatus(code)
	return ctx.Error(err)
}

// IsAborted 汇报当前上下文是否已被中止。
func (ctx *RequestContext) IsAborted() bool {
	return ctx.index >= rConsts.AbortIndex
}

// Error 附加一个错误到当前上下文的错误列表。
//
// 建议请求处理过程中的每个错误都要调用 Error 进行记录。
// 中间件可用于收集所有错误并将它们一起推送至数据库、打印日志或追加至 HTTP 响应。
//
// Error 会在 err 为空时触发恐慌。
func (ctx *RequestContext) Error(err error) *errors.Error {
	if err == nil {
		panic("err 不可为空")
	}

	parsedErr, ok := err.(*errors.Error)
	if !ok {
		parsedErr = &errors.Error{
			Err:  err,
			Type: errors.ErrorTypePrivate,
		}
	}

	ctx.Errors = append(ctx.Errors, parsedErr)
	return parsedErr
}

// File 将给定的 filepath 高效写入响应的正文流。
func (ctx *RequestContext) File(filepath string) {
	ServeFile(ctx, filepath)
}

// FileFromFS 将给定的 filepath 以给定的配置 fs，高效写入到响应的正文流。
func (ctx *RequestContext) FileFromFS(filepath string, fs *FS) {
	defer func(old string) {
		ctx.Request.URI().SetPath(old)
	}(string(ctx.Request.URI().Path()))

	ctx.Request.URI().SetPath(filepath)

	fs.NewRequestHandler()(context.Background(), ctx)
}

// FileAttachment 将给定的 filepath 高效写入响应的正文流。
//
// 当客户端下载该文件，将会以给定的 filename 重命名。
func (ctx *RequestContext) FileAttachment(filepath, filename string) {
	ctx.Response.Header.Set("content-disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	ServeFile(ctx, filepath)
}

// URI 返回请求的完整网址。
func (ctx *RequestContext) URI() *protocol.URI {
	return ctx.Request.URI()
}

// Path 返回请求的路径。
func (ctx *RequestContext) Path() []byte {
	return ctx.URI().Path()
}

// SetStatusCode 设置响应的状态码。
func (ctx *RequestContext) SetStatusCode(statusCode int) {
	ctx.Response.SetStatusCode(statusCode)
}

// SetContentType 设置响应的内容类型标头值。
func (ctx *RequestContext) SetContentType(contentType string) {
	ctx.Response.Header.SetContentType(contentType)
}

// SetContentTypeBytes 设置响应的内容类型标头值。
func (ctx *RequestContext) SetContentTypeBytes(contentType []byte) {
	ctx.Response.Header.SetContentTypeBytes(contentType)
}

// SetBodyStream 设置响应的正文流和大小（可选）。
func (ctx *RequestContext) SetBodyStream(bodyStream io.Reader, bodySize int) {
	ctx.Response.SetBodyStream(bodyStream, bodySize)
}

// SetBodyString 设置响应的主体。
func (ctx *RequestContext) SetBodyString(body string) {
	ctx.Response.SetBodyString(body)
}

// HijackHandler 被劫持连接的处理函数。
type HijackHandler func(c network.Conn)

// Hijack 注册被劫持连接的处理器。
func (ctx *RequestContext) Hijack(handler HijackHandler) {
	ctx.hijackHandler = handler
}

// Hijacked 报告是否已调用 Hijack。
func (ctx *RequestContext) Hijacked() bool {
	return ctx.hijackHandler != nil
}

// IfModifiedSince 如果 lastModified 超过请求标头中的 'If-Modified-Since' 值，则返回真。
//
// 若无此标头或日期解析失败也返回真。
func (ctx *RequestContext) IfModifiedSince(lastModified time.Time) bool {
	ifModStr := ctx.Request.Header.PeekIfModifiedSinceBytes()
	if len(ifModStr) == 0 {
		return true
	}
	ifMod, err := bytesconv.ParseHTTPDate(ifModStr)
	if err != nil {
		return true
	}
	lastModified = lastModified.Truncate(time.Second)
	return ifMod.Before(lastModified)
}

// NotModified 重置响应并将响应的状态码设置为 '304 Not Modified'。
func (ctx *RequestContext) NotModified() {
	ctx.Response.Reset()
	ctx.SetStatusCode(consts.StatusNotModified)
}

// NotFound 重置响应并将响应的状态码设置为 '404 Not Found'。
func (ctx *RequestContext) NotFound() {
	ctx.Response.Reset()
	ctx.SetStatusCode(consts.StatusNotFound)
	ctx.SetBodyString(consts.StatusMessage(consts.StatusNotFound))
}

// IsHead 是否为 HEAD 请求？
func (ctx *RequestContext) IsHead() bool {
	return ctx.Request.Header.IsHead()
}

// IsGet 是否为 GET 请求？
func (ctx *RequestContext) IsGet() bool {
	return ctx.Request.Header.IsGet()
}

// IsPost 是否为 POST 请求？
func (ctx *RequestContext) IsPost() bool {
	return ctx.Request.Header.IsPost()
}

// Method 返回请求的方法。
func (ctx *RequestContext) Method() []byte {
	return ctx.Request.Header.Method()
}

// Host 获取请求的主机地址。
func (ctx *RequestContext) Host() []byte {
	return ctx.URI().Host()
}

// WriteString 附加 s 到响应的主体。
func (ctx *RequestContext) WriteString(s string) (int, error) {
	ctx.Response.AppendBodyString(s)
	return len(s), nil
}

func (ctx *RequestContext) GetTraceInfo() traceinfo.TraceInfo {
	return ctx.traceInfo
}

func (ctx *RequestContext) SetTraceInfo(t traceinfo.TraceInfo) {
	ctx.traceInfo = t
}

// IsEnableTrace 汇报是否启用了链路跟踪。
func (ctx *RequestContext) IsEnableTrace() bool {
	return ctx.enableTrace
}

// SetEnableTrace 设置是否启用跟踪。
//
// 注意：业务处理程序不可修改此值，否则可能引起恐慌。
func (ctx *RequestContext) SetEnableTrace(enable bool) {
	ctx.enableTrace = enable
}

// SetClientIPFunc 设置获取客户端 IP 的自定义函数。
func (ctx *RequestContext) SetClientIPFunc(fn ClientIP) {
	ctx.clientIPFunc = fn
}

// SetFormValueFunc 设置获取表单值的自定义函数。
func (ctx *RequestContext) SetFormValueFunc(f FormValueFunc) {
	ctx.formValueFunc = f
}

// QueryArgs 返回请求 URL 中的查询参数。
//
// 不会返回 POST 请求的参数 - 请使用 PostArgs()。
// 其他参数请看 PostArgs, FormValue and FormFile。
func (ctx *RequestContext) QueryArgs() *protocol.Args {
	return ctx.URI().QueryArgs()
}

// PostArgs 返回 POST 参数。
func (ctx *RequestContext) PostArgs() *protocol.Args {
	return ctx.Request.PostArgs()
}

// FormFile 返回表单中指定 name 的第一个文件头。
func (ctx *RequestContext) FormFile(name string) (*multipart.FileHeader, error) {
	return ctx.Request.FormFile(name)
}

// FormValue 获取给定表单字段 key 的值。
//
// 查找位置：
//
//   - 查询字符串参数 QueryArgs
//   - POST 或 PUT 正文参数 PostArgs
//   - 表单数据 MultipartForm
//
// 还有更多细粒度的方法可获取表单值：
//
//   - QueryArgs 用于获取查询字符串中的值。
//   - PostArgs 用于获取 POST 或 PUT 正文的值。
//   - MultipartForm 用于获取多部分表单的值。
//   - FormFile 用于获取上传的文件。
//
// 通过 engine.SetCustomFormValueFunc 可改变 FormValue 的取值行为。
func (ctx *RequestContext) FormValue(key string) []byte {
	if ctx.formValueFunc != nil {
		return ctx.formValueFunc(ctx, key)
	}
	return defaultFormValue(ctx, key)
}

// MultipartForm 返回请求的多部分表单。
//
// 若请求的内容类型不是 'multipart/form-data' 则返回 errors.ErrNoMultipartForm。
//
// 所有上传的临时文件都将被自动删除。如果你想保留上传的文件，可移动或复制到新位置。
//
// 使用 SaveMultipartFile 可持久化保存上传的文件。
//
// 另见 FormFile and FormValue.
func (ctx *RequestContext) MultipartForm() (*multipart.Form, error) {
	return ctx.Request.MultipartForm()
}

// SaveUploadedFile 上传表单文件到指定位置。
func (ctx *RequestContext) SaveUploadedFile(file *multipart.FileHeader, dst string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

// Reset 重设请求上下文。
//
// 注意：这是一个内部函数。你不应该使用它。
func (ctx *RequestContext) Reset() {
	ctx.ResetWithoutConn()
	ctx.conn = nil
}

// ResetWithoutConn 重置请求信息（连接除外）。
func (ctx *RequestContext) ResetWithoutConn() {
	ctx.Params = ctx.Params[0:0]
	ctx.Errors = ctx.Errors[0:0]
	ctx.handlers = nil
	ctx.index = -1
	ctx.fullPath = ""
	ctx.Keys = nil

	if ctx.finished != nil {
		close(ctx.finished)
		ctx.finished = nil
	}

	ctx.Request.ResetWithoutConn()
	ctx.Response.Reset()
	if ctx.IsEnableTrace() {
		ctx.traceInfo.Reset()
	}
}

func (ctx *RequestContext) SetConn(c network.Conn) {
	ctx.conn = c
}

func (ctx *RequestContext) GetConn() network.Conn {
	return ctx.conn
}

func (ctx *RequestContext) GetReader() network.Reader {
	return ctx.conn
}

// SetConnectionClose 设置 'Connection: close' 响应头。
func (ctx *RequestContext) SetConnectionClose() {
	ctx.Response.SetConnectionClose()
}

// GetWriter 获取网络写入器。
func (ctx *RequestContext) GetWriter() network.Writer {
	return ctx.conn
}

// Body 返回请求的正文字节。
func (ctx *RequestContext) Body() ([]byte, error) {
	return ctx.Request.BodyE()
}

// GetRawData 返回请求的正文字节。
func (ctx *RequestContext) GetRawData() []byte {
	return ctx.Request.Body()
}

// GetIndex 获取处理链的当前索引。
func (ctx *RequestContext) GetIndex() int8 {
	return ctx.index
}

// GetHijackHandler 获取被劫持的连接的处理器。
func (ctx *RequestContext) GetHijackHandler() HijackHandler {
	return ctx.hijackHandler
}

// SetHijackHandler 设置被劫持的连接的处理器。
func (ctx *RequestContext) SetHijackHandler(h HijackHandler) {
	ctx.hijackHandler = h
}

// RequestBodyStream 返回请求的正文流。
func (ctx *RequestContext) RequestBodyStream() io.Reader {
	return ctx.Request.BodyStream()
}

// 写入 p 到响应正文。
func (ctx *RequestContext) Write(p []byte) (int, error) {
	ctx.Response.AppendBody(p)
	return len(p), nil
}

// Flush 是 ctx.Response.GetHijackWriter().Flush() 的快捷键。
// 若响应书写器未被劫持，则返回空。
func (ctx *RequestContext) Flush() error {
	if ctx.Response.GetHijackWriter() == nil {
		return nil
	}
	return ctx.Response.GetHijackWriter().Flush()
}

// ClientIP 尝试解析标头中的 [X-Real-IP, X-Forwarded-For]，它在后台调用 RemoteAddr。
//
// 若不能满足要求，可使用 route.engine.SetClientIPFunc 注入个性化实现。
func (ctx *RequestContext) ClientIP() string {
	if ctx.clientIPFunc != nil {
		return ctx.clientIPFunc(ctx)
	}
	return defaultClientIP(ctx)
}

// Next 仅限中间件内部使用。
// 它将执行当前处理链内部所有挂起的处理器。
func (ctx *RequestContext) Next(c context.Context) {
	ctx.index++
	for ctx.index < int8(len(ctx.handlers)) {
		ctx.handlers[ctx.index](c, ctx)
		ctx.index++
	}
}

// Finished 返回请求是否完成的信道。
func (ctx *RequestContext) Finished() <-chan struct{} {
	ctx.finishedMu.Lock()
	if ctx.finished == nil {
		ctx.finished = make(chan struct{})
	}
	ch := ctx.finished
	ctx.finishedMu.Unlock()
	return ch
}

// Handler 返回当前请求上下文的主处理器。
func (ctx *RequestContext) Handler() HandlerFunc {
	return ctx.handlers.Last()
}

// HandlerName 返回主处理器的函数名。
func (ctx *RequestContext) HandlerName() string {
	return utils.NameOfFunction(ctx.handlers.Last())
}

// Handlers 返回当前请求上下文的处理链。
func (ctx *RequestContext) Handlers() HandlersChain {
	return ctx.handlers
}

// SetHandlers 设置当前请求上下文的处理链。
func (ctx *RequestContext) SetHandlers(handlers HandlersChain) {
	ctx.handlers = handlers
}

// SetFullPath 设置当前请求上下文的完整路径。
func (ctx *RequestContext) SetFullPath(p string) {
	ctx.fullPath = p
}

// FullPath 返回匹配路由的完整路径。未找到的路由返回空白字符串。
//
//	router.GET("/user/:id", func(c *wind.RequestContext) {
//	    ctx.FullPath() == "/user/:id" // true
//	})
func (ctx *RequestContext) FullPath() string {
	return ctx.fullPath
}

// Redirect 重定向网址。
func (ctx *RequestContext) Redirect(statusCode int, uri []byte) {
	ctx.redirect(uri, statusCode)
}

func (ctx *RequestContext) redirect(uri []byte, statusCode int) {
	ctx.Response.Header.SetCanonical(bytestr.StrLocation, uri)
	statusCode = getRedirectStatusCode(statusCode)
	ctx.Response.SetStatusCode(statusCode)
}

// Render 写入响应标头并调用 render.Render 来渲染数据。
func (ctx *RequestContext) Render(code int, r render.Render) {
	ctx.SetStatusCode(code)

	if !bodyAllowedForStatus(code) {
		r.WriteContentType(&ctx.Response)
		return
	}

	if err := r.Render(&ctx.Response); err != nil {
		panic(err)
	}
}

// Data 写入数据至正文字节缓冲区并更新响应状态码。
func (ctx *RequestContext) Data(code int, contentType string, data []byte) {
	ctx.Render(code, render.Data{
		ContentType: contentType,
		Data:        data,
	})
}

// ProtoBuf 将给定的结构作为 protobuf 序列化到响应体中。
func (ctx *RequestContext) ProtoBuf(code int, obj any) {
	ctx.Render(code, render.ProtoBuf{Data: obj})
}

// String 以字符串形式渲染给定格式的字符串，并写入状态码。
func (ctx *RequestContext) String(code int, format string, values ...any) {
	ctx.Render(code, render.String{Format: format, Data: values})
}

// HTML 渲染给定文件名的 HTML 模板。
//
// 同时会更新状态码并将 Content-Type 自动置为 "text/html"。
func (ctx *RequestContext) HTML(code int, name string, obj any) {
	instance := ctx.HTMLRender.Instance(name, obj)
	ctx.Render(code, instance)
}

// JSON 序列化给定的结构体以 json 形式写入响应正文。
//
// 同时会更新状态码并将 Content-Type 自动设置为 "application/json"。
func (ctx *RequestContext) JSON(code int, obj any) {
	ctx.Render(code, render.JSONRender{Data: obj})
}

// PureJSON 序列化给定的结构体以纯 json 形式写入响应正文。
//
// 不同于 JSON，不会用 unicode 实体替换特殊的 html 字符。
func (ctx *RequestContext) PureJSON(code int, obj any) {
	ctx.Render(code, render.PureJSON{Data: obj})
}

// IndentedJSON 序列化给定的结构体以带缩进的 json 形式写入响应正文。
//
// 它也会自动将 Content-Type 设置为 "application/json"。
func (ctx *RequestContext) IndentedJSON(code int, obj any) {
	ctx.Render(code, render.IndentedJSON{Data: obj})
}

// Query 返回给定 key 的查询值，否则返回空白字符串 `""`。
//
// 示例：
//
//	GET /path?id=123&name=Mike&value=
//		c.Query("id") == "123"
//		c.Query("name") == "Mike"
//		c.Query("value") == ""
//		c.Query("wtf") == ""
func (ctx *RequestContext) Query(key string) string {
	value, _ := ctx.GetQuery(key)
	return value
}

// DefaultQuery 返回指定 key 的查询值，若 key 不存在则返回默认值 defaultValue。
func (ctx *RequestContext) DefaultQuery(key, defaultValue string) string {
	if value, ok := ctx.GetQuery(key); ok {
		return value
	}
	return defaultValue
}

// GetQuery 返回指定 key 的查询值。
//
// 若存在则返回 (value, true)（哪怕值为空白字符串），否则返回 ("", false)
//
// 示例： GET /?name=Mike&lastname=
//   - ("Mike", true) == c.GetQuery("name")
//   - ("", false) == c.GetQuery("id")
//   - ("", true) == c.GetQuery("lastname")
func (ctx *RequestContext) GetQuery(key string) (string, bool) {
	return ctx.QueryArgs().PeekExists(key)
}

// Param 返回指定 key 的 路由参数的值。
// 它是 ctx.Params.ByName(key) 的快捷键。
//
//	router.GET("/user/:id", func(ctx *app.RequestContext) {
//		// GET 请求 /user/mike
//		id := ctx.Param("id") // id == "mike"
//	})
func (ctx *RequestContext) Param(key string) string {
	return ctx.Params.ByName(key)
}

// PostForm 返回给定的键在经过网址编码后的 POST 表单 或多部分表单中
// 对应的值，若键不存在则返回 ""。
func (ctx *RequestContext) PostForm(key string) string {
	value, _ := ctx.GetPostForm(key)
	return value
}

// DefaultPostForm 返回给定的键在经过网址编码后的 POST 表单 或多部分表单中
// 对应的值，若键不存在则返回默认字符串。
//
// 查看: PostForm() 和 GetPostForm() 了解更多信息。
func (ctx *RequestContext) DefaultPostForm(key, defaultValue string) string {
	if value, ok := ctx.GetPostForm(key); ok {
		return value
	}
	return defaultValue
}

// GetPostForm 类似 PostForm(key)，返回给定的键在经过网址编码后的 POST 表单或多部分表单中
// 存在的值和状态 `(value, true`)，若键不存在则返回 `("", false)`。
//
// 例如，在一个 PATCH 请求更新用户邮箱过程中：
//
//	email=mail@example.com	--> ("mail@example.com", true) := GetPostForm("email") // 设邮箱为 "mail@example.com"
//		email=		--> ("", true) := GetPostForm("email") // 设邮箱为 ""
//				--> ("", false) := GetPostForm("email") // email 键也不存在
func (ctx *RequestContext) GetPostForm(key string) (string, bool) {
	if v, exists := ctx.PostArgs().PeekExists(key); exists {
		return v, exists
	}
	return ctx.multipartFormValue(key)
}

// BindAndValidate 绑定上下文的请求数据到 obj 并按需验证。 注意：obj 应为一个指针。
func (ctx *RequestContext) BindAndValidate(obj any) error {
	return binding.BindAndValidate(&ctx.Request, obj, ctx.Params)
}

// Bind 绑定上下文的请求数据到 obj。注意：obj 应为一个指针。
func (ctx *RequestContext) Bind(obj any) error {
	return binding.Bind(&ctx.Request, obj, ctx.Params)
}

// Validate  用 "vd" 标签验证 obj。
// 注意：
//
//   - obj 应为一个指针。
//   - 验证应在 Bind 之后再调用。
func (ctx *RequestContext) Validate(obj any) error {
	return binding.Validate(obj)
}

// RemoteAddr 返回当前请求的远程计算机的IP地址或域名。
//
// 若为空默认 zeroTCPAddr（形如："0.0.0.0:0"）。
func (ctx *RequestContext) RemoteAddr() net.Addr {
	if ctx.conn == nil {
		return zeroTCPAddr
	}
	addr := ctx.conn.RemoteAddr()
	if addr == nil {
		return zeroTCPAddr
	}
	return addr
}

// GetHeader 从请求头中获取给定键的值。
func (ctx *RequestContext) GetHeader(key string) []byte {
	return ctx.Request.Header.Peek(key)
}

// Header 向响应头中添加给定键值对。
// 注意：若值为 "" 则意为删除该响应头。
func (ctx *RequestContext) Header(key, value string) {
	if value == "" {
		ctx.Response.Header.Del(key)
		return
	}
	ctx.Response.Header.Set(key, value)
}

// GetRequest 返回当前请求上下文的请求副本。
func (ctx *RequestContext) GetRequest() (dst *protocol.Request) {
	dst = &protocol.Request{}
	ctx.Request.CopyTo(dst)
	return
}

// GetResponse 获取当前请求上下文的响应副本。
func (ctx *RequestContext) GetResponse() (dst *protocol.Response) {
	dst = &protocol.Response{}
	ctx.Response.CopyTo(dst)
	return
}

// Set 设置给定的键值对。
func (ctx *RequestContext) Set(key string, value any) {
	ctx.mu.Lock()
	if ctx.Keys == nil {
		ctx.Keys = make(map[string]any)
	}

	ctx.Keys[key] = value
	ctx.mu.Unlock()
}

// ForEachKey 遍历所有 Keys 键值对。
func (ctx *RequestContext) ForEachKey(fn func(k string, v any)) {
	ctx.mu.RLock()
	for key, val := range ctx.Keys {
		fn(key, val)
	}
	ctx.mu.RUnlock()
}

// VisitAllQueryArgs 为每个现有的查询参数调用 f。
//
// f 在返回后不能保留对键值的引用。
// 若返回后你需要保存他们请使用键值的副本。
func (ctx *RequestContext) VisitAllQueryArgs(f func(key, value []byte)) {
	ctx.QueryArgs().VisitAll(f)
}

// VisitAllPostArgs 为每个现有的 POST 参数调用 f。
//
// f 在返回后不能保留对键值的引用。
// 若返回后你需要保存他们请使用键值的副本。
func (ctx *RequestContext) VisitAllPostArgs(f func(key, value []byte)) {
	ctx.Request.PostArgs().VisitAll(f)
}

// VisitAllHeaders 为每个现有的请求头调用 f。
//
// f 在返回后不能保留对键值的引用。
// 若返回后你需要保存他们请使用键值的副本。
func (ctx *RequestContext) VisitAllHeaders(f func(key, value []byte)) {
	ctx.Request.Header.VisitAll(f)
}

// VisitAllCookie 为每个现有的请求 cookie 调用 f。
//
// f 在返回后不能保留对键值的引用。
// 若返回后你需要保存他们请使用键值的副本。
func (ctx *RequestContext) VisitAllCookie(f func(key, value []byte)) {
	ctx.Request.Header.VisitAllCookie(f)
}

// Value 返回给定键的值。若不存在返回 nil。
func (ctx *RequestContext) Value(key any) any {
	if ctx.Keys == nil {
		return nil
	}
	if keyString, ok := key.(string); ok {
		val, _ := ctx.Get(keyString)
		return val
	}
	return nil
}

// Get 返回给定键的值，如：(value, true)。
// 若键不存在则返回 (nil, false)。
func (ctx *RequestContext) Get(key string) (value any, exists bool) {
	ctx.mu.RLock()
	value, exists = ctx.Keys[key]
	ctx.mu.RUnlock()
	return
}

// MustGet 返回给定键的值，若键不存则触发恐慌。
func (ctx *RequestContext) MustGet(key string) any {
	if value, exists := ctx.Get(key); exists {
		return value
	}
	panic("Key \"" + key + "\" 不存在")
}

// GetString 返回给定键关联值的字符串形式，当类型错误时返回 ""。
func (ctx *RequestContext) GetString(key string) (s string) {
	if val, ok := ctx.Get(key); ok && val != nil {
		s, _ = val.(string)
	}
	return
}

// GetBool 返回给定键关联值的布尔形式，当类型错误时返回 false。
func (ctx *RequestContext) GetBool(key string) (b bool) {
	if val, ok := ctx.Get(key); ok && val != nil {
		b, _ = val.(bool)
	}
	return
}

// GetInt 返回给定键关联值的整数形式，当类型错误时返回 0。
func (ctx *RequestContext) GetInt(key string) (i int) {
	if val, ok := ctx.Get(key); ok && val != nil {
		i, _ = val.(int)
	}
	return
}

// GetInt32 返回给定键关联值的整数形式，当类型错误时返回 int32(0)。
func (ctx *RequestContext) GetInt32(key string) (i32 int32) {
	if val, ok := ctx.Get(key); ok && val != nil {
		i32, _ = val.(int32)
	}
	return
}

// GetInt64 返回给定键关联值的整数形式，当类型错误时返回 int64(0)。
func (ctx *RequestContext) GetInt64(key string) (i64 int64) {
	if val, ok := ctx.Get(key); ok && val != nil {
		i64, _ = val.(int64)
	}
	return
}

// GetUint 返回给定键关联值的整数形式，当类型错误时返回 uint(0)。
func (ctx *RequestContext) GetUint(key string) (ui uint) {
	if val, ok := ctx.Get(key); ok && val != nil {
		ui, _ = val.(uint)
	}
	return
}

// GetUint32 返回给定键关联值的整数形式，当类型错误时返回 uint32(0)。
func (ctx *RequestContext) GetUint32(key string) (ui32 uint32) {
	if val, ok := ctx.Get(key); ok && val != nil {
		ui32, _ = val.(uint32)
	}
	return
}

// GetUint64 返回给定键关联值的整数形式，当类型错误时返回 uint64(0)。
func (ctx *RequestContext) GetUint64(key string) (ui64 uint64) {
	if val, ok := ctx.Get(key); ok && val != nil {
		ui64, _ = val.(uint64)
	}
	return
}

// GetFloat32 返回给定键关联值的整数形式，当类型错误时返回 float32(0)。
func (ctx *RequestContext) GetFloat32(key string) (f32 float32) {
	if val, ok := ctx.Get(key); ok && val != nil {
		f32, _ = val.(float32)
	}
	return
}

// GetFloat64 返回给定键关联值的整数形式，当类型错误时返回 float64(0)。
func (ctx *RequestContext) GetFloat64(key string) (f64 float64) {
	if val, ok := ctx.Get(key); ok && val != nil {
		f64, _ = val.(float64)
	}
	return
}

// GetTime 返回给定键关联值的时间形式，当类型错误时返回 time.Time{}。
func (ctx *RequestContext) GetTime(key string) (t time.Time) {
	if val, ok := ctx.Get(key); ok && val != nil {
		t, _ = val.(time.Time)
	}
	return
}

// GetDuration 返回给定键关联值的时长形式，当类型错误时返回 time.Duration{}。
func (ctx *RequestContext) GetDuration(key string) (t time.Duration) {
	if val, ok := ctx.Get(key); ok && val != nil {
		t, _ = val.(time.Duration)
	}
	return
}

// GetStringSlice 返回给定键关联值的字符串切片形式，当类型错误时返回 []string(nil)。
func (ctx *RequestContext) GetStringSlice(key string) (ss []string) {
	if val, ok := ctx.Get(key); ok && val != nil {
		ss, _ = val.([]string)
	}
	return
}

// GetStringMap 返回给定键关联值的字典形式，当类型错误时返回 map[string]any(nil)。
func (ctx *RequestContext) GetStringMap(key string) (sm map[string]any) {
	if val, ok := ctx.Get(key); ok && val != nil {
		sm, _ = val.(map[string]any)
	}
	return
}

// GetStringMapString 返回给定键关联值的字典形式，当类型错误时返回 map[string]string(nil)。
func (ctx *RequestContext) GetStringMapString(key string) (sms map[string]string) {
	if val, ok := ctx.Get(key); ok && val != nil {
		sms, _ = val.(map[string]string)
	}
	return
}

// GetStringMapStringSlice 返回给定键关联值的字典形式，当类型错误时返回 map[string][]string(nil)。
func (ctx *RequestContext) GetStringMapStringSlice(key string) (smss map[string][]string) {
	if val, ok := ctx.Get(key); ok && val != nil {
		smss, _ = val.(map[string][]string)
	}
	return
}

// ContentType 返回请求的内容类型标头值。
func (ctx *RequestContext) ContentType() []byte {
	return ctx.Request.Header.ContentType()
}

// Cookie 返回请求头中给定 key 的 cookie 值。
func (ctx *RequestContext) Cookie(key string) []byte {
	return ctx.Request.Header.Cookie(key)
}

// SetCookie 添加一个 Set-Cookie 响应头。
//
//	参数包括：
//	name 和 value 用于设置 cookie 的名称和值，如：Set-Cookie: name=value
//	maxAge 到期秒数，用于设置 cookie 的过期时间，如: Set-Cookie: name=value; max-age=1
//	path 和 domain 用于设置 cookie 的范围，如: Set-Cookie: name=value; domain=localhost; path=/
//	secure 和 httpOnly 用于设置 cookie 的安全性，如: Set-Cookie: name=value;HttpOnly;secure
//	sameSite 允许服务器指定是否/何时使用跨站请求发送 cookie，如: Set-Cookie: name=value;HttpOnly; secure; SameSite=Lax;
//
//	例如：
//	1. ctx.SetCookie("user", "wind", 1, "/", "localhost", protocol.CookieSameSiteLaxMode, true, true)
//	添加响应头 ---> Set-Cookie: user=wind; max-age=1; domain=localhost; path=/; HttpOnly; secure; SameSite=Lax;
//	2. ctx.SetCookie("user", "wind", 10, "/", "localhost", protocol.CookieSameSiteLaxMode, false, false)
//	添加响应头 ---> Set-Cookie: user=wind; max-age=10; domain=localhost; path=/; SameSite=Lax;
//	3. ctx.SetCookie("", "wind", 10, "/", "localhost", protocol.CookieSameSiteLaxMode, false, false)
//	添加响应头 ---> Set-Cookie: wind; max-age=10; domain=localhost; path=/; SameSite=Lax;
//	4. ctx.SetCookie("user", "", 10, "/", "localhost", protocol.CookieSameSiteLaxMode, false, false)
//	添加响应头 ---> Set-Cookie: user=; max-age=10; domain=localhost; path=/; SameSite=Lax;
func (ctx *RequestContext) SetCookie(name, value string, maxAge int, path, domain string, sameSite protocol.CookieSameSite, secure, httpOnly bool) {
	if path == "" {
		path = "/"
	}
	cookie := protocol.AcquireCookie()
	defer protocol.ReleaseCookie(cookie)
	cookie.SetKey(name)
	cookie.SetValue(url.QueryEscape(value))
	cookie.SetMaxAge(maxAge)
	cookie.SetPath(path)
	cookie.SetDomain(domain)
	cookie.SetSecure(secure)
	cookie.SetHTTPOnly(httpOnly)
	cookie.SetSameSite(sameSite)
	ctx.Response.Header.SetCookie(cookie)
}

// UserAgent 返回请求的用户代理值。
func (ctx *RequestContext) UserAgent() []byte {
	return ctx.Request.Header.UserAgent()
}

// Status 设置 HTTP 响应状态吗。
func (ctx *RequestContext) Status(code int) {
	ctx.SetStatusCode(code)
}

// Copy 返回当前上下文可在请求范围之外安全使用的副本。
//
// 注意：若想将 RequestContext 传入协程，需调此方法传递副本。
func (ctx *RequestContext) Copy() *RequestContext {
	cp := &RequestContext{
		conn:   ctx.conn,
		Params: ctx.Params,
	}
	ctx.Request.CopyTo(&cp.Request)
	ctx.Response.CopyTo(&cp.Response)
	cp.index = rConsts.AbortIndex
	cp.handlers = nil
	cp.Keys = map[string]any{}
	ctx.mu.RLock()
	for k, v := range ctx.Keys {
		cp.Keys[k] = v
	}
	ctx.mu.RUnlock()
	paramsCopy := make([]param.Param, len(cp.Params))
	copy(paramsCopy, cp.Params)
	cp.Params = paramsCopy
	return cp
}

func (ctx *RequestContext) multipartFormValue(key string) (string, bool) {
	mf, err := ctx.MultipartForm()
	if err == nil && mf.Value != nil {
		vv := mf.Value[key]
		if len(vv) > 0 {
			return vv[0], true
		}
	}
	return "", false
}

// bodyAllowedForStatus 拷贝自 http.bodyAllowedForStatus，
// 用于报告给定的响应状态代码是否允许响应正文。
func bodyAllowedForStatus(status int) bool {
	switch {
	case status >= 100 && status <= 199:
		return false
	case status == consts.StatusNoContent:
		return false
	case status == consts.StatusNotModified:
		return false
	}
	return true
}

func getRedirectStatusCode(statusCode int) int {
	if statusCode == consts.StatusMovedPermanently ||
		statusCode == consts.StatusFound ||
		statusCode == consts.StatusSeeOther ||
		statusCode == consts.StatusTemporaryRedirect ||
		statusCode == consts.StatusPermanentRedirect {
		return statusCode
	}
	return consts.StatusFound
}

type (
	// ClientIP 是获取获取客户端 IP 的自定义函数。
	ClientIP        func(ctx *RequestContext) string
	ClientIPOptions struct {
		RemoteIPHeaders []string     // 客户端IP标头名切片，默认为 []string{"X-Real-IP", "X-Forwarded-For"}
		TrustedCIDRs    []*net.IPNet // 是可信代理IP(非客户端)，故需从 X-Forwarded-For 中跳过。默认IP为 0.0.0.0，亦为可信代理。
	}

	// FormValueFunc 是获取表单值的自定义函数。
	FormValueFunc func(*RequestContext, string) []byte
)

var defaultFormValue = func(ctx *RequestContext, key string) []byte {
	v := ctx.QueryArgs().Peek(key)
	if len(v) > 0 {
		return v
	}
	v = ctx.PostArgs().Peek(key)
	if len(v) > 0 {
		return v
	}
	mf, err := ctx.MultipartForm()
	if err == nil && mf.Value != nil {
		vv := mf.Value[key]
		if len(vv) > 0 {
			return []byte(vv[0])
		}
	}
	return nil
}

var defaultTrustedCIDRs = []*net.IPNet{
	{ // 0.0.0.0/0 (IPv4)
		IP:   net.IP{0x0, 0x0, 0x0, 0x0},
		Mask: net.IPMask{0x0, 0x0, 0x0, 0x0},
	},
	{ // ::/0 (IPv6)
		IP:   net.IP{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		Mask: net.IPMask{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
	},
}

var defaultClientIPOptions = ClientIPOptions{
	RemoteIPHeaders: []string{"X-Forwarded-For", "X-Real-IP"},
	TrustedCIDRs:    defaultTrustedCIDRs,
}

var defaultClientIP = ClientIPWithOption(defaultClientIPOptions)

// SetClientIPFunc 设置 ClientIP 函数实现自定义 IP 获取方法。
// Deprecated: 使用 engine.SetClientIPFunc 替代此方法。
func SetClientIPFunc(fn ClientIP) {
	defaultClientIP = fn
}

// ClientIPWithOption 用于生成自定义的 ClientIP 函数，由 engine.SetClientIPFunc 设置。
func ClientIPWithOption(opts ClientIPOptions) ClientIP {
	return func(ctx *RequestContext) string {
		remoteIPHeaders := opts.RemoteIPHeaders
		trustedCIDRs := opts.TrustedCIDRs

		// 优先级 1：尝试 net.Conn.RemoteAddr 作为客户端 IP
		remoteIPStr, _, err := net.SplitHostPort(strings.TrimSpace(ctx.RemoteAddr().String()))
		if err != nil {
			return ""
		}

		remoteIP := net.ParseIP(remoteIPStr)
		if remoteIP == nil {
			return ""
		}

		// 优先级 2：若上述IP是可信代理，则需继续追查。
		trusted := isTrustedProxy(trustedCIDRs, remoteIP)

		if trusted {
			// 按配置的远程IP标头顺序，逐个检查是否为有效的客户端IP
			for _, headerName := range remoteIPHeaders {
				ip, valid := validateHeader(trustedCIDRs, ctx.Request.Header.Get(headerName))
				if valid {
					return ip
				}
			}
		}

		// 若该远程IP不是可信代理，则作为客户端IP
		return remoteIPStr
	}
}

// isTrustedProxy 基于 trustedCIDRs 检查 IP 地址是否包含在受信任的列表中。
func isTrustedProxy(trustedCIDRs []*net.IPNet, remoteIP net.IP) bool {
	if trustedCIDRs == nil {
		return false
	}

	for _, cidr := range trustedCIDRs {
		if cidr.Contains(remoteIP) {
			return true
		}
	}
	return false
}

// validateHeader 将解析 X-Real-IP 和 X-Forwarded-For 标头，并返回初始客户端 IP 地址或不受信任的 IP 地址。
func validateHeader(trustedCIDRs []*net.IPNet, header string) (clientIP string, valid bool) {
	if header == "" {
		return "", false
	}
	items := strings.Split(header, ",")
	for i := len(items) - 1; i >= 0; i-- {
		ipStr := strings.TrimSpace(items[i])
		ip := net.ParseIP(ipStr)
		if ip == nil {
			break
		}

		// X-Forwarded-For 由代理追加
		// 反向检查 IP 地址，直到找到不受信任的代理为止。
		if (i == 0) || (!isTrustedProxy(trustedCIDRs, ip)) {
			return ipStr, true
		}
	}
	return "", false
}
