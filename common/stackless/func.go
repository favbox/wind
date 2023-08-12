package stackless

import (
	"runtime"
	"sync"
)

// NewFunc 返回函数 f 的无栈包装器。
//
// 与 f 不同，返回的无栈包装器在调用它的 goroutine 上不使用栈空间。
// 若满足如下条件，包装器可节省大量栈空间：
//
//   - f 无阻塞（不含对网络、I/O或通道的阻塞调用）；
//   - f 大堆栈（使用了大量栈空间）；
//   - f 高并发（包装器被大量并发 goroutine 调用）。
//
// 若因高负载而无法处理调用，则返回 false。
func NewFunc(f func(ctx any)) func(ctx any) bool {
	if f == nil {
		panic("BUG：f 不能为空")
	}

	funcWorkCh := make(chan *funcWork, runtime.GOMAXPROCS(-1)*2048)
	onceInit := func() {
		n := runtime.GOMAXPROCS(-1)
		for i := 0; i < n; i++ {
			go funcWorker(funcWorkCh, f)
		}
	}
	var once sync.Once

	return func(ctx any) bool {
		once.Do(onceInit)
		fw := getFuncWork()
		fw.ctx = ctx

		select {
		case funcWorkCh <- fw:
		default:
			putFuncWork(fw)
			return false
		}
		<-fw.done
		putFuncWork(fw)
		return true
	}
}

type funcWork struct {
	ctx  any
	done chan struct{}
}

var funcWorkPool sync.Pool

func getFuncWork() *funcWork {
	v := funcWorkPool.Get()
	if v == nil {
		v = &funcWork{
			done: make(chan struct{}, 1),
		}
	}
	return v.(*funcWork)
}

func putFuncWork(fw *funcWork) {
	fw.ctx = nil
	funcWorkPool.Put(fw)
}

func funcWorker(funcWorkCh <-chan *funcWork, f func(ctx any)) {
	for fw := range funcWorkCh {
		f(fw.ctx)
		fw.done <- struct{}{}
	}
}
