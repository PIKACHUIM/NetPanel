package storage

import (
	"testing"

	"github.com/netpanel/netpanel/model"
)

func TestValidateConfigSMBNameInjection(t *testing.T) {
	cfg := model.StorageConfig{Protocol: "smb", RootPath: "/srv/share", Name: "ok-name_1"}
	if err := ValidateConfig(&cfg); err != nil {
		t.Fatalf("合法共享名不应报错: %v", err)
	}

	for _, bad := range []string{"evil\nroot preexec = /bin/sh", "bad]name", "名称"} {
		cfg.Name = bad
		if err := ValidateConfig(&cfg); err == nil {
			t.Errorf("SMB 共享名 %q 应被拒绝", bad)
		}
	}
}

func TestValidateConfigSFTPRequiresAuth(t *testing.T) {
	cfg := model.StorageConfig{Protocol: "sftp", RootPath: "/srv/share"}
	if err := ValidateConfig(&cfg); err == nil {
		t.Error("SFTP 匿名（无用户名）应被拒绝")
	}
	cfg.Username = "shareuser"
	if err := ValidateConfig(&cfg); err == nil {
		t.Error("SFTP 无密码应被拒绝")
	}
	cfg.Password = "s3cret"
	if err := ValidateConfig(&cfg); err != nil {
		t.Fatalf("合法 SFTP 配置不应报错: %v", err)
	}
}

func TestValidateConfigRootPath(t *testing.T) {
	cfg := model.StorageConfig{Protocol: "webdav"}
	cfg.RootPath = "relative/path"
	if err := ValidateConfig(&cfg); err == nil {
		t.Error("相对路径应被拒绝")
	}
	cfg.RootPath = "/srv/../etc"
	if err := ValidateConfig(&cfg); err == nil {
		t.Error("含 .. 的路径应被拒绝")
	}
	cfg.RootPath = "/srv/share"
	if err := ValidateConfig(&cfg); err != nil {
		t.Fatalf("合法路径不应报错: %v", err)
	}
	cfg.ListenAddr = "not-an-ip"
	if err := ValidateConfig(&cfg); err == nil {
		t.Error("非法 ListenAddr 应被拒绝")
	}
	cfg.ListenAddr = "192.168.1.10"
	if err := ValidateConfig(&cfg); err != nil {
		t.Fatalf("合法 ListenAddr 不应报错: %v", err)
	}
}
