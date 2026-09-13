package storage

import (
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/netpanel/netpanel/model"
)

var (
	// smbNameRe SMB 共享名：仅字母/数字/下划线/连字符。共享名未清洗直接写入
	// smb.conf 时，换行可注入任意指令（如 root preexec，以 root 执行命令）。
	smbNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	// sysUserRe 系统用户名（useradd argv / smb.conf valid users / SFTP 账号）
	sysUserRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]{0,31}$`)
)

// ValidateConfig 校验存储配置的安全相关字段。Create/Update handler 与
// Start 都必须调用（前者给用户即时反馈，后者纵深防御）。
//
// 已知限制：SFTP 基于 sftp.NewServer 以绝对路径服务，未做文件系统沙箱，
// 认证通过后可在 RootPath 之外读写（面板进程权限）。因此 SFTP 强制要求
// 用户名密码，且 ListenAddr 建议绑定内网地址；完整沙箱（request-server
// 自定义 Handler）见 ROADMAP 长期池。
func ValidateConfig(cfg *model.StorageConfig) error {
	root := strings.TrimSpace(cfg.RootPath)
	if root == "" || !filepath.IsAbs(root) {
		return fmt.Errorf("RootPath 必须为绝对路径")
	}
	if cleaned := filepath.Clean(root); cleaned != root {
		return fmt.Errorf("RootPath 格式不合法（包含 .. 或冗余分隔符）")
	}
	if cfg.ListenAddr != "" {
		if net.ParseIP(cfg.ListenAddr) == nil {
			return fmt.Errorf("ListenAddr 必须为合法 IP 或留空")
		}
	}

	switch cfg.Protocol {
	case "smb":
		name := strings.TrimSpace(cfg.Name)
		if name != "" && !smbNameRe.MatchString(name) {
			return fmt.Errorf("共享名仅允许字母、数字、下划线和连字符（最长 64 位）")
		}
		if cfg.Username != "" && !sysUserRe.MatchString(cfg.Username) {
			return fmt.Errorf("用户名格式不合法")
		}
	case "sftp":
		// SFTP 无文件系统沙箱，匿名访问等于向局域网开放面板权限的整个磁盘
		if cfg.Username == "" || cfg.Password == "" {
			return fmt.Errorf("SFTP 必须配置用户名和密码（当前实现不支持匿名访问）")
		}
		if !sysUserRe.MatchString(cfg.Username) {
			return fmt.Errorf("用户名格式不合法")
		}
	case "webdav":
		if cfg.Username != "" && !sysUserRe.MatchString(cfg.Username) {
			return fmt.Errorf("用户名格式不合法")
		}
	default:
		return fmt.Errorf("不支持的协议: %s", cfg.Protocol)
	}
	return nil
}
