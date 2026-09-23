package firewall

import (
	"testing"

	"github.com/netpanel/netpanel/model"
)

func TestValidateRuleAcceptsNormalRule(t *testing.T) {
	rule := &model.FirewallRule{
		IPVersion: 4,
		Direction: "in",
		Action:    "allow",
		Protocol:  "tcp",
		SrcIP:     "192.168.1.0/24",
		DstIP:     "10.0.0.1",
		Port:      "8080-8090",
		Interface: "eth0",
	}
	if err := validateRule(rule); err != nil {
		t.Fatalf("正常规则不应被拒绝: %v", err)
	}
}

// 端口字段会被拼进 nftables 表达式，必须拒绝可注入语句的内容。
func TestValidateRuleRejectsNftInjection(t *testing.T) {
	cases := []struct {
		name string
		rule model.FirewallRule
	}{
		{"端口注入分号", model.FirewallRule{Protocol: "tcp", Port: "22 accept; flush ruleset; #"}},
		{"端口注入大括号", model.FirewallRule{Protocol: "tcp", Port: "22 } drop all {"}},
		{"源地址非法", model.FirewallRule{Protocol: "tcp", SrcIP: "1.1.1.1 accept; flush ruleset"}},
		{"目标地址非法", model.FirewallRule{Protocol: "tcp", DstIP: "not-an-ip"}},
		{"CIDR 非法", model.FirewallRule{Protocol: "tcp", SrcIP: "10.0.0.0/99"}},
		{"协议非法", model.FirewallRule{Protocol: "tcp; drop"}},
		{"动作非法", model.FirewallRule{Protocol: "tcp", Action: "allow; drop"}},
		{"方向非法", model.FirewallRule{Protocol: "tcp", Direction: "input"}},
		{"网卡名注入", model.FirewallRule{Protocol: "tcp", Interface: "eth0\" ; drop "}},
		{"IP 版本非法", model.FirewallRule{Protocol: "tcp", IPVersion: 5}},
	}
	for _, c := range cases {
		rule := c.rule
		if err := validateRule(&rule); err == nil {
			t.Errorf("%s: 期望校验失败，实际通过", c.name)
		}
	}
}

func TestValidateRuleAllowsEmptyOptionalFields(t *testing.T) {
	rule := &model.FirewallRule{Protocol: "all"}
	if err := validateRule(rule); err != nil {
		t.Fatalf("可选字段留空应通过: %v", err)
	}
}
