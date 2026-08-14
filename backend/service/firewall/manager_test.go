package firewall

import (
	"reflect"
	"testing"

	"github.com/netpanel/netpanel/model"
)

// TestIptablesCmd IP 版本对应命令选择
func TestIptablesCmd(t *testing.T) {
	m := &Manager{}

	cases := []struct {
		name string
		rule *model.FirewallRule
		want string
	}{
		{"默认 IPv4", &model.FirewallRule{}, "iptables"},
		{"显式 IPv4", &model.FirewallRule{IPVersion: 4}, "iptables"},
		{"IPv6", &model.FirewallRule{IPVersion: 6}, "ip6tables"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.iptablesCmd(tc.rule); got != tc.want {
				t.Fatalf("iptablesCmd() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBuildIptablesArgs_IPv4 IPv4 规则参数构造
func TestBuildIptablesArgs_IPv4(t *testing.T) {
	m := &Manager{}
	rule := &model.FirewallRule{
		IPVersion: 4,
		Direction: "in",
		Action:    "allow",
		Protocol:  "tcp",
		SrcIP:     "192.168.1.0/24",
		DstIP:     "10.0.0.1",
		Port:      "8080",
	}

	got := m.buildIptablesArgs(rule, "-A")
	want := []string{"-A", "INPUT", "-p", "tcp", "-s", "192.168.1.0/24", "-d", "10.0.0.1", "--dport", "8080", "-j", "ACCEPT"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildIptablesArgs(IPv4) = %v, want %v", got, want)
	}
}

// TestBuildIptablesArgs_IPv6 IPv6 规则参数构造(含 icmp→icmpv6 适配)
func TestBuildIptablesArgs_IPv6(t *testing.T) {
	m := &Manager{}
	rule := &model.FirewallRule{
		IPVersion: 6,
		Direction: "in",
		Action:    "allow",
		Protocol:  "icmp",
		SrcIP:     "2001:db8::/32",
		DstIP:     "2001:db8::1",
	}

	got := m.buildIptablesArgs(rule, "-A")
	want := []string{"-A", "INPUT", "-p", "icmpv6", "-s", "2001:db8::/32", "-d", "2001:db8::1", "-j", "ACCEPT"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildIptablesArgs(IPv6) = %v, want %v", got, want)
	}
}

// TestBuildIptablesArgs_IPv6TCP IPv6 TCP 端口规则
func TestBuildIptablesArgs_IPv6TCP(t *testing.T) {
	m := &Manager{}
	rule := &model.FirewallRule{
		IPVersion: 6,
		Direction: "in",
		Action:    "deny",
		Protocol:  "tcp",
		Port:      "443",
	}

	got := m.buildIptablesArgs(rule, "-D")
	want := []string{"-D", "INPUT", "-p", "tcp", "--dport", "443", "-j", "DROP"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildIptablesArgs(IPv6 TCP) = %v, want %v", got, want)
	}
}

// TestBuildIptablesArgs_IPv6端口范围 IPv6 端口范围参数
func TestBuildIptablesArgs_IPv6端口范围(t *testing.T) {
	m := &Manager{}
	rule := &model.FirewallRule{
		IPVersion: 6,
		Direction: "in",
		Action:    "allow",
		Protocol:  "tcp+udp",
		Port:      "1000-2000",
	}

	got := m.buildIptablesArgs(rule, "-A")
	want := []string{"-A", "INPUT", "-p", "tcp", "--dport", "1000:2000", "-j", "ACCEPT"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildIptablesArgs(IPv6 端口范围) = %v, want %v", got, want)
	}
}
