package waf

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
)

// newTestManager 构造带内存数据库的 WAF 管理器
func newTestManager(t *testing.T) *Manager {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&model.WafConfig{}); err != nil {
		t.Fatalf("迁移 WafConfig 失败: %v", err)
	}
	return NewManager(db)
}

// TestTestRule_合法规则 合法规则应校验通过
func TestTestRule_合法规则(t *testing.T) {
	m := newTestManager(t)

	rule := `SecRule REQUEST_URI "@contains /etc/passwd" "id:100001,phase:1,deny,status:403"`
	if err := m.TestRule(rule); err != nil {
		t.Fatalf("合法规则应通过, got err: %v", err)
	}
}

// TestTestRule_非法规则 非法规则应返回错误
func TestTestRule_非法规则(t *testing.T) {
	m := newTestManager(t)

	if err := m.TestRule("SecRule 语法错误！"); err == nil {
		t.Fatal("非法规则应返回错误")
	}
}

// TestTestRule_空规则 空规则应返回错误
func TestTestRule_空规则(t *testing.T) {
	m := newTestManager(t)

	if err := m.TestRule("   "); err == nil {
		t.Fatal("空规则应返回错误")
	}
}

// TestStart_配置不存在 启动不存在的配置应返回错误
func TestStart_配置不存在(t *testing.T) {
	m := newTestManager(t)

	if err := m.Start(999); err == nil {
		t.Fatal("启动不存在的配置应返回错误")
	}
}

// TestStart_正常启动 WAF 配置存在时启动应成功并更新状态
func TestStart_正常启动(t *testing.T) {
	m := newTestManager(t)

	cfg := model.WafConfig{
		Name: "test-waf",
		CustomRules: `SecRule REQUEST_URI "@contains /admin" "id:100002,phase:1,deny,status:403"`,
	}
	if err := m.db.Create(&cfg).Error; err != nil {
		t.Fatalf("创建 WAF 配置失败: %v", err)
	}

	if err := m.Start(cfg.ID); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}

	var updated model.WafConfig
	m.db.First(&updated, cfg.ID)
	if updated.Status != "running" || !updated.Enable {
		t.Fatalf("启动后状态应为 running/enable, got status=%q enable=%v", updated.Status, updated.Enable)
	}

	// 停止
	if err := m.Stop(cfg.ID); err != nil {
		t.Fatalf("Stop 失败: %v", err)
	}
	m.db.First(&updated, cfg.ID)
	if updated.Status != "stopped" || updated.Enable {
		t.Fatalf("停止后状态应为 stopped/disable, got status=%q enable=%v", updated.Status, updated.Enable)
	}
}

// TestBuildDirectives 指令构建包含引擎开启与自定义规则
func TestBuildDirectives(t *testing.T) {
	cfg := model.WafConfig{CustomRules: `SecRule REQUEST_URI "@streq /x" "id:1,deny"`}
	d := buildDirectives(cfg)

	for _, want := range []string{"SecRuleEngine On", "SecRequestBodyAccess On", "SecRule REQUEST_URI"} {
		if !strings.Contains(d, want) {
			t.Fatalf("指令缺少 %q", want)
		}
	}
}
