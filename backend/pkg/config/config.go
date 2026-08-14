package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Config 系统配置
type Config struct {
	DataDir  string
	Debug    bool
	Version  string
	BinDir   string // 存放 easytier 等二进制的目录
	CertDir  string // 证书存储目录
	LogDir   string // 日志目录
	MCPToken string // MCP 服务端访问令牌（Authorization: Bearer）
}

// Init 初始化配置
func Init(dataDir string) *Config {
	cfg := &Config{
		DataDir:  dataDir,
		Debug:    getEnvBool("NETPANEL_DEBUG", false),
		Version:  "1.0.0",
		BinDir:   filepath.Join(dataDir, "bin"),
		CertDir:  filepath.Join(dataDir, "certs"),
		LogDir:   filepath.Join(dataDir, "logs"),
		MCPToken: os.Getenv("NETPANEL_MCP_TOKEN"),
	}

	// MCP token 未配置时生成随机令牌，避免默认空令牌暴露管理接口
	if cfg.MCPToken == "" {
		cfg.MCPToken = generateToken()
	}

	// 创建必要目录
	dirs := []string{cfg.BinDir, cfg.CertDir, cfg.LogDir}
	for _, dir := range dirs {
		os.MkdirAll(dir, 0755)
	}

	return cfg
}

// generateToken 生成随机访问令牌（32 位十六进制）
func generateToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 极低概率：回退到时间戳+进程号，仍保证非空
		return fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())
	}
	return hex.EncodeToString(b)
}

func getEnvBool(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return defaultVal
	}
	return b
}
