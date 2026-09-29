//go:build unix

package stun

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// controlReusePort 在绑定/拨号前设置 SO_REUSEPORT，使保活连接能够与
// TCP 监听共用同一本地端口（同一本地端口维持同一 NAT 映射）。
// Windows 无此语义，见 reuseport_windows.go。
func controlReusePort(network, address string, c syscall.RawConn) error {
	var serr error
	if err := c.Control(func(fd uintptr) {
		serr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
	}); err != nil {
		return err
	}
	return serr
}
