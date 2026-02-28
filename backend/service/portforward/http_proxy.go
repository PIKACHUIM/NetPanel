package portforward

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// HTTPProxy 处理 HTTP / HTTPS / WebSocket 的反向代理
// - ListenPortType 为 http      → 监听 HTTP，转发到 http://target
// - ListenPortType 为 https     → 监听 HTTPS（TLS 终止在本地），转发到 http(s)://target
// - ListenPortType 为 websocket → 同 http，ReverseProxy 自动处理 Upgrade
type HTTPProxy struct {
	listenIP   string
	listenPort int
	targetURL  *url.URL // 目标地址，含 scheme

	// TLS 证书（仅 HTTPS 监听时使用）
	certFile string
	keyFile  string

	server     *http.Server
	serverMu   sync.Mutex
	trafficIn  int64
	trafficOut int64
	log        *logrus.Logger
}

// newHTTPProxy 构造 HTTPProxy
// scheme 应为 "http" 或 "https"（决定转发到目标时使用的协议）
// certFile/keyFile 仅在本地监听 HTTPS 时需要，为空则以 HTTP 方式监听
func newHTTPProxy(listenIP, targetAddr string, listenPort, targetPort int, scheme, certFile, keyFile string, log *logrus.Logger) *HTTPProxy {
	rawURL := fmt.Sprintf("%s://%s:%d", scheme, targetAddr, targetPort)
	target, err := url.Parse(rawURL)
	if err != nil {
		log.Errorf("[HTTP代理] 解析目标地址失败: %v", err)
	}
	return &HTTPProxy{
		listenIP:   listenIP,
		listenPort: listenPort,
		targetURL:  target,
		certFile:   certFile,
		keyFile:    keyFile,
		log:        log,
	}
}

func (p *HTTPProxy) Start() error {
	p.serverMu.Lock()
	defer p.serverMu.Unlock()
	if p.server != nil {
		return nil
	}

	rp := httputil.NewSingleHostReverseProxy(p.targetURL)

	// 自定义 Transport：支持转发到 HTTPS 目标时跳过证书验证（内网场景）
	rp.Transport = &countingTransport{
		inner: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		trafficIn:  &p.trafficIn,
		trafficOut: &p.trafficOut,
	}

	// 错误处理
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		p.log.Errorf("[HTTP代理] 转发请求 %s 失败: %v", r.URL, err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}

	// 修改请求 Host，使目标服务器能正确路由
	origDirector := rp.Director
	rp.Director = func(req *http.Request) {
		origDirector(req)
		req.Host = p.targetURL.Host
	}

	addr := fmt.Sprintf("%s:%d", p.listenIP, p.listenPort)
	p.server = &http.Server{
		Addr:              addr,
		Handler:           rp,
		ReadHeaderTimeout: 30 * time.Second,
	}

	// 判断是否需要 TLS 监听
	if p.certFile != "" && p.keyFile != "" {
		// 加载证书
		cert, err := tls.LoadX509KeyPair(p.certFile, p.keyFile)
		if err != nil {
			p.server = nil
			return fmt.Errorf("[HTTPS代理] 加载证书失败 (cert=%s key=%s): %w", p.certFile, p.keyFile, err)
		}
		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		ln, err := tls.Listen("tcp", addr, tlsCfg)
		if err != nil {
			p.server = nil
			return fmt.Errorf("[HTTPS代理] TLS 监听 %s 失败: %w", addr, err)
		}
		p.log.Infof("[端口转发][HTTPS] 开始 TLS 监听 %s -> %s", addr, p.targetURL)
		go func() {
			if err := p.server.Serve(ln); err != nil && err != http.ErrServerClosed {
				p.log.Errorf("[HTTPS代理] Serve 错误: %v", err)
			}
		}()
	} else {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			p.server = nil
			return fmt.Errorf("[HTTP代理] 监听 %s 失败: %w", addr, err)
		}
		p.log.Infof("[端口转发][HTTP] 开始监听 %s -> %s", addr, p.targetURL)
		go func() {
			if err := p.server.Serve(ln); err != nil && err != http.ErrServerClosed {
				p.log.Errorf("[HTTP代理] Serve 错误: %v", err)
			}
		}()
	}
	return nil
}

func (p *HTTPProxy) Stop() {
	p.serverMu.Lock()
	defer p.serverMu.Unlock()
	if p.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := p.server.Shutdown(ctx); err != nil {
			p.log.Errorf("[HTTP代理] Shutdown 错误: %v", err)
		}
		p.server = nil
		p.log.Infof("[端口转发][HTTP] 停止监听 %s:%d", p.listenIP, p.listenPort)
	}
}

func (p *HTTPProxy) GetStatus() string {
	p.serverMu.Lock()
	defer p.serverMu.Unlock()
	if p.server != nil {
		return "running"
	}
	return "stopped"
}

func (p *HTTPProxy) GetTrafficIn() int64  { return atomic.LoadInt64(&p.trafficIn) }
func (p *HTTPProxy) GetTrafficOut() int64 { return atomic.LoadInt64(&p.trafficOut) }

// ===== countingTransport：统计流量 =====

type countingTransport struct {
	inner      http.RoundTripper
	trafficIn  *int64 // 请求体（上行）
	trafficOut *int64 // 响应体（下行）
}

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 统计请求体大小（上行）
	if req.ContentLength > 0 {
		atomic.AddInt64(t.trafficIn, req.ContentLength)
	}

	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// 统计响应体大小（下行）
	if resp.ContentLength > 0 {
		atomic.AddInt64(t.trafficOut, resp.ContentLength)
	}
	return resp, nil
}
