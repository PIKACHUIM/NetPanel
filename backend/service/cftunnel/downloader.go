package cftunnel

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// cloudflaredVersion 默认下载版本
	cloudflaredVersion = "2024.8.2"
	// downloadBaseURL GitHub Release 下载基础 URL
	downloadBaseURL = "https://github.com/cloudflare/cloudflared/releases/download"
	// downloadTimeout 整体下载超时，避免网络挂死
	downloadTimeout = 30 * time.Minute
)

// ghProxyEnv GitHub 加速镜像环境变量（如 https://ghfast.top、https://gh-proxy.com）。
// 国内网络直连 GitHub Releases 下载 cloudflared 常年卡死；配置后下载地址
// 变为 <mirror>/<原始URL>。留空则直连。
const ghProxyEnv = "NETPANEL_GH_MIRROR"

const (
	// caFileEnv 额外信任的 PEM 格式 CA 证书文件（企业代理/自签根证书场景）
	caFileEnv = "NETPANEL_CA_FILE"
	// insecureTLSEnv 设为 1 时跳过 TLS 证书校验，仅用于无法导入根证书的受限环境
	insecureTLSEnv = "NETPANEL_INSECURE_TLS"
)

// newDownloadClient 构造下载用 HTTP 客户端。
// 默认显式使用操作系统根证书池，避免 SSL_CERT_FILE 等环境变量指向
// 不完整的 CA 包时，把合法证书误判为 unknown authority。
func newDownloadClient() *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if pool := buildRootCAs(); pool != nil {
		tlsCfg.RootCAs = pool
	}
	if strings.TrimSpace(os.Getenv(insecureTLSEnv)) == "1" {
		//nolint:gosec // 显式开关：用于无法安装根证书的受限网络，风险需自行承担
		tlsCfg.InsecureSkipVerify = true
		logrus.Warnf("[cftunnel] 已按 %s=1 关闭 TLS 证书校验，存在中间人风险", insecureTLSEnv)
	}
	transport.TLSClientConfig = tlsCfg

	return &http.Client{Transport: transport, Timeout: downloadTimeout}
}

// buildRootCAs 返回系统根证书池，并按需追加 NETPANEL_CA_FILE 中的证书。
func buildRootCAs() *x509.CertPool {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		logrus.Warnf("[cftunnel] 读取系统根证书失败，使用空证书池: %v", err)
		pool = x509.NewCertPool()
	}

	caFile := strings.TrimSpace(os.Getenv(caFileEnv))
	if caFile == "" {
		return pool
	}
	pem, err := os.ReadFile(caFile)
	if err != nil {
		logrus.Warnf("[cftunnel] 读取 %s=%s 失败: %v", caFileEnv, caFile, err)
		return pool
	}
	if !pool.AppendCertsFromPEM(pem) {
		logrus.Warnf("[cftunnel] %s=%s 中未找到可用的 PEM 证书", caFileEnv, caFile)
	}
	return pool
}

// tlsHint 为证书校验类错误补充可操作的处理建议。
func tlsHint(err error) string {
	var unknownAuthority x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	var invalidCert x509.CertificateInvalidError
	if errors.As(err, &unknownAuthority) || errors.As(err, &hostnameErr) || errors.As(err, &invalidCert) {
		return fmt.Sprintf(
			"（TLS 证书校验失败：若处于代理/安全软件中间人环境，请把其根证书导入系统，"+
				"或设置 %s 指向 CA 证书文件；也可设置 %s 使用镜像；"+
				"仅在信任当前网络时可用 %s=1 临时跳过校验）",
			caFileEnv, ghProxyEnv, insecureTLSEnv,
		)
	}
	return ""
}

// applyGHProxy 为 GitHub 下载地址拼接加速镜像前缀（已配置时）
func applyGHProxy(rawURL string) string {
	mirror := strings.TrimRight(strings.TrimSpace(os.Getenv(ghProxyEnv)), "/")
	if mirror == "" || !strings.HasPrefix(rawURL, "https://github.com/") {
		return rawURL
	}
	return mirror + "/" + rawURL
}

