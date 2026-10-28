package monitor

import (
	"testing"
)

// TestProbeICMP_InvalidAddr 非法地址应返回错误而不 panic
func TestProbeICMP_InvalidAddr(t *testing.T) {
	c := &Collector{}

	ok, _, err := c.ProbeICMP("999.999.999.999", 2)
	if ok {
		t.Fatalf("expected probe failure for invalid address")
	}
	if err == nil {
		t.Fatal("expected error for invalid address")
	}
}

// TestProbeICMP_EmptyAddr 空地址应返回错误
func TestProbeICMP_EmptyAddr(t *testing.T) {
	c := &Collector{}

	ok, _, err := c.ProbeICMP("", 2)
	if ok {
		t.Fatalf("expected probe failure for empty address")
	}
	if err == nil {
		t.Fatal("expected error for empty address")
	}
}

// TestProbeICMP_LocalHost 回环地址探测在不同运行权限下可成功或返回错误。
func TestProbeICMP_LocalHost(t *testing.T) {
	c := &Collector{}

	ok, _, err := c.ProbeICMP("127.0.0.1", 2)
	if ok == (err != nil) {
		t.Fatalf("ICMP probe result must pair success with nil error: ok=%v err=%v", ok, err)
	}
}
