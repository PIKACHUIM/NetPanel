package caddy

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// adminEndpoint 描述 Caddy Admin API 的监听方式与访问方式。
//
// Caddy 的 Admin API 可以无认证地替换整份配置（包括挂载 file_server root:/
// 或反代到内网），因此绝不能监听在对外可达的 TCP 端口上。
// Unix 平台改用仅属主可读写的 unix socket；Windows 无此机制，
// 退回回环 TCP 并通过 Origins 限制，尽量收窄攻击面。
type adminEndpoint struct {
	// listen 传给 Caddy AdminConfig.Listen 的地址
	listen string
	// baseURL 客户端请求前缀
	baseURL string
	// socketPath 非空时表示走 unix socket
	socketPath string
}

func newAdminEndpoint(dataDir string) adminEndpoint {
	if runtime.GOOS == "windows" {
		return adminEndpoint{
			listen:  "127.0.0.1:2019",
			baseURL: "http://127.0.0.1:2019",
		}
	}
	sock := filepath.Join(dataDir, "caddy-admin.sock")
	// 残留的 socket 文件会导致监听失败，启动前先清理
	_ = os.Remove(sock)
	return adminEndpoint{
		// 0600：仅运行 NetPanel 的用户可以访问 Admin API
		listen:     "unix/" + sock + "|0600",
		baseURL:    "http://caddy-admin",
		socketPath: sock,
	}
}

// origins 限制 Admin API 接受的 Host，抵御 DNS rebinding 与跨源请求。
func (e adminEndpoint) origins() []string {
	if e.socketPath != "" {
		return []string{"caddy-admin"}
	}
	return []string{"127.0.0.1:2019"}
}

// httpClient 构造访问 Admin API 的客户端，unix socket 场景下替换拨号方式。
func (e adminEndpoint) httpClient() *http.Client {
	c := &http.Client{Timeout: 10 * time.Second}
	if e.socketPath == "" {
		return c
	}
	c.Transport = &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", e.socketPath)
		},
	}
	return c
}
