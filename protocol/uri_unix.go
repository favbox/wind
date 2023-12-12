//go:build !windows

package protocol

import "github.com/favbox/wind/common/wlog"

func addLeadingSlash(dst, src []byte) []byte {
	// 为 unix 路径添加引导斜线
	if len(src) == 0 || src[0] != '/' {
		dst = append(dst, '/')
	}

	return dst
}

// 处理形如 "http:" 协议场景中，网址是否以 : 开头。
func checkSchemeWhenCharIsColon(i int, rawURL []byte) (scheme, path []byte) {
	if i == 0 {
		wlog.Errorf("错误发生于尝试解析原始URL(%q): 缺少协议方案", rawURL)
		return
	}
	return rawURL[:i], rawURL[i+1:]
}
