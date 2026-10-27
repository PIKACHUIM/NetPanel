package frpmaster

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/netpanel/netpanel/model"
)

// newTestDB 创建内存 SQLite 并迁移所需模型。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&model.FrpMasterNode{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return db
}

// newTestMgr 创建短离线窗口的 Manager（测试快速翻转状态）。
func newTestMgr(t *testing.T) *Manager {
	t.Helper()
	m := NewManager(newTestDB(t), nil)
	m.SetOfflineAfter(40 * time.Millisecond)
	return m
}

func TestCreateReturnsPlaintextTokenOnce(t *testing.T) {
	m := newTestMgr(t)
	node, token, err := m.Create(CreateRequest{
		Name: "home", Region: "cn-east",
		ServerAddr: "1.2.3.4", ServerPort: 7000, FrpsToken: "frps-secret",
	})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if node.ID == 0 || token == "" {
		t.Fatalf("应返回节点 id 与明文 token: id=%d token=%q", node.ID, token)
	}
	if node.NodeTokenHash == "" || node.NodeTokenHash == token {
		t.Fatalf("入库的应为 token 哈希，而非明文")
	}
	// 落库校验
	var got model.FrpMasterNode
	if err := m.db.First(&got, node.ID).Error; err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if got.FrpsToken != "frps-secret" || got.NodeTokenHash != node.NodeTokenHash {
		t.Fatalf("落库字段不一致: %+v", got)
	}
}

func TestCreateValidation(t *testing.T) {
	m := newTestMgr(t)
	cases := []CreateRequest{
		{Name: "", ServerAddr: "x", ServerPort: 7000},   // 缺 name
		{Name: "n", ServerAddr: "", ServerPort: 7000},   // 缺 server_addr
		{Name: "n", ServerAddr: "x", ServerPort: 0},     // 端口 0
		{Name: "n", ServerAddr: "x", ServerPort: 70000}, // 端口越界
	}
	for i, req := range cases {
		if _, _, err := m.Create(req); err == nil {
			t.Errorf("case %d 应报错: %+v", i, req)
		}
	}
}

func TestAuthenticate(t *testing.T) {
	m := newTestMgr(t)
	_, token, err := m.Create(CreateRequest{Name: "n", ServerAddr: "x", ServerPort: 7000})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	var n model.FrpMasterNode
	m.db.First(&n)
	if !m.Authenticate(n.ID, token) {
		t.Fatal("正确 token 应通过认证")
	}
	if m.Authenticate(n.ID, "wrong") {
		t.Fatal("错误 token 不应通过认证")
	}
	if m.Authenticate(9999, token) {
		t.Fatal("不存在的节点不应通过认证")
	}
}

func TestHeartbeatFlipsOnlineOffline(t *testing.T) {
	m := newTestMgr(t)
	if _, _, err := m.Create(CreateRequest{Name: "n", ServerAddr: "x", ServerPort: 7000}); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	// 从未心跳 → offline
	nodes, _ := m.List()
	if len(nodes) != 1 || nodes[0].Status != StatusOffline {
		t.Fatalf("未心跳节点应为 offline，得到 %+v", nodes)
	}
	// 心跳 → online
	var n model.FrpMasterNode
	m.db.First(&n)
	if err := m.Heartbeat(n.ID); err != nil {
		t.Fatalf("心跳失败: %v", err)
	}
	nodes, _ = m.List()
	if nodes[0].Status != StatusOnline {
		t.Fatalf("心跳后应为 online，得到 %+v", nodes[0])
	}
	// 超过离线窗口 → offline
	time.Sleep(80 * time.Millisecond)
	nodes, _ = m.List()
	if nodes[0].Status != StatusOffline {
		t.Fatalf("心跳超时后应为 offline，得到 %+v", nodes[0])
	}
}

func TestSaveStatusStoresRawJSON(t *testing.T) {
	m := newTestMgr(t)
	if _, _, err := m.Create(CreateRequest{Name: "n", ServerAddr: "x", ServerPort: 7000}); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	var n model.FrpMasterNode
	m.db.First(&n)
	payload := `[{"name":"ssh","type":"tcp","status":"running"}]`
	if err := m.SaveStatus(n.ID, payload); err != nil {
		t.Fatalf("保存状态失败: %v", err)
	}
	var got model.FrpMasterNode
	m.db.First(&got, n.ID)
	if got.LastTunnels != payload {
		t.Fatalf("状态未按原样保存: %q", got.LastTunnels)
	}
}

func TestConfigGeneration(t *testing.T) {
	m := newTestMgr(t)
	if _, _, err := m.Create(CreateRequest{
		Name: "n", ServerAddr: "frp.example.com", ServerPort: 7000, FrpsToken: "tk",
	}); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	var n model.FrpMasterNode
	m.db.First(&n)
	out, err := m.Config(n.ID)
	if err != nil {
		t.Fatalf("生成配置失败: %v", err)
	}
	if !strings.Contains(out, `serverAddr = "frp.example.com"`) || !strings.Contains(out, `token = "tk"`) {
		t.Fatalf("配置内容不符:\n%s", out)
	}
	if _, err := m.Config(9999); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("不存在节点应返回 ErrNodeNotFound，得到 %v", err)
	}
}

func TestDelete(t *testing.T) {
	m := newTestMgr(t)
	node, _, err := m.Create(CreateRequest{Name: "n", ServerAddr: "x", ServerPort: 7000})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if err := m.Delete(node.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if nodes, _ := m.List(); len(nodes) != 0 {
		t.Fatalf("删除后应无节点，得到 %+v", nodes)
	}
}
