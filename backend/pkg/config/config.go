package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// Config 系统配置
type Config struct {
	DataDir  string
	Debug    bool
	Version  string
	BinDir   string // 存放 easytier 等二进制的目录
	CertDir  string // 证书存储目录
	LogDir   string // 日志目录
}

// Init 初始化配置
func Init(dataDir string) *Config {
	cfg := &Config{
		DataDir: dataDir,
		Debug:   getEnvBool("NETPANEL_DEBUG", false),
		Version: "1.0.0",
		BinDir:  filepath.Join(dataDir, "bin"),
		CertDir: filepath.Join(dataDir, "certs"),
		LogDir:  filepath.Join(dataDir, "logs"),
	}

	// 创建必要目录
	dirs := []string{cfg.BinDir, cfg.CertDir, cfg.LogDir}
	for _, dir := range dirs {
		os.MkdirAll(dir, 0755)
	}

	return cfg
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
