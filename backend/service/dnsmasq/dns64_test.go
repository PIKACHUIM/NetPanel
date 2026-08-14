package dnsmasq

import (
	"net"
	"testing"

	"github.com/miekg/dns"
)

// TestHasAAAA 应答中是否已有 AAAA 记录
func TestHasAAAA(t *testing.T) {
	none := []dns.RR{}
	if hasAAAA(none) {
		t.Fatal("空应答不应包含 AAAA")
	}

	aOnly := []dns.RR{&dns.A{Hdr: dns.RR_Header{Name: "x.", Rrtype: dns.TypeA}, A: net.ParseIP("1.2.3.4").To4()}}
	if hasAAAA(aOnly) {
		t.Fatal("仅 A 记录不应判定为有 AAAA")
	}

	withAAAA := []dns.RR{&dns.AAAA{Hdr: dns.RR_Header{Name: "x.", Rrtype: dns.TypeAAAA}, AAAA: net.ParseIP("2001:db8::1")}}
	if !hasAAAA(withAAAA) {
		t.Fatal("含 AAAA 记录应判定为有 AAAA")
	}
}

// TestSynthesizeAAAA_默认前缀 默认 64:ff9b::/96 前缀合成
func TestSynthesizeAAAA_默认前缀(t *testing.T) {
	aAnswers := []dns.RR{
		&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA}, A: net.ParseIP("1.2.3.4").To4()},
	}

	out := synthesizeAAAA(aAnswers, "64:ff9b::/96")
	if len(out) != 1 {
		t.Fatalf("应合成 1 条 AAAA, got %d", len(out))
	}
	aaaa, ok := out[0].(*dns.AAAA)
	if !ok {
		t.Fatalf("合成记录应为 AAAA, got %T", out[0])
	}
	want := net.ParseIP("64:ff9b::102:304") // 64:ff9b::0102:0304
	if !aaaa.AAAA.Equal(want) {
		t.Fatalf("合成 AAAA = %s, want %s", aaaa.AAAA, want)
	}
}

// TestSynthesizeAAAA_非法前缀 非法前缀返回 nil
func TestSynthesizeAAAA_非法前缀(t *testing.T) {
	aAnswers := []dns.RR{
		&dns.A{Hdr: dns.RR_Header{Name: "x.", Rrtype: dns.TypeA}, A: net.ParseIP("1.2.3.4").To4()},
	}
	if out := synthesizeAAAA(aAnswers, "not-a-prefix"); out != nil {
		t.Fatalf("非法前缀应返回 nil, got %v", out)
	}
}

// TestSynthesizeAAAA_自定义前缀 自定义 96 位前缀
func TestSynthesizeAAAA_自定义前缀(t *testing.T) {
	aAnswers := []dns.RR{
		&dns.A{Hdr: dns.RR_Header{Name: "x.", Rrtype: dns.TypeA}, A: net.ParseIP("10.0.0.1").To4()},
	}
	out := synthesizeAAAA(aAnswers, "2001:db8:64::/96")
	if len(out) != 1 {
		t.Fatalf("应合成 1 条 AAAA, got %d", len(out))
	}
	aaaa, _ := out[0].(*dns.AAAA)
	want := net.ParseIP("2001:db8:64::a00:1") // 2001:db8:64::0a00:0001
	if !aaaa.AAAA.Equal(want) {
		t.Fatalf("合成 AAAA = %s, want %s", aaaa.AAAA, want)
	}
}

// TestAQueryFromAAAA AAAA 查询转 A 查询
func TestAQueryFromAAAA(t *testing.T) {
	q := &dns.Msg{}
	q.SetQuestion("example.com.", dns.TypeAAAA)
	a := aQueryFromAAAA(q)

	if len(a.Question) != 1 {
		t.Fatalf("A 查询应有 1 个问题, got %d", len(a.Question))
	}
	if a.Question[0].Qtype != dns.TypeA {
		t.Fatalf("A 查询 Qtype = %d, want %d", a.Question[0].Qtype, dns.TypeA)
	}
	if a.Question[0].Name != "example.com." {
		t.Fatalf("A 查询域名 = %s, want example.com.", a.Question[0].Name)
	}
}
