package firewall

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/netpanel/netpanel/model"
)

// portCharsRe 端口字段允许的字符集：数字、逗号、连字符、冒号、空格。
//
// 端口字段会被拼进 nftables 表达式字符串（buildNftExpr），若不限制字符集，
// 攻击者可提交 "22 accept; flush ruleset; #" 之类内容注入额外的 nft 语句，
// 清空整份规则集。iptables/netsh 分支虽然以独立 argv 传参，但同样是不可信输入。
var portCharsRe = regexp.MustCompile(`^[0-9,:\- ]+$`)

// ifaceForbiddenRe 网卡名中禁止出现的字符（可破坏命令行/表达式结构的元字符）。
// 不做过强约束，因为 Windows 的 InterfaceTypes 可能是 "Local Area Network" 这类
// 含空格的人类可读名称。
var ifaceForbiddenRe = regexp.MustCompile("[`\"'\\\\;|&$<>(){}#*?\r\n\t]")

// validateRule 校验防火墙规则字段。
//
// 该函数是防火墙规则进入命令行的唯一闸门：所有 ApplyRule / RemoveRule 调用
// 都必须先通过校验，避免把用户输入直接拼进 iptables/nft/ufw/firewalld/netsh 参数。
func validateRule(rule *model.FirewallRule) error {
	if rule == nil {
		return fmt.Errorf("规则为空")
	}

	switch rule.Direction {
	case "", "in", "out":
	default:
		return fmt.Errorf("方向只能是 in 或 out，当前为 %q", rule.Direction)
	}

	switch rule.Action {
	case "", "allow", "deny":
	default:
		return fmt.Errorf("动作只能是 allow 或 deny，当前为 %q", rule.Action)
	}

	switch strings.ToLower(strings.TrimSpace(rule.Protocol)) {
	case "", "all", "any", "tcp", "udp", "tcp+udp", "icmp", "icmpv6":
	default:
		return fmt.Errorf("协议不合法: %q", rule.Protocol)
	}

	if rule.IPVersion != 0 && rule.IPVersion != 4 && rule.IPVersion != 6 {
		return fmt.Errorf("IP 版本只能是 4 或 6，当前为 %d", rule.IPVersion)
	}

	if err := validateAddr(rule.SrcIP, "源地址"); err != nil {
		return err
	}
	if err := validateAddr(rule.DstIP, "目标地址"); err != nil {
		return err
	}

	if p := strings.TrimSpace(rule.Port); p != "" {
		if !portCharsRe.MatchString(p) {
			return fmt.Errorf("端口字段只能包含数字、逗号、连字符、冒号与空格，当前为 %q", rule.Port)
		}
	}

	if iface := strings.TrimSpace(rule.Interface); iface != "" {
		if len(iface) > 100 || ifaceForbiddenRe.MatchString(iface) {
			return fmt.Errorf("网卡名不合法: %q", rule.Interface)
		}
	}

	return nil
}

// validateAddr 校验地址字段必须是合法 IP 或 CIDR。
func validateAddr(value, label string) error {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil
	}
	if strings.Contains(v, "/") {
		if _, _, err := net.ParseCIDR(v); err != nil {
			return fmt.Errorf("%s 不是合法的 CIDR: %q", label, value)
		}
		return nil
	}
	if net.ParseIP(v) == nil {
		return fmt.Errorf("%s 不是合法的 IP: %q", label, value)
	}
	return nil
}
