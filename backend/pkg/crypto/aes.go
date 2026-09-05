package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
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

// InitKey 从环境变量加载并验证加密密钥
func InitKey() error {
	key := os.Getenv(EnvSecretKey)
	if key == "" {
		return errors.New("secret key not found: set " + EnvSecretKey + " environment variable")
	}
	if len([]byte(key)) < MinKeyLength {
		return errors.New("secret key too short: must be at least " + strconv.Itoa(MinKeyLength) + " bytes")
	}
	globalKey = []byte(key)
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

// EncryptIfKeyAvailable 有密钥则加密，否则返回原值（兼容未配置密钥场景）
func EncryptIfKeyAvailable(plaintext string) (string, error) {
	if !keyInitialized {
		return plaintext, nil
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
