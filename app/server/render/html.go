package render

import (
	"html/template"
	"log"
	"sync"
	"time"

	"github.com/favbox/gosky/wind/pkg/common/hlog"
	"github.com/favbox/gosky/wind/pkg/protocol"
	"github.com/fsnotify/fsnotify"
)

var htmlContentType = "text/html; charset=utf-8"

// HTML 包含 HTML 名称、模板和所需的数据。
type HTML struct {
	Template *template.Template
	Name     string
	Data     any
}

// Render 渲染 HTML 超文本。
func (r HTML) Render(resp *protocol.Response) error {
	writeContentType(resp, htmlContentType)

	if r.Name == "" {
		return r.Template.Execute(resp.BodyWriter(), r.Data)
	}
	return r.Template.ExecuteTemplate(resp.BodyWriter(), r.Name, r.Data)
}

// WriteContentType 写入HTML 超文本内容类型。
func (r HTML) WriteContentType(resp *protocol.Response) {
	writeContentType(resp, htmlContentType)
}

// HTMLRender 超文本渲染器，会被 HTMLProduction 和 HTMLDebug 实现。
type HTMLRender interface {
	// Instance 返回一个 HTML 实例。
	Instance(string, any) Render
	Close() error
}

// HTMLProduction 用于生产环境的 HTML 渲染器。
type HTMLProduction struct {
	Template *template.Template
}

func (r HTMLProduction) Instance(name string, data any) Render {
	return HTML{
		Template: r.Template,
		Name:     name,
		Data:     data,
	}
}

func (r HTMLProduction) Close() error {
	return nil
}

// Delims 用于 HTML 模板渲染的左、右分隔符。
type Delims struct {
	Left  string // 左分隔符，默认为 "{{"。
	Right string // 右分隔符，默认为 "}}"。
}

// HTMLDebug 用于调试的 HTML 渲染器。
type HTMLDebug struct {
	sync.Once
	Template        *template.Template
	RefreshInterval time.Duration // 若 > 0 则按间隔时间重载，反之使用 fsnotify 自动重载。

	Files   []string
	FuncMap template.FuncMap
	Delims  Delims

	reloadCh chan struct{}
	watcher  *fsnotify.Watcher
}

func (r *HTMLDebug) Instance(name string, data any) Render {
	r.Do(func() {
		r.startChecker()
	})

	select {
	case <-r.reloadCh:
		r.reload()
	default:
	}

	return HTML{
		Template: r.Template,
		Name:     name,
		Data:     data,
	}
}

func (r *HTMLDebug) Close() error {
	if r.watcher == nil {
		return nil
	}
	return r.watcher.Close()
}

func (r *HTMLDebug) startChecker() {
	r.reloadCh = make(chan struct{})

	// 按指定间隔重载
	if r.RefreshInterval > 0 {
		go func() {
			hlog.SystemLogger().Debugf("[HTMLDebug] HTML 模板间隔 %v 重载一次", r.RefreshInterval)
			for range time.Tick(r.RefreshInterval) {
				hlog.SystemLogger().Debugf("[HTMLDebug] 正在触发 HTML 模板重载")
				r.reloadCh <- struct{}{}
				hlog.SystemLogger().Debugf("[HTMLDebug] HTML 模板已重载，下次于 %v 后重载", r.RefreshInterval)
			}
		}()
		return
	}

	// 利用 fsnotify 自动监视文件并重载
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	r.watcher = watcher
	for _, f := range r.Files {
		err := watcher.Add(f)
		hlog.SystemLogger().Debugf("[HTMLDebug] 正在监视文件：%s", f)
		if err != nil {
			hlog.SystemLogger().Errorf("[HTMLDebug] 添加监视文件：%s，出现错误：%v", f, err)
		}
	}

	go func() {
		hlog.SystemLogger().Debugf("[HTMLDebug] HTML 模板将通过文件监听自动重载 ")
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					hlog.SystemLogger().Debugf("[HTMLDebug] 修改的文件：%s，HTML 模板将在下次渲染时重载", event.Name)
					r.reloadCh <- struct{}{}
					hlog.SystemLogger().Debugf("[HTMLDebug] HTML模板已重载")
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				hlog.SystemLogger().Errorf("错误发生于监视渲染文件：%v", err)
			}
		}
	}()
}

func (r *HTMLDebug) reload() {
	r.Template = template.Must(template.New("").
		Delims(r.Delims.Left, r.Delims.Right).
		Funcs(r.FuncMap).
		ParseFiles(r.Files...))
}
