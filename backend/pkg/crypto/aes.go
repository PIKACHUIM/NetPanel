package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

const (
	// EnvSecretKey 加密密钥环境变量名
	EnvSecretKey = "NETPANEL_SECRET_KEY"
	// MinKeyLength AES-256 要求密钥至少 32 字节
	MinKeyLength = 32
)

var (
	// globalKey 全局加密密钥，启动时初始化
	globalKey []byte
	// keyInitialized 密钥是否已初始化
	keyInitialized bool
)

// InitKey 从环境变量加载并验证加密密钥。
// 此函数主要供测试和显式环境配置场景使用。
func InitKey() error {
	key := os.Getenv(EnvSecretKey)
	if key == "" {
		return errors.New("secret key not found: set " + EnvSecretKey + " or use InitKeyFromDataDir")
	}
	return setKey([]byte(key))
}

// InitKeyFromDataDir 加载用户提供的加密密钥；未提供密钥时关闭字段级加密。
// secret.key 与 NETPANEL_SECRET_KEY 同时缺失时，Secret 字段将以明文存储。
func InitKeyFromDataDir(dataDir string) error {
	if key := os.Getenv(EnvSecretKey); key != "" {
		return setKey([]byte(key))
	}

	keyPath := filepath.Join(dataDir, "secret.key")
	key, err := os.ReadFile(keyPath)
	if err == nil {
		if err := setKey(key); err != nil {
			return errors.New("invalid persisted secret key: " + err.Error())
		}
		if runtime.GOOS != "windows" {
			if err := os.Chmod(keyPath, 0600); err != nil {
				return errors.New("secure secret key file permissions: " + err.Error())
			}
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return errors.New("read persisted secret key: " + err.Error())
	}

	clearKey()
	return nil
}

func clearKey() {
	for i := range globalKey {
		globalKey[i] = 0
	}
	globalKey = nil
	keyInitialized = false
}

func setKey(key []byte) error {
	if len(key) != MinKeyLength {
		globalKey = nil
		keyInitialized = false
		return errors.New("secret key must be exactly " + strconv.Itoa(MinKeyLength) + " bytes for AES-256")
	}
	globalKey = append(globalKey[:0], key...)
	keyInitialized = true
	return nil
}

// IsInitialized 返回加密模块是否已初始化
func IsInitialized() bool {
	return keyInitialized
}

// Encrypt 使用 AES-256-GCM 加密明文，返回 base64 编码的密文（含 nonce）
func Encrypt(plaintext string) (string, error) {
	if !keyInitialized {
		return "", errors.New("crypto: key not initialized")
	}
	block, err := aes.NewCipher(globalKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密 base64 编码的密文，返回原始明文
func Decrypt(ciphertextB64 string) (string, error) {
	if !keyInitialized {
		return "", errors.New("crypto: key not initialized")
	}
	data, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(globalKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// EncryptIfKeyAvailable 加密明文；密钥未初始化时失败关闭。
func EncryptIfKeyAvailable(plaintext string) (string, error) {
	if !keyInitialized {
		return "", errors.New("crypto: key not initialized; refusing plaintext fallback")
	}
	return Encrypt(plaintext)
}

// DecryptIfExists 有值则尝试解密，解密失败返回原值（兼容旧明文数据）
func DecryptIfExists(ciphertextB64 string) string {
	if ciphertextB64 == "" || !keyInitialized {
		return ciphertextB64
	}
	plain, err := Decrypt(ciphertextB64)
	if err != nil {
		// 可能是旧明文数据，直接返回
		return ciphertextB64
	}
	return plain
}
