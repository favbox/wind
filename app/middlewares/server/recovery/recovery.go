package recovery

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/favbox/wind/app"
)

var (
	dunno     = []byte("???") // I don't know
	slash     = []byte("/")
	dot       = []byte(".")
	centerDot = []byte("·")
)

// Recovery 返回一个可以从任何 panic 恢复的中间件。
// 默认情况下，它将打印错误的时间、内容和堆栈信息，并写入500。
// 通过覆盖 Option 配置可以自定义错误的打印逻辑。
func Recovery(opts ...Option) app.HandlerFunc {
	cfg := newOptions(opts...)

	return func(c context.Context, ctx *app.RequestContext) {
		defer func() {
			if err := recover(); err != nil {
				stack := stack(3)

				cfg.recoveryHandler(c, ctx, err, stack)
			}
		}()
		ctx.Next(c)
	}
}

// 跳过给定的堆栈帧，返回一个格式化良好的堆栈帧。
func stack(skip int) []byte {
	buf := new(bytes.Buffer) // 返回的数据
	// 循环打开文件并读取，如下变量用于记录当前已加载的文件。
	var lines [][]byte
	var lastFile string
	for i := skip; ; i++ { // 跳过给定的帧数
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		// 至少打印这么多。如果找不到错误来源则不会显示。
		fmt.Fprintf(buf, "%s:%d (0x%x)\n", file, line, pc) // program counter
		if file != lastFile {
			data, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			lines = bytes.Split(data, []byte{'\n'})
			lastFile = file
		}
		fmt.Fprintf(buf, "\t%s: %s\n", function(pc), source(lines, line))
	}
	return buf.Bytes()
}

// 返回第 n 行去掉空格的切片。
func source(lines [][]byte, n int) []byte {
	// 在堆栈跟踪中，行是1索引的，但我们的数组是0索引的
	n--

	// 找不到，我不知道
	if n < 0 || n >= len(lines) {
		return dunno
	}
	return bytes.TrimSpace(lines[n])
}

// 返回包含程序计数器 pc 的函数名称。
func function(pc uintptr) []byte {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return dunno
	}
	name := []byte(fn.Name())

	// 该名称包括包路径，这是不必要的，因为文件名已经包括在内。另外，它有中心点 '·'。
	// 也就是说，我们看到的是
	//	runtime/debug.*T·ptrmethod
	// 我们想要的是
	//	*T.ptrmethod
	// 另外，包路径可能包含句点 '.'（如 google.com/...），因此首先消除路径前缀。
	if lastSlash := bytes.LastIndex(name, slash); lastSlash >= 0 {
		name = name[lastSlash+1:]
	}
	if period := bytes.Index(name, dot); period >= 0 {
		name = name[period+1:]
	}
	name = bytes.Replace(name, centerDot, dot, -1)
	return name
}
