//go:build !windows

package protocol

func addLeadingSlash(dst, src []byte) []byte {
	// 为 unix 路径添加引导斜线
	if len(src) == 0 || src[0] != '/' {
		dst = append(dst, '/')
	}

	return dst
}
