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
	"reflect"
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

// PreserveSecrets 把 old 中非空的 Secret 字段回填到 cur 对应的空字段。
//
// 背景：handler 的 Update 普遍直接 db.Save(请求体)，而 Secret 字段经 GET
// 回显为掩码，前端原样提交时 UnmarshalJSON 会把掩码转为空串——若不回填，
// 每次保存都会把数据库里的真实凭据清空。在 Save 之前调用本函数执行
// "空值 = 不修改"契约。
//
// 仅处理直接字段（含匿名嵌入结构体一层）；切片/Map 内的 Secret 子记录
// 需 handler 自行处理。
func PreserveSecrets(cur, old any) {
	cv := reflect.ValueOf(cur)
	ov := reflect.ValueOf(old)
	if cv.Kind() == reflect.Ptr {
		cv = cv.Elem()
	}
	if ov.Kind() == reflect.Ptr {
		ov = ov.Elem()
	}
	if cv.Kind() != reflect.Struct || ov.Kind() != reflect.Struct {
		return
	}
	secretType := reflect.TypeOf(Secret(""))
	ct, ot := cv.Type(), ov.Type()
	for i := 0; i < ct.NumField(); i++ {
		f := ct.Field(i)
		// 嵌入/具名结构体字段：递归处理（跳过 Secret 本身与未导出字段）
		if f.Type.Kind() == reflect.Struct && f.Type != secretType && f.IsExported() {
			PreserveSecrets(cv.Field(i).Addr().Interface(), ov.Field(i).Addr().Interface())
			continue
		}
		if f.Type != secretType || !f.IsExported() {
			continue
		}
		// old 侧找不到同名字段时跳过
		j, ok := fieldIndexByName(ot, f.Name)
		if !ok {
			continue
		}
		if cv.Field(i).String() == "" && ov.Field(j).String() != "" {
			cv.Field(i).Set(ov.Field(j))
		}
	}
}

// fieldIndexByName 按名查找字段索引（含一层匿名嵌入提升字段）。
func fieldIndexByName(t reflect.Type, name string) (int, bool) {
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Name == name {
			return i, true
		}
	}
	return 0, false
}
