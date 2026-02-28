//go:build !windows && !linux

package svcutil

import "fmt"

func isWindowsService() bool { return false }

func runWindowsService(runFn func(), stopFn func()) error {
	runFn()
	return nil
}

func installWindowsService(exePath string) error {
	return fmt.Errorf("Windows 服务注册仅在 Windows 平台可用")
}

func uninstallWindowsService() error {
	return fmt.Errorf("Windows 服务卸载仅在 Windows 平台可用")
}


