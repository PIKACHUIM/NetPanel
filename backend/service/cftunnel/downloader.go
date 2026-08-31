package cftunnel

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/sirupsen/logrus"
)

const (
	// cloudflaredVersion 默认下载版本
	cloudflaredVersion = "2024.8.2"
	// downloadBaseURL GitHub Release 下载基础 URL
	downloadBaseURL = "https://github.com/cloudflare/cloudflared/releases/download"
)

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
	return url, filename, nil
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
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("下载请求失败: %w", err)
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
		"error":     func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}(),
	}
}
