package server

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/favbox/wind/app/middlewares/server/recovery"
	"github.com/favbox/wind/common/config"
	"github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/common/hlog"
	"github.com/favbox/wind/route"
)

// New 创建一个无默认配置的 wind 实例。
func New(opts ...config.Option) *Wind {
	options := config.NewOptions(opts)
	w := &Wind{
		Engine: route.NewEngine(options),
	}
	return w
}

// Default 创建默认带有 recovery 中间件的 wind 实例。
func Default(opts ...config.Option) *Wind {
	w := New(opts...)
	w.Use(recovery.Recovery())

	return w
}

// Wind 是 wind 的核心结构。
//
// 组合了路由引擎 route.Engine 和 优雅退出函数。
type Wind struct {
	*route.Engine
	// 用于接收信息实现优雅退出
	signalWaiter func(err chan error) error
}

// Spin 运行服务器直至捕获 os.Signal 或 w.Run 返回错误。
// 支持优雅退出。
func (w *Wind) Spin() {
	errCh := make(chan error)
	w.initOnRunHooks(errCh) // 调用服务注册
	go func() {
		errCh <- w.Run()
	}()

	signalWaiter := defaultSignalWaiter
	if w.signalWaiter != nil {
		signalWaiter = w.signalWaiter
	}

	if err := signalWaiter(errCh); err != nil {
		hlog.SystemLogger().Errorf("收到退出信号：错误=%v", err)
		if err = w.Engine.Close(); err != nil {
			hlog.SystemLogger().Errorf("退出错误：%v", err)
		}
		return
	}

	hlog.SystemLogger().Infof("开始优雅退出，最多等待 %d 秒...", w.GetOptions().ExitWaitTimeout/time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), w.GetOptions().ExitWaitTimeout)
	defer cancel()

	if err := w.Shutdown(ctx); err != nil {
		hlog.SystemLogger().Errorf("退出错误：%v", err)
	}
}

// SetCustomSignalWaiter 设置自定义的信号等待者。
// 若默认的信号等待实现不符要求，则可以自定义。
// Wind 在 f 返回错误后会立即退出，否则它将优雅退出。
func (w *Wind) SetCustomSignalWaiter(f func(err chan error) error) {
	w.signalWaiter = f
}

// 初始运行钩子：尝试注册服务
func (w *Wind) initOnRunHooks(errChan chan error) {
	// 添加服务注册函数到 runHooks 钩子中
	opts := w.GetOptions()
	w.OnRun = append(w.OnRun, func(ctx context.Context) error {
		go func() {
			// 延迟 1 秒再注册
			time.Sleep(1 * time.Second)
			if err := opts.Registry.Register(opts.RegistryInfo); err != nil {
				hlog.SystemLogger().Errorf("服务注册出错：%v", err)
				// 传递错误到错误通道
				errChan <- err
			}
		}()
		return nil
	})
}

// 信号等待者的默认实现。
// SIGTERM 立即退出。
// SIGHUP|SIGINT 触发优雅退出。
func defaultSignalWaiter(errCh chan error) error {
	signalToNotify := []os.Signal{
		syscall.SIGINT,
		syscall.SIGHUP,
		syscall.SIGTERM,
	}
	if signal.Ignored(syscall.SIGHUP) {
		signalToNotify = []os.Signal{
			syscall.SIGINT,
			syscall.SIGTERM,
		}
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, signalToNotify...)

	select {
	case sig := <-signals:
		switch sig {
		case syscall.SIGTERM:
			// 强制退出
			return errors.NewPublic(sig.String())
		case syscall.SIGHUP, syscall.SIGINT:
			hlog.SystemLogger().Infof("收到退出信号：%s\n", sig)
			// 优雅退出
			return nil
		}
	case err := <-errCh:
		// 出现错误，立即退出
		return err
	}

	return nil
}
