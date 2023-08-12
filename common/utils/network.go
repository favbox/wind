package utils

// TLSRecordHeaderLooksLikeHTTP 判断 TLS 记录头是否看起来像是发自 HTTP 的请求？
func TLSRecordHeaderLooksLikeHTTP(hdr [5]byte) bool {
	switch string(hdr[:]) {
	case "GET /", "HEAD ", "POST ", "PUT /", "OPTIO":
		return true
	}
	return false
}
