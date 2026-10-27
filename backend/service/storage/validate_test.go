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
	// ValidateRootPath 会调 filepath.EvalSymlinks 解析软链（用于防止用软链
	// 绕过目录校验）。macOS 的 t.TempDir() 返回 /var/folders/...，
	// 而它是指向 /private/var/folders/... 的符号链接，因此期望值必须
	// 同样先解析软链，否则该断言只在没有软链的系统（如 Linux CI）上成立。
	want := dir
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		want = resolved
	}
	if got != filepath.Clean(want) {
		t.Fatalf("应返回规范化绝对路径，got=%s want=%s", got, filepath.Clean(want))
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

// TestValidateRootPathResolvesSymlink 覆盖该函数防绕过的核心语义：
// 传入指向别处的软链时，必须返回软链解析后的真实路径，
// 否则攻击者可用软链把共享根目录指到 smb.conf 等敏感文件上。
func TestValidateRootPathResolvesSymlink(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "real")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("当前环境不支持创建符号链接: %v", err)
	}

	got, err := ValidateRootPath(link)
	if err != nil {
		t.Fatalf("指向合法目录的软链应通过: %v", err)
	}
	if got == link {
		t.Fatalf("应返回软链解析后的路径而非原软链路径 %s", got)
	}
	want := target
	if resolved, err := filepath.EvalSymlinks(target); err == nil {
		want = resolved
	}
	if got != filepath.Clean(want) {
		t.Fatalf("软链应解析为目标目录，got=%s want=%s", got, filepath.Clean(want))
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
