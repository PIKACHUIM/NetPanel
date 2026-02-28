//go:build windows

package svcutil

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// isWindowsService 判断当前进程是否以 Windows 服务方式运行
func isWindowsService() bool {
	inSvc, err := svc.IsWindowsService()
	if err != nil {
		return false
	}
	return inSvc
}

// netpanelSvc 实现 svc.Handler 接口
type netpanelSvc struct {
	runFn  func()
	stopFn func()
}

func (s *netpanelSvc) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	changes <- svc.Status{State: svc.StartPending}

	// 在独立 goroutine 中运行业务逻辑
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.runFn()
	}()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				s.stopFn()
				break loop
			}
		case <-done:
			break loop
		}
	}

	changes <- svc.Status{State: svc.Stopped}
	return false, 0
}

// runWindowsService 以 Windows 服务模式运行（阻塞）
func runWindowsService(runFn func(), stopFn func()) error {
	handler := &netpanelSvc{runFn: runFn, stopFn: stopFn}
	return svc.Run(ServiceName, handler)
}

// installWindowsService 向 Windows SCM 注册服务
func installWindowsService(exePath string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("连接 SCM 失败: %w", err)
	}
	defer m.Disconnect()

	// 检查服务是否已存在
	s, err := m.OpenService(ServiceName)
	if err == nil {
		s.Close()
		return fmt.Errorf("服务 %q 已存在，请先卸载", ServiceName)
	}

	s, err = m.CreateService(
		ServiceName,
		exePath,
		mgr.Config{
			DisplayName:      ServiceDisplayName,
			Description:      ServiceDescription,
			StartType:        mgr.StartAutomatic,
			ServiceStartName: "LocalSystem",
		},
		// 传递 --service 标志，让程序知道自己以服务模式运行
		"--service",
	)
	if err != nil {
		return fmt.Errorf("创建服务失败: %w", err)
	}
	defer s.Close()

	fmt.Fprintf(os.Stdout, "✅ 服务 %q 注册成功\n", ServiceName)
	fmt.Fprintf(os.Stdout, "   使用 'sc start %s' 或 'net start %s' 启动服务\n", ServiceName, ServiceName)
	return nil
}

// uninstallWindowsService 从 Windows SCM 卸载服务
func uninstallWindowsService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("连接 SCM 失败: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(ServiceName)
	if err != nil {
		return fmt.Errorf("服务 %q 不存在: %w", ServiceName, err)
	}
	defer s.Close()

	if err := s.Delete(); err != nil {
		return fmt.Errorf("删除服务失败: %w", err)
	}

	fmt.Fprintf(os.Stdout, "✅ 服务 %q 已卸载\n", ServiceName)
	return nil
}
