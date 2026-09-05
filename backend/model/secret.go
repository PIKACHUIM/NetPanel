// Package model 敏感字段序列化辅助。
//
// 背景：MonitorServer.SSHPassword、AiProvider.ApiKey、FrpcConfig.Token 等凭据字段
// 原先直接随模型序列化返回给前端，任何登录用户（甚至因鉴权缺陷导致的未认证者）
// 都可通过 List 接口批量读取全部明文凭据。
//
// 这里提供 Secret 类型：
//   - 序列化（写给前端）时输出掩码，不泄露原文；
//   - 反序列化（前端提交）时按原文接收；空字符串表示"不修改"，由 handler 负责处理；
//   - 数据库读写透明（实现 driver.Valuer 与 sql.Scanner），不改变既有存储格式。
package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/netpanel/netpanel/pkg/crypto"
)

// secretMask 前端展示用的掩码。使用固定长度，避免泄露原文长度信息。
const secretMask = "********"

// Secret 敏感字符串。JSON 序列化时输出掩码，反序列化时接收原文。
type Secret string

// String 返回原文，供服务层使用。
func (s Secret) String() string { return string(s) }

// IsEmpty 判断是否为空。
func (s Secret) IsEmpty() bool { return s == "" }

// MarshalJSON 输出掩码而非原文；为空时输出空字符串，
// 便于前端区分"未配置"与"已配置"。
func (s Secret) MarshalJSON() ([]byte, error) {
	if s == "" {
		return []byte(`""`), nil
	}
	return []byte(`"` + secretMask + `"`), nil
}

// UnmarshalJSON 接收前端提交的原文。
// 若提交的正是掩码，说明前端回显了未修改的值，此处置空表示"不修改"，
// 避免把掩码字符串当作真实凭据写入数据库。
func (s *Secret) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	if v == secretMask {
		*s = ""
		return nil
	}
	*s = Secret(v)
	return nil
}

// Value 实现 driver.Valuer：以原文存入数据库，保持既有存储格式不变。
func (s Secret) Value() (driver.Value, error) { return string(s), nil }

// Scan 实现 sql.Scanner：从数据库读取原文。
func (s *Secret) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*s = ""
	case string:
		*s = Secret(v)
	case []byte:
		*s = Secret(v)
	default:
		return fmt.Errorf("无法将 %T 扫描为 Secret", src)
	}
	return nil
}

// EncryptValue 加密 Secret 为 base64 密文字符串（未初始化密钥时原样返回）
func (s Secret) EncryptValue() (string, error) {
	if s == "" {
		return "", nil
	}
	if !crypto.IsInitialized() {
		return string(s), nil
	}
	return crypto.Encrypt(string(s))
}

// DecryptValue 解密密文字符串为 Secret（解密失败时返回原值，兼容旧明文数据）
func (s Secret) DecryptValue() Secret {
	if s == "" {
		return ""
	}
	return Secret(crypto.DecryptIfExists(string(s)))
}
