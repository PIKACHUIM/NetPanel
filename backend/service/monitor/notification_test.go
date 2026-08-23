package monitor

import (
	"strings"
	"testing"

	"github.com/netpanel/netpanel/model"
)

// TestParseEmailList 收件人列表解析与去空
func TestParseEmailList(t *testing.T) {
	cases := []struct {
		name string
		raw  interface{}
		want int
	}{
		{"正常列表", "a@x.com, b@y.com, c@z.com", 3},
		{"带空项", "a@x.com,, ,b@y.com", 2},
		{"空白字符串", "   ", 0},
		{"空值", "", 0},
		{"非字符串", 123, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseEmailList(tc.raw)
			if len(got) != tc.want {
				t.Fatalf("parseEmailList(%v) 长度 = %d, want %d", tc.raw, len(got), tc.want)
			}
		})
	}
}

// TestBuildEmailMessage 邮件内容包含必要头字段
func TestBuildEmailMessage(t *testing.T) {
	msg := buildEmailMessage("from@x.com", []string{"to@y.com", "to2@z.com"}, "测试标题", "测试正文")

	s := string(msg)
	for _, want := range []string{
		"From: from@x.com",
		"To: to@y.com, to2@z.com",
		"Subject: =?UTF-8?",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"测试正文",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("邮件内容缺少 %q", want)
		}
	}
}

// TestSendEmail_缺配置 sendEmail 缺少关键配置应返回错误
func TestSendEmail_缺配置(t *testing.T) {
	n := &NotificationManager{}

	// 空配置
	if err := n.sendEmail(model.CallbackAccount{Config: "{}"}, "t", "m"); err == nil {
		t.Fatal("空配置应返回错误")
	}

	// 缺发件人
	cfg := `{"smtp_host":"smtp.x.com","smtp_port":587,"smtp_user":"u","smtp_password":"p","to_emails":"a@x.com"}`
	if err := n.sendEmail(model.CallbackAccount{Config: cfg}, "t", "m"); err == nil {
		t.Fatal("缺发件人应返回错误")
	}

	// 缺收件人
	cfg = `{"smtp_host":"smtp.x.com","smtp_port":587,"smtp_from":"f@x.com"}`
	if err := n.sendEmail(model.CallbackAccount{Config: cfg}, "t", "m"); err == nil {
		t.Fatal("缺收件人应返回错误")
	}

	// 非法 JSON
	if err := n.sendEmail(model.CallbackAccount{Config: "{bad"}, "t", "m"); err == nil {
		t.Fatal("非法 JSON 应返回错误")
	}
}

// TestSendEmail_配置合法但连接失败 完整配置应尝试连接并返回失败(而非"未实现"式静默成功)
func TestSendEmail_配置合法但连接失败(t *testing.T) {
	n := &NotificationManager{}
	cfg := `{"smtp_host":"127.0.0.1","smtp_port":1,"smtp_user":"u","smtp_password":"p","smtp_from":"f@x.com","to_emails":"a@x.com"}`

	if err := n.sendEmail(model.CallbackAccount{Config: cfg}, "t", "m"); err == nil {
		t.Fatal("连接失败应返回错误")
	} else if !strings.Contains(err.Error(), "SMTP 发送失败") {
		t.Fatalf("错误应包含 SMTP 发送失败, got: %v", err)
	}
}
