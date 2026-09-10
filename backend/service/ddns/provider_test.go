package ddns

import (
	"encoding/hex"
	"testing"
)

func TestSplitDomain(t *testing.T) {
	cases := []struct {
		in   string
		rr   string
		root string
	}{
		{"home.example.com", "home", "example.com"},
		{"example.com", "@", "example.com"},
		{"example.com.", "@", "example.com"},   // 结尾点号
		{"  a.b.com  ", "a", "b.com"},          // 首尾空白
		{"home:yyy.eu.org", "home", "yyy.eu.org"}, // 手动指定语法
		{"@:yyy.eu.org", "@", "yyy.eu.org"},       // 手动指定 @
		{"a.b.c.example.com", "a.b.c", "example.com"},
		// 无法识别的公共后缀走回退：取最后两段
		{"xxx.yyy.eu.org", "xxx", "yyy.eu.org"},
	}
	for _, c := range cases {
		rr, root := splitDomain(c.in)
		if rr != c.rr || root != c.root {
			t.Errorf("splitDomain(%q) = (%q, %q)，期望 (%q, %q)", c.in, rr, root, c.rr, c.root)
		}
	}
}

func TestSplitDomainManualSyntaxEdge(t *testing.T) {
	// 根域名为空时整体作为 @
	rr, root := splitDomain("home:")
	if rr != "@" || root != "home:" {
		t.Errorf("splitDomain(%q) = (%q, %q)，期望 (@, home:)", "home:", rr, root)
	}
}

func TestBuildQuery(t *testing.T) {
	got := buildQuery(map[string]string{"Action": "DescribeRecords", "Key with space": "a&b"})
	// buildQuery 不排序，只做转义拼接
	if got == "" {
		t.Fatal("buildQuery 不应返回空串")
	}
	contains := func(s string) bool {
		for _, part := range splitQuery(got) {
			if part == s {
				return true
			}
		}
		return false
	}
	if !contains("Action=DescribeRecords") {
		t.Errorf("缺少 Action 参数: %q", got)
	}
	if !contains("Key+with+space=a%26b") {
		t.Errorf("参数转义错误: %q", got)
	}
}

func splitQuery(q string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(q); i++ {
		if q[i] == '&' {
			parts = append(parts, q[start:i])
			start = i + 1
		}
	}
	return append(parts, q[start:])
}

func TestHashSHA256(t *testing.T) {
	// sha256("abc") 的标准测试向量
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := hashSHA256("abc"); got != want {
		t.Errorf("hashSHA256(%q) = %q，期望 %q", "abc", got, want)
	}
}

func TestHmacSHA256(t *testing.T) {
	// RFC 4231 Test Case 2：key="Jefe"，data="what do ya want for nothing?"
	want := "5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843"
	got := hmacSHA256([]byte("Jefe"), "what do ya want for nothing?")
	if hex.EncodeToString(got) != want {
		t.Errorf("hmacSHA256 = %s，期望 %s", hex.EncodeToString(got), want)
	}
}

func TestMd5Hash(t *testing.T) {
	// md5("abc") = 900150983cd24fb0d6963f7d28e17f72
	if got := md5Hash("abc"); got != "900150983cd24fb0d6963f7d28e17f72" {
		t.Errorf("md5Hash(%q) = %q", "abc", got)
	}
}
