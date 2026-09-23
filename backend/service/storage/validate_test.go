package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRootPath(t *testing.T) {
	dir := t.TempDir()

	got, err := ValidateRootPath(dir)
	if err != nil {
		t.Fatalf("合法目录应通过: %v", err)
	}
	if got != filepath.Clean(dir) {
		t.Fatalf("应返回规范化绝对路径，got=%s want=%s", got, dir)
	}

	if _, err := ValidateRootPath(filepath.Join(dir, "not-exist")); err == nil {
		t.Error("不存在的目录应被拒绝")
	}

	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateRootPath(file); err == nil {
		t.Error("文件（非目录）应被拒绝")
	}

	if _, err := ValidateRootPath(""); err == nil {
		t.Error("空路径应被拒绝")
	}
	if _, err := ValidateRootPath(dir + "\npath = /etc"); err == nil {
		t.Error("含换行的路径应被拒绝（smb.conf 注入）")
	}
}

// 网络存储必须配置凭据：留空会退化为匿名访问。
func TestValidateCredentials(t *testing.T) {
	if err := ValidateCredentials("u", "p", "webdav"); err != nil {
		t.Fatalf("合法凭据应通过: %v", err)
	}
	if err := ValidateCredentials("", "p", "webdav"); err == nil {
		t.Error("用户名为空应被拒绝")
	}
	if err := ValidateCredentials("u", "", "sftp"); err == nil {
		t.Error("密码为空应被拒绝")
	}
	if err := ValidateCredentials("   ", "p", "smb"); err == nil {
		t.Error("空白用户名应被拒绝")
	}
}

func TestValidateListenAddrAndPort(t *testing.T) {
	for _, ok := range []string{"", "0.0.0.0", "127.0.0.1", "::1"} {
		if err := ValidateListenAddr(ok); err != nil {
			t.Errorf("合法监听地址 %q 不应被拒绝: %v", ok, err)
		}
	}
	if err := ValidateListenAddr("localhost:8080"); err == nil {
		t.Error("非 IP 监听地址应被拒绝")
	}

	if err := ValidateListenPort(8080); err != nil {
		t.Errorf("合法端口不应被拒绝: %v", err)
	}
	for _, bad := range []int{0, -1, 70000} {
		if err := ValidateListenPort(bad); err == nil {
			t.Errorf("非法端口 %d 应被拒绝", bad)
		}
	}
}
