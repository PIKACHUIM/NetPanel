// Package secret 集中管理服务端签名密钥。
//
// 背景：原实现将 JWT 密钥以常量 "netpanel-secret-key-change-in-production"
// 硬编码在 api/middleware/auth.go，并在 platform_auth.go 与
// service/access/manager.go 中重复出现。由于该值随开源代码公开，
// 任何人都可离线伪造 JWT 与 session Cookie，使全站认证失效。
//
// 现改为：进程启动时加载密钥，优先级为
//  1. 环境变量 NETPANEL_JWT_SECRET（便于多实例共享与容器化部署）
//  2. 数据目录下的 jwt_secret 文件（首次启动用 crypto/rand 生成并落盘，权限 0600）
//
// 同时对不同用途做密钥分域（HKDF-like 派生），避免 JWT 与 Cookie 复用同一密钥。
package secret

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	// envJWTSecret 允许通过环境变量注入主密钥
	envJWTSecret = "NETPANEL_JWT_SECRET"
	// secretFileName 主密钥持久化文件名
	secretFileName = "jwt_secret"
	// masterKeyBytes 主密钥长度（32 字节 = 256 位）
	masterKeyBytes = 32
)

var (
	mu        sync.RWMutex
	masterKey []byte
)

// Init 加载或生成主密钥。应在进程启动早期调用一次。
// dataDir 为数据目录；返回错误时调用方应终止启动，避免退化到不安全的默认密钥。
func Init(dataDir string) error {
	mu.Lock()
	defer mu.Unlock()

	// 1. 环境变量优先
	if v := os.Getenv(envJWTSecret); v != "" {
		if len(v) < 16 {
			return fmt.Errorf("%s 长度不足（至少 16 字符）", envJWTSecret)
		}
		masterKey = []byte(v)
		return nil
	}

	// 2. 数据目录中的密钥文件
	path := filepath.Join(dataDir, secretFileName)
	if b, err := os.ReadFile(path); err == nil && len(b) >= masterKeyBytes {
		masterKey = b
		return nil
	}

	// 3. 首次启动：生成随机主密钥并落盘
	key := make([]byte, masterKeyBytes)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("生成签名密钥失败: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}
	// 0600：仅属主可读写，避免同机其他用户读取后伪造令牌
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return fmt.Errorf("持久化签名密钥失败: %w", err)
	}
	masterKey = key
	return nil
}

// InitForTest 供单元测试注入固定密钥，避免测试依赖文件系统。
func InitForTest(key string) {
	mu.Lock()
	defer mu.Unlock()
	masterKey = []byte(key)
}

// derive 按用途派生子密钥，确保不同用途的密钥互相独立：
// 即使某一用途的令牌格式存在缺陷，也不会波及其他用途。
func derive(purpose string) []byte {
	mu.RLock()
	key := masterKey
	mu.RUnlock()

	if len(key) == 0 {
		// Init 未成功调用属于编程错误：宁可 panic 也不能退化为固定密钥
		panic("secret: 主密钥未初始化，请先调用 secret.Init()")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("netpanel/v1/" + purpose))
	return mac.Sum(nil)
}

// JWTKey 返回用于签发/校验 JWT 的密钥。
func JWTKey() []byte { return derive("jwt") }

// SessionKey 返回用于 session Cookie 签名的密钥。
func SessionKey() []byte { return derive("session") }

// Fingerprint 返回主密钥指纹（前 8 字节 hex），用于日志中确认密钥已加载，
// 且不泄露密钥本身。
func Fingerprint() string {
	sum := derive("fingerprint")
	return hex.EncodeToString(sum[:8])
}
