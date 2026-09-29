//go:build windows

package stun

import "syscall"

// controlReusePort Windows 不支持 SO_REUSEPORT：保活连接无法与监听共用
// 端口，Keepalive 拨号会绑定失败，上层降级为仅转发并记录警告。
func controlReusePort(network, address string, c syscall.RawConn) error {
	return nil
}
