//go:build !linux

package svcutil

import "fmt"

func installLinuxService(_ string) error {
	return fmt.Errorf("Linux 服务注册仅在 Linux 平台可用")
}

func uninstallLinuxService() error {
	return fmt.Errorf("Linux 服务卸载仅在 Linux 平台可用")
}
