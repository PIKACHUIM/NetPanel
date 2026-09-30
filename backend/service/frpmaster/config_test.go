package frpmaster

import (
	"strings"
	"testing"
)

// TestGenerateFrpcTomlMinimal 最小配置：仅 [common]，无 [[proxies]]。
func TestGenerateFrpcTomlMinimal(t *testing.T) {
	out := GenerateFrpcToml(ClientConfig{
		ServerAddr: "frp.example.com",
		ServerPort: 7000,
		Token:      "secret-token",
		TLSEnable:  true,
	})
	for _, want := range []string{
		`serverAddr = "frp.example.com"`,
		"serverPort = 7000",
		"[auth]",
		`token = "secret-token"`,
		"[tls]",
		"enable = true",
		"[log]",
		`level = "info"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("输出缺少 %q，完整输出:\n%s", want, out)
		}
	}
	if strings.Contains(out, "[[proxies]]") {
		t.Errorf("无代理配置时不应输出 [[proxies]]，完整输出:\n%s", out)
	}
}

// TestGenerateFrpcTomlDefaultPort serverPort<=0 时回退默认 7000。
func TestGenerateFrpcTomlDefaultPort(t *testing.T) {
	out := GenerateFrpcToml(ClientConfig{ServerAddr: "x"})
	if !strings.Contains(out, "serverPort = 7000") {
		t.Errorf("默认端口应为 7000，输出:\n%s", out)
	}
}

// TestGenerateFrpcTomlTCPProxy tcp 代理全字段渲染。
func TestGenerateFrpcTomlTCPProxy(t *testing.T) {
	out := GenerateFrpcToml(ClientConfig{ServerAddr: "x", Proxies: []Proxy{
		{Name: "ssh", Type: "tcp", LocalIP: "127.0.0.1", LocalPort: 22, RemotePort: 6022, UseCompression: true},
	}})
	for _, want := range []string{
		"[[proxies]]",
		`name = "ssh"`,
		`type = "tcp"`,
		`localIP = "127.0.0.1"`,
		"localPort = 22",
		"remotePort = 6022",
		"transport.useCompression = true",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("输出缺少 %q，完整输出:\n%s", want, out)
		}
	}
}

// TestGenerateFrpcTomlHTTPProxy http 代理 customDomains 转 toml 数组。
func TestGenerateFrpcTomlHTTPProxy(t *testing.T) {
	out := GenerateFrpcToml(ClientConfig{ServerAddr: "x", Proxies: []Proxy{
		{Name: "web", Type: "https", LocalIP: "127.0.0.1", LocalPort: 8080, CustomDomains: "a.com, b.com", UseEncryption: true},
	}})
	for _, want := range []string{
		`type = "https"`,
		`customDomains = ["a.com", "b.com"]`,
		"transport.useEncryption = true",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("输出缺少 %q，完整输出:\n%s", want, out)
		}
	}
}

// TestGenerateFrpcTomlSkipInvalidProxies 字段不满足类型要求的代理被跳过并注释。
func TestGenerateFrpcTomlSkipInvalidProxies(t *testing.T) {
	out := GenerateFrpcToml(ClientConfig{ServerAddr: "x", Proxies: []Proxy{
		{Name: "", Type: "tcp"},                                    // 缺 name
		{Name: "a", Type: "tcp"},                                   // 缺 remotePort
		{Name: "b", Type: "http"},                                  // 缺域名
		{Name: "c", Type: "stcp"},                                  // 缺 secretKey
		{Name: "d", Type: "quic"},                                  // 不支持类型
		{Name: "ok", Type: "udp", LocalPort: 53, RemotePort: 5353}, // 合法
	}})
	if !strings.Contains(out, "[[proxies]]") {
		t.Fatalf("应输出合法代理，输出:\n%s", out)
	}
	if strings.Contains(out, `name = "a"`) || strings.Contains(out, `name = "b"`) ||
		strings.Contains(out, `name = "c"`) || strings.Contains(out, `name = "d"`) {
		t.Errorf("非法代理不应被渲染，输出:\n%s", out)
	}
	if strings.Count(out, "# [skip]") < 5 {
		t.Errorf("应有 5 条 skip 注释，实际 %d 条，输出:\n%s", strings.Count(out, "# [skip]"), out)
	}
}

// TestGenerateFrpcTomlQuoting 字符串值需正确转义。
func TestGenerateFrpcTomlQuoting(t *testing.T) {
	out := GenerateFrpcToml(ClientConfig{ServerAddr: `a"b\c`, Proxies: []Proxy{
		{Name: `n"1`, Type: "tcp", RemotePort: 1},
	}})
	if !strings.Contains(out, `serverAddr = "a\"b\\c"`) {
		t.Errorf("serverAddr 转义错误，输出:\n%s", out)
	}
	if !strings.Contains(out, `name = "n\"1"`) {
		t.Errorf("name 转义错误，输出:\n%s", out)
	}
}