// getDownloadURL 根据当前系统获取 cloudflared 下载 URL
func getDownloadURL() (string, string, error) {
	version := cloudflaredVersion
	var filename string
	var url string

	switch runtime.GOOS {
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			filename = "cloudflared-windows-amd64.exe"
		case "386":
			filename = "cloudflared-windows-386.exe"
		default:
			return "", "", fmt.Errorf("unsupported Windows architecture: %s", runtime.GOARCH)
		}
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			filename = "cloudflared-linux-amd64"
		case "386":
			filename = "cloudflared-linux-386"
		case "arm64":
			filename = "cloudflared-linux-arm64"
		case "arm":
			filename = "cloudflared-linux-arm"
		default:
			return "", "", fmt.Errorf("unsupported Linux architecture: %s", runtime.GOARCH)
		}
	case "darwin":
		switch runtime.GOARCH {
		case "amd64":
			filename = "cloudflared-darwin-amd64.tgz"
		case "arm64":
			filename = "cloudflared-darwin-amd64.tgz" // 2024.8.2 版本 arm64 使用 amd64 包（Rosetta）
		default:
			return "", "", fmt.Errorf("unsupported macOS architecture: %s", runtime.GOARCH)
		}
	default:
		return "", "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	url = fmt.Sprintf("%s/%s/%s", downloadBaseURL, version, filename)
	return applyGHProxy(url), filename, nil
}

// DownloadBinary 下载 cloudflared 二进制到指定目录
// 返回最终文件路径和错误
func DownloadBinary(binDir string, progressCallback func(downloaded, total int64)) (string, error) {
	url, filename, err := getDownloadURL()
	if err != nil {
		return "", fmt.Errorf("无法获取下载 URL: %w", err)
	}

	logrus.Infof("开始下载 cloudflared: %s", url)

	// 确保 bin 目录存在
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("创建 bin 目录失败: %w", err)
	}

	// 下载到临时文件
	tempFile := filepath.Join(binDir, filename+".tmp")
	finalPath := filepath.Join(binDir, binaryName)
	if runtime.GOOS == "windows" {
		finalPath += ".exe"
	}

	// 发送 HTTP GET 请求
	resp, err := newDownloadClient().Get(url)
	if err != nil {
		return "", fmt.Errorf("下载请求失败: %w%s", err, tlsHint(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	// 创建临时文件
	out, err := os.Create(tempFile)
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer out.Close()

	// 下载并报告进度
	totalSize := resp.ContentLength
	downloaded := int64(0)
	buf := make([]byte, 32*1024) // 32KB 缓冲区

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				os.Remove(tempFile)
				return "", fmt.Errorf("写入文件失败: %w", writeErr)
			}
			downloaded += int64(n)
			if progressCallback != nil {
				progressCallback(downloaded, totalSize)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			os.Remove(tempFile)
			return "", fmt.Errorf("读取响应失败: %w", err)
		}
	}

	out.Close()

	// 重命名为最终文件名
	if err := os.Rename(tempFile, finalPath); err != nil {
		os.Remove(tempFile)
		return "", fmt.Errorf("重命名文件失败: %w", err)
	}

	// 设置可执行权限（Unix 系统）
	if runtime.GOOS != "windows" {
		if err := os.Chmod(finalPath, 0755); err != nil {
			return "", fmt.Errorf("设置可执行权限失败: %w", err)
		}
	}

	logrus.Infof("cloudflared 下载成功: %s (%.2f MB)", finalPath, float64(downloaded)/(1024*1024))
	return finalPath, nil
}

// IsBinaryDownloadSupported 检查当前平台是否支持自动下载
func IsBinaryDownloadSupported() bool {
	_, _, err := getDownloadURL()
	return err == nil
}

// GetDownloadInfo 获取下载信息（用于前端显示）
func GetDownloadInfo() map[string]interface{} {
	url, filename, err := getDownloadURL()
	supported := err == nil

	return map[string]interface{}{
		"supported": supported,
		"url":       url,
		"filename":  filename,
		"version":   cloudflaredVersion,
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
		"error": func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}(),
	}
}
