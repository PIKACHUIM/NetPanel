//go:build !windows

package sysutil

import "os"

// IsAdmin 检测当前进程是否具有管理员（root）权限
func IsAdmin() bool {
	return os.Getuid() == 0
}
