package api

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// 鉴权矩阵回归守卫：宿主机级写操作必须挂在 admin 组（JWTAuth+AdminOnly）。
// 此前 firewall/wireguard/caddy/storage 等高危写操作仅要求登录（auth 组），
// 构成提权面；本测试通过扫描路由注册源码防止此类回归。
//
// 注意：这是"源码约定"级别的守卫，真正的行为由 middleware 的单元测试
// （auth_test.go）与集成测试保证；两份测试互为补充。

// adminRequiredPrefixes 这些资源前缀的 POST/PUT/DELETE/PATCH 注册必须使用 admin 组
var adminRequiredPrefixes = []string{
	"/system/config",     // 系统配置（含提权链历史）
	"/port-forward",      // 宿主机端口监听
	"/stun",              // UPnP/打洞
	"/frpc",              // 进程
	"/frps",              // 进程
	"/nps/server",        // 进程
	"/nps/client",        // 进程
	"/easytier/client",   // 进程/可选 TUN
	"/easytier/server",   // 进程
	"/tunservice",        // 启停线路客户端进程
	"/wireguard",         // PostUp/PreUp 可执行 shell
	"/caddy",             // 反代/证书行为
	"/cftunnel",          // 进程 + 二进制下载
	"/storage",           // 系统用户/smb.conf/监听端口
	"/security/firewall", // iptables 级操作
	"/mesh/proxy",        // 代理到远程节点
}

var writeMethodRe = regexp.MustCompile(`^\s*(auth|admin)\.(POST|PUT|DELETE|PATCH|Any)\("([^"]+)"`)

func TestHostLevelWriteRoutesRequireAdmin(t *testing.T) {
	lines := strings.Split(routerDotGo, "\n")
	if len(lines) < 50 {
		t.Fatal("无法读取 router.go")
	}
	for _, line := range lines {
		m := writeMethodRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		group, path := m[1], m[3]
		if group != "auth" {
			continue
		}
		for _, prefix := range adminRequiredPrefixes {
			if path == prefix || len(path) > len(prefix) && path[:len(prefix)] == prefix {
				t.Errorf("路由 %q 注册在 auth 组，宿主机级写操作必须使用 admin 组: %s", path, strings.TrimSpace(line))
			}
		}
	}
}

var routerDotGo = `
` + readRouterDotGo()

func readRouterDotGo() string {
	b, err := os.ReadFile("router.go")
	if err != nil {
		return ""
	}
	return string(b)
}
