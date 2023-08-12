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
