package storage

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// systemUserRe 系统/Samba 用户名允许的字符集。
var systemUserRe = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

// shareNameRe Samba 共享名允许的字符集。
var shareNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// validateSystemUsername 校验用户名可安全地作为 useradd/smbpasswd 的参数，
// 并可安全写入 smb.conf 的 valid users 行。
//
// 以 '-' 开头的名字会被这些命令当作选项解析（如 --groups=root），
// 含换行的名字则能向 smb.conf 注入任意指令。
func validateSystemUsername(name string) error {
	if !systemUserRe.MatchString(name) {
		return fmt.Errorf("用户名不合法：仅允许小写字母、数字、下划线和连字符，须以字母或下划线开头，最长 32 位")
	}
	return nil
}

// validateShareName 校验共享名，防止借助换行向 smb.conf 注入额外配置段。
func validateShareName(name string) error {
	if !shareNameRe.MatchString(name) {
		return fmt.Errorf("共享名不合法：仅允许字母、数字、点、下划线和连字符，最长 64 位")
	}
	return nil
}

// ValidateRootPath 校验共享根目录。
//
// 网络存储会以面板进程权限（通常为 root/Administrator）读写该目录，
// 因此根目录必须真实存在且为目录，并拒绝换行符（换行可向 smb.conf 注入
// 任意配置指令）。
func ValidateRootPath(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf("共享根目录不能为空")
	}
	if strings.ContainsAny(root, "\r\n") {
		return "", fmt.Errorf("共享根目录不能包含换行符")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("无效的共享根目录: %w", err)
	}
	abs = filepath.Clean(abs)

	// 解析符号链接，避免用软链绕过目录校验
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = filepath.Clean(resolved)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("共享根目录不存在或不可访问: %s", abs)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("共享根目录必须是目录: %s", abs)
	}
	return abs, nil
}

// ValidateCredentials 强制要求为网络存储配置访问凭据。
//
// 原实现允许用户名为空：WebDAV/SFTP 会放行任意登录（SFTP 甚至接受任意口令），
// SMB 则退化为匿名访客共享。结合「任意登录用户可创建指向任意目录的存储」，
// 等价于把整盘以匿名方式暴露到网络上，因此这里强制要求用户名与密码。
func ValidateCredentials(username, password, protocol string) error {
	if strings.TrimSpace(username) == "" {
		return fmt.Errorf("%s 服务必须配置访问用户名（留空将允许任意人无需凭据访问该目录）", protocol)
	}
	if password == "" {
		return fmt.Errorf("%s 服务必须配置访问密码", protocol)
	}
	return nil
}

// ValidateListenAddr 校验监听地址必须是合法 IP（或留空表示 0.0.0.0）。
func ValidateListenAddr(addr string) error {
	addr = strings.TrimSpace(addr)
	if addr == "" || addr == "0.0.0.0" || addr == "::" {
		return nil
	}
	if net.ParseIP(addr) == nil {
		return fmt.Errorf("监听地址必须是合法的 IP 地址: %s", addr)
	}
	return nil
}

// ValidateListenPort 校验监听端口范围。
func ValidateListenPort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("监听端口必须在 1~65535 之间: %d", port)
	}
	return nil
}
