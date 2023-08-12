package network

import "syscall"

// UnlinkUdsFile 取消 unix 网络连接。
func UnlinkUdsFile(network, addr string) error {
	if network == "unix" {
		return syscall.Unlink(addr)
	}
	return nil
}
