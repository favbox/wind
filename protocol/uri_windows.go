//go:build windows

package protocol

func addLeadingSlash(dst, src []byte) []byte {
	// 空路径和 "C:/" 情况
	isDisk := len(src) > 2 && src[1] == ':'
	if len(src) == 0 || (!isDisk && src[0] != '/') {
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

	// case :\
	if i+1 < len(rawURL) && rawURL[i+1] == '\\' {
		return nil, rawURL
	}

	return rawURL[:i], rawURL[i+1:]
}
