package caddy

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// siteRootWhitelist 静态站点根目录白名单。
//
// 站点配置由普通登录用户提交，若不加限制，创建一个 root_path=/ 且开启
// 目录浏览的静态站点即可通过 HTTP 暴露整个磁盘，等同于任意文件读取。
// 因此静态站点根目录必须落在这些受控目录之内。
var siteRootWhitelist []string

// SetSiteRootWhitelist 配置允许作为静态站点根目录的父目录集合。
func SetSiteRootWhitelist(dirs []string) {
	siteRootWhitelist = siteRootWhitelist[:0]
	for _, d := range dirs {
		if d == "" {
			continue
		}
		abs, err := filepath.Abs(d)
		if err != nil {
			continue
		}
		siteRootWhitelist = append(siteRootWhitelist, filepath.Clean(abs))
	}
}

// ValidateRootPath 校验静态站点根目录合法且位于白名单目录内，返回规范化后的绝对路径。
func ValidateRootPath(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("静态文件根目录不能为空")
	}

	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("无效的根目录: %w", err)
	}
	abs = filepath.Clean(abs)

	// 解析符号链接，防止用白名单内的软链指向白名单外的目录
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = filepath.Clean(resolved)
	}

	if len(siteRootWhitelist) == 0 {
		return "", fmt.Errorf("未配置静态站点根目录白名单，拒绝创建静态站点")
	}
	for _, base := range siteRootWhitelist {
		if withinDir(base, abs) {
			return abs, nil
		}
	}
	return "", fmt.Errorf("根目录 %s 不在允许的范围内，请使用 %s 下的子目录", abs, strings.Join(siteRootWhitelist, ", "))
}

// withinDir 判断 target 是否等于 base 或位于 base 之下。
// 使用 filepath.Rel 而非字符串前缀比较，避免 /data 误匹配 /database。
func withinDir(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != ".."
}

// ValidateRedirectTarget 校验重定向目标，避免站点变成开放重定向跳板。
// 只接受站内相对路径，或 http/https 且带明确主机名的绝对地址。
func ValidateRedirectTarget(target string) error {
	t := strings.TrimSpace(target)
	if t == "" {
		return fmt.Errorf("重定向目标地址不能为空")
	}
	// 排除 //evil.com 这种协议相对地址：浏览器会当作跨站绝对地址处理
	if strings.HasPrefix(t, "//") {
		return fmt.Errorf("重定向目标不能以 // 开头")
	}
	if strings.HasPrefix(t, "/") {
		return nil
	}

	u, err := url.Parse(t)
	if err != nil {
		return fmt.Errorf("重定向目标地址格式无效: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("重定向目标仅支持 http/https 协议")
	}
	if u.Host == "" {
		return fmt.Errorf("重定向目标缺少主机名")
	}
	return nil
}
