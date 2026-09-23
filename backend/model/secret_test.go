package model

import (
	"encoding/json"
	"testing"
)

func TestSecretMarshalMasks(t *testing.T) {
	b, err := json.Marshal(struct {
		Pwd Secret `json:"pwd"`
	}{Pwd: "real-secret"})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"pwd":"********"}` {
		t.Errorf("非空 Secret 应输出掩码, got %s", b)
	}

	b, err = json.Marshal(struct {
		Pwd Secret `json:"pwd"`
	}{})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"pwd":""}` {
		t.Errorf("空 Secret 应输出空串（前端区分未配置）, got %s", b)
	}
}

func TestSecretUnmarshalMaskMeansNoChange(t *testing.T) {
	var out struct {
		Pwd Secret `json:"pwd"`
	}
	// 掩码回显提交 -> 置空（"不修改"），由 handler 回填数据库现值
	if err := json.Unmarshal([]byte(`{"pwd":"********"}`), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Pwd.IsEmpty() {
		t.Errorf("掩码提交应转为空串, got %q", out.Pwd)
	}
	// 原文提交 -> 原样接收
	if err := json.Unmarshal([]byte(`{"pwd":"new-secret"}`), &out); err != nil {
		t.Fatal(err)
	}
	if out.Pwd != "new-secret" {
		t.Errorf("原文应原样接收, got %q", out.Pwd)
	}
}

func TestPreserveSecrets(t *testing.T) {
	type nested struct {
		Key Secret `json:"key"`
	}
	type rec struct {
		BaseModel
		Name     string `json:"name"`
		Password Secret `json:"password"`
		Token    Secret `json:"token"`
		Plain    string `json:"plain"`
		N        nested
	}

	old := rec{Name: "old-name", Password: "real-password", Token: "real-token", Plain: "p", N: nested{Key: "nested-key"}}
	cur := rec{Name: "new-name", N: nested{Key: ""}} // Password/Token 为空（掩码回显提交）
	cur.ID = old.ID

	PreserveSecrets(&cur, &old)

	if cur.Password != "real-password" {
		t.Errorf("空 Secret 应回填现值, got %q", cur.Password)
	}
	if cur.Token != "real-token" {
		t.Errorf("空 Secret 应回填现值, got %q", cur.Token)
	}
	if cur.Name != "new-name" {
		t.Errorf("非 Secret 字段不应被修改, got %q", cur.Name)
	}
	if cur.N.Key != "nested-key" {
		t.Errorf("匿名嵌入内的空 Secret 应回填, got %q", cur.N.Key)
	}

	// 显式提交新值时不应被覆盖
	cur2 := rec{Password: "updated-password"}
	PreserveSecrets(&cur2, &old)
	if cur2.Password != "updated-password" {
		t.Errorf("显式新值应保留, got %q", cur2.Password)
	}
}
