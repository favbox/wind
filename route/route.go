package route

import (
	"context"
	"path"
	"regexp"
	"strings"

	"github.com/favbox/wind/app"
	"github.com/favbox/wind/protocol/consts"
	rConsts "github.com/favbox/wind/route/consts"
)

var upperLetterReg = regexp.MustCompile("^[A-Z]+$")

// Route 表示一个路由信息，包括请求方法、路径及其处理程序。
type Route struct {
	Method      string          // 请求方法
	Path        string          // 请求路径
	Handler     string          // 处理器名称
	HandlerFunc app.HandlerFunc // 处理器函数
}

// Routes 定义了一组路由信息。
type Routes []Route

// Router 定义路由器接口。
type Router interface {
	Use(...app.HandlerFunc) Router
	Handle(string, string, ...app.HandlerFunc) Router
	Any(string, ...app.HandlerFunc) Router
	GET(string, ...app.HandlerFunc) Router
	POST(string, ...app.HandlerFunc) Router
	DELETE(string, ...app.HandlerFunc) Router
	PATCH(string, ...app.HandlerFunc) Router
	PUT(string, ...app.HandlerFunc) Router
	OPTIONS(string, ...app.HandlerFunc) Router
	HEAD(string, ...app.HandlerFunc) Router
	StaticFile(string, string) Router
	Static(string, string) Router
	StaticFS(string, *app.FS) Router
}

// Routers 定义路由器接口，包括单路由和分组路由。
type Routers interface {
	Router
	Group(string, ...app.HandlerFunc) *RouterGroup
}

// RouterGroup 表示一个路由组，由前缀路径和一组处理器（中间件）组成。
type RouterGroup struct {
	Handlers app.HandlersChain
	basePath string
	engine   *Engine
	root     bool
}

var _ Routers = (*RouterGroup)(nil)

// BasePath 获取路由组的基本路径，即这组路由的共同前缀。
func (group *RouterGroup) BasePath() string {
	return group.basePath
}

// Group 创建分组路由。可添加有相同前缀和中间件的路由（如使用同一鉴权中间件的 /admin 路由）。
func (group *RouterGroup) Group(relativePath string, handlers ...app.HandlerFunc) *RouterGroup {
	return &RouterGroup{
		Handlers: group.combineHandlers(handlers),
		basePath: group.calculateAbsolutePath(relativePath),
		engine:   group.engine,
	}
}

// Use 添加中间件到该分组路由。
func (group *RouterGroup) Use(middleware ...app.HandlerFunc) Router {
	group.Handlers = append(group.Handlers, middleware...)
	return group.asObject()
}

// Handle 路由注册的通用函数，最后一个处理器为主函数，其余为中间件。 也可用于低频或非标的请求方法（如：与代理的内部通信等）。
func (group *RouterGroup) Handle(httpMethod string, relativePath string, handlers ...app.HandlerFunc) Router {
	if matches := upperLetterReg.MatchString(httpMethod); !matches {
		panic("http 请求方法 `" + httpMethod + "` 无效")
	}
	return group.handle(httpMethod, relativePath, handlers)
}

// Any 注册一条支持所有标准请求方法的路由。
func (group *RouterGroup) Any(path string, handlers ...app.HandlerFunc) Router {
	group.handle(consts.MethodGet, path, handlers)
	group.handle(consts.MethodPost, path, handlers)
	group.handle(consts.MethodPut, path, handlers)
	group.handle(consts.MethodPatch, path, handlers)
	group.handle(consts.MethodHead, path, handlers)
	group.handle(consts.MethodOptions, path, handlers)
	group.handle(consts.MethodDelete, path, handlers)
	group.handle(consts.MethodConnect, path, handlers)
	group.handle(consts.MethodTrace, path, handlers)
	return group.asObject()
}

// GET 注册一条 GET 路由，是 Handle("GET", relativePath, handlers) 的快捷方式。
func (group *RouterGroup) GET(relativePath string, handlers ...app.HandlerFunc) Router {
	return group.handle(consts.MethodGet, relativePath, handlers)
}

// POST 注册一条 POST 路由， 是 Handle("POST", relativePath, handlers) 的快捷方式。
func (group *RouterGroup) POST(relativePath string, handlers ...app.HandlerFunc) Router {
	return group.handle(consts.MethodPost, relativePath, handlers)
}

