package utils

import "net"

const UnknownIPAddr = "-"

var localIP string

// LocalIP 返回主机的IP地址。
func LocalIP() string {
	return localIP
}

// 遍历本地网络接口以查找本地IP，它只应在初始化阶段调用。
func getLocalIP() string {
	inters, err := net.Interfaces()
	if err != nil {
		return UnknownIPAddr
	}
	for _, inter := range inters {
		if inter.Flags&net.FlagLoopback != net.FlagLoopback &&
			inter.Flags&net.FlagUp != 0 {
			addrs, err := inter.Addrs()
			if err != nil {
				return UnknownIPAddr
			}
			for _, addr := range addrs {
				if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
					return ipNet.IP.String()
				}
			}
		}
	}

	return UnknownIPAddr
}

func init() {
	localIP = getLocalIP()
}

// TLSRecordHeaderLooksLikeHTTP 判断 TLS 标头是否看起来像是发自 HTTP 的纯文本请求？
func TLSRecordHeaderLooksLikeHTTP(hdr [5]byte) bool {
	switch string(hdr[:]) {
	case "GET /", "HEAD ", "POST ", "PUT /", "OPTIO":
		return true
	}
	return false
}
