//go:build linux

package svcutil

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

const systemdUnitTemplate = `[Unit]
Description={{.Description}}
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart={{.ExecPath}} --service
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier={{.Name}}

[Install]
WantedBy=multi-user.target
`

type unitData struct {
	Name        string
	Description string
	ExecPath    string
}

func installLinuxService(exePath string) error {
	unitPath := filepath.Join("/etc/systemd/system", ServiceName+".service")

	// 渲染 unit 文件
	tmpl, err := template.New("unit").Parse(systemdUnitTemplate)
	if err != nil {
		return fmt.Errorf("解析 unit 模板失败: %w", err)
	}

	f, err := os.Create(unitPath)
	if err != nil {
		return fmt.Errorf("创建 unit 文件失败（需要 root 权限）: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, unitData{
		Name:        ServiceName,
		Description: ServiceDescription,
		ExecPath:    exePath,
	}); err != nil {
		return fmt.Errorf("写入 unit 文件失败: %w", err)
	}

	// 重载 systemd 并启用服务
	if err := runCmd("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload 失败: %w", err)
	}
	if err := runCmd("systemctl", "enable", ServiceName); err != nil {
		return fmt.Errorf("systemctl enable 失败: %w", err)
	}

	fmt.Fprintf(os.Stdout, "✅ 服务 %q 注册成功: %s\n", ServiceName, unitPath)
	fmt.Fprintf(os.Stdout, "   使用 'systemctl start %s' 启动服务\n", ServiceName)
	fmt.Fprintf(os.Stdout, "   使用 'journalctl -u %s -f' 查看日志\n", ServiceName)
	return nil
}

func uninstallLinuxService() error {
	// 先停止服务（忽略错误，服务可能未运行）
	_ = runCmd("systemctl", "stop", ServiceName)
	_ = runCmd("systemctl", "disable", ServiceName)

	unitPath := filepath.Join("/etc/systemd/system", ServiceName+".service")
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除 unit 文件失败: %w", err)
	}

	_ = runCmd("systemctl", "daemon-reload")

	fmt.Fprintf(os.Stdout, "✅ 服务 %q 已卸载\n", ServiceName)
	return nil
}