// DELETE 注册一条 DELETE 路由， 是 Handle("DELETE", relativePath, handlers) 的快捷方式。
func (group *RouterGroup) DELETE(relativePath string, handlers ...app.HandlerFunc) Router {
	return group.handle(consts.MethodDelete, relativePath, handlers)

}

// PATCH 注册一条 PATCH 路由， 是 Handle("PATCH", relativePath, handlers) 的快捷方式。
func (group *RouterGroup) PATCH(relativePath string, handlers ...app.HandlerFunc) Router {
	return group.handle(consts.MethodPatch, relativePath, handlers)
}

// PUT 注册一条 PUT 路由， 是 Handle("PUT", relativePath, handlers) 的快捷方式。
func (group *RouterGroup) PUT(relativePath string, handlers ...app.HandlerFunc) Router {
	return group.handle(consts.MethodPut, relativePath, handlers)
}

// OPTIONS 注册给定路径需 OPTIONS 处处理器 是 Handle("OPTIONS", relativePath, handlers) 的快捷方式。
func (group *RouterGroup) OPTIONS(relativePath string, handlers ...app.HandlerFunc) Router {
	return group.handle(consts.MethodOptions, relativePath, handlers)
}

// HEAD 注册一条 HEAD 路由， 是 Handle("HEAD", relativePath, handlers) 的快捷方式。
func (group *RouterGroup) HEAD(relativePath string, handlers ...app.HandlerFunc) Router {
	return group.handle(consts.MethodHead, relativePath, handlers)
}

// StaticFile 单文件服务。用法：StaticFile("favicon.ico", "./resources/favicon.ico")
func (group *RouterGroup) StaticFile(relativePath string, filepath string) Router {
	if strings.Contains(relativePath, ":") || strings.Contains(relativePath, "*") {
		panic("提供静态文件服务时不能使用 URL 参数，如':*'")
	}
	handler := func(c context.Context, ctx *app.RequestContext) {
		ctx.File(filepath)
	}
	group.GET(relativePath, handler)
	group.HEAD(relativePath, handler)
	return group.asObject()
}

// Static 文件夹服务。用法：router.Static("/static", "/var/www")
func (group *RouterGroup) Static(relativePath string, root string) Router {
	return group.StaticFS(relativePath, &app.FS{Root: root})
}

// StaticFS 用法同 Static() ，但可自定义 app.FS。
func (group *RouterGroup) StaticFS(relativePath string, fs *app.FS) Router {
	if strings.Contains(relativePath, ":") || strings.Contains(relativePath, "*") {
		panic("URL 命名参数不可用于静态文件夹服务")
	}
	urlPattern := path.Join(relativePath, "/*filepath")

	// 注册 GET 和 HEAD 处理器
	handler := fs.NewRequestHandler()
	group.GET(urlPattern, handler)
	group.HEAD(urlPattern, handler)
	return group.asObject()
}

func (group *RouterGroup) asObject() Routers {
	if group.root {
		return group.engine
	}
	return group
}

func (group *RouterGroup) handle(httpMethod, relativePath string, handlers app.HandlersChain) Router {
	absolutePath := group.calculateAbsolutePath(relativePath)
	handlers = group.combineHandlers(handlers)
	group.engine.addRoute(httpMethod, absolutePath, handlers)
	return group.asObject()
}

func (group *RouterGroup) calculateAbsolutePath(relativePath string) string {
	return joinPaths(group.basePath, relativePath)
}

// 合并处理链至当前路由组。注意：若合并后长度超过 consts.AbortIndex 会引发恐慌。
func (group *RouterGroup) combineHandlers(handlers app.HandlersChain) app.HandlersChain {
	finalSize := len(group.Handlers) + len(handlers)
	if finalSize >= int(rConsts.AbortIndex) {
		panic("处理函数过多")
	}
	mergedHandlers := make(app.HandlersChain, finalSize)
	copy(mergedHandlers, group.Handlers)
	copy(mergedHandlers[len(group.Handlers):], handlers)
	return mergedHandlers
}

func joinPaths(absolutePath, relativePath string) string {
	if relativePath == "" {
		return absolutePath
	}

	finalPath := path.Join(absolutePath, relativePath)
	appendSlash := lastChar(relativePath) == '/' && lastChar(finalPath) != '/'
	if appendSlash {
		return finalPath + "/"
	}
	return finalPath
}

func lastChar(s string) uint8 {
	if s == "" {
		panic("字符串长度不能为 0")
	}
	return s[len(s)-1]
}
