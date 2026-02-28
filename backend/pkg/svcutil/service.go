// Package svcutil 提供跨平台系统服务（daemon）注册与运行支持。
// Windows 使用 golang.org/x/sys/windows/svc，Linux/macOS 使用 os/signal 守护进程模式。
package svcutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const ServiceName = "netpanel"
const ServiceDisplayName = "NetPanel Network Manager"
const ServiceDescription = "NetPanel 网络管理面板服务"

// IsWindowsService 判断当前是否以 Windows 服务方式运行
func IsWindowsService() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	return isWindowsService()
}

// RunService 以服务模式运行（阻塞），runFn 为实际业务逻辑
func RunService(runFn func(), stopFn func()) error {
	if runtime.GOOS == "windows" {
		return runWindowsService(runFn, stopFn)
	}
	// Linux/macOS：直接运行，信号由 main 处理
	runFn()
	return nil
}

// InstallService 注册系统服务
func InstallService() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("解析可执行文件路径失败: %w", err)
	}

	switch runtime.GOOS {
	case "windows":
		return installWindowsService(exePath)
	case "linux":
		return installLinuxService(exePath)
	default:
		return fmt.Errorf("当前平台 %s 不支持自动注册服务，请手动配置", runtime.GOOS)
	}
}

// UninstallService 卸载系统服务
func UninstallService() error {
	switch runtime.GOOS {
	case "windows":
		return uninstallWindowsService()
	case "linux":
		return uninstallLinuxService()
	default:
		return fmt.Errorf("当前平台 %s 不支持自动卸载服务", runtime.GOOS)
	}
}

// StartService 启动已注册的系统服务
func StartService() error {
	switch runtime.GOOS {
	case "windows":
		return runCmd("sc", "start", ServiceName)
	case "linux":
		return runCmd("systemctl", "start", ServiceName)
	default:
		return fmt.Errorf("不支持的平台: %s", runtime.GOOS)
	}
}

// StopService 停止已注册的系统服务
func StopService() error {
	switch runtime.GOOS {
	case "windows":
		return runCmd("sc", "stop", ServiceName)
	case "linux":
		return runCmd("systemctl", "stop", ServiceName)
	default:
		return fmt.Errorf("不支持的平台: %s", runtime.GOOS)
	}
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
