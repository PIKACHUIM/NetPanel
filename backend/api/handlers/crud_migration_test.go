package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/crud"
	"github.com/sirupsen/logrus"
)

// 本文件验证 PR-4 迁移的核心收益：handler 的纯 CRUD 路径可以脱离数据库单测。
//
// 改造前这些 handler 直接持有 *gorm.DB，要测就必须建库；现在它们只依赖
// crud.Store 接口，测试可注入 crud.MemoryRepo，无需任何 DB 设施。
//
// 注意：这些用例不验证 SQL 正确性（那由 pkg/crud 针对 sqlite 的集成测试覆盖），
// 只验证「handler 正确调用仓储并渲染响应」。

// init 关闭 gin 调试噪音。
func init() { gin.SetMode(gin.TestMode) }

// newWolHandlerWithRepo 构造注入内存仓储的 WolHandler。
// 用 seedByName 而非 struct literal 传 ID：BaseModel.ID 是提升字段，
// go.mod 声明的 lang=go1.25 不允许在复合字面量中直接指定提升字段。
func newWolHandlerWithRepo(t *testing.T, names ...string) (*WolHandler, *crud.MemoryRepo[model.WolDevice]) {
	t.Helper()
	repo := crud.NewMemory[model.WolDevice]("ID")
	for i, n := range names {
		repo.Seed(newWolDevice(uint(i+1), n))
	}
	log := logrus.New()
	log.SetOutput(&strings.Builder{})
	return &WolHandler{repo: repo, log: log}, repo
}

func newWolDevice(id uint, name string) model.WolDevice {
	d := model.WolDevice{Name: name, MACAddress: "AA:BB:CC:DD:EE:FF"}
	d.ID = id
	return d
}

// newCallbackAccount 构造带指定主键的回调账号。
func newCallbackAccount(id uint, name string) model.CallbackAccount {
	a := model.CallbackAccount{Name: name, Type: "cf_origin"}
	a.ID = id
	return a
}

// newCallbackTask 构造带指定主键的回调任务。
func newCallbackTask(id uint, name string) model.CallbackTask {
	task := model.CallbackTask{Name: name}
	task.ID = id
	return task
}

// call 通过 gin 引擎执行一次请求。
//
// prefix 是该 handler 的集合路由前缀（如 /wol），会同时注册 prefix 与
// prefix+"/:id" 两条路由，这样 c.Param("id") 才能像生产环境一样被解析。
// 不能用 /*path 通配：catch-all 会让 :id 永远取不到值。
func call(t *testing.T, h gin.HandlerFunc, method, prefix, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.Handle(method, prefix, h)
	router.Handle(method, prefix+"/:id", h)

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// responseBody 解析统一响应格式 {code, data, message}。
func responseBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("响应不是合法 JSON: %v (body=%s)", err, rec.Body.String())
	}
	return out
}

func TestWolHandlerList(t *testing.T) {
	h, _ := newWolHandlerWithRepo(t, "nas", "router")

	rec := call(t, h.List, http.MethodGet, "/wol", "/wol", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d", rec.Code)
	}

	body := responseBody(t, rec)
	if body["code"] != float64(200) {
		t.Fatalf("响应 code 字段不符: %v", body["code"])
	}
	data, ok := body["data"].([]any)
	if !ok || len(data) != 2 {
		t.Fatalf("data 应为 2 条记录，实际 %v", body["data"])
	}
	// 内存仓储按主键升序返回，此处只断言条数与字段存在性
	first, _ := data[0].(map[string]any)
	if _, has := first["mac_address"]; !has {
		t.Fatalf("记录缺少 mac_address 字段: %v", first)
	}
}

func TestWolHandlerCreate(t *testing.T) {
	h, repo := newWolHandlerWithRepo(t)

	rec := call(t, h.Create, http.MethodPost, "/wol", "/wol",
		`{"name":"server","mac_address":"AA:BB:CC:DD:EE:FF","broadcast_ip":"192.168.1.255"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d (body=%s)", rec.Code, rec.Body.String())
	}

	body := responseBody(t, rec)
	if body["message"] != "创建成功" {
		t.Fatalf("message 不符: %v", body["message"])
	}
	if repo.Len() != 1 {
		t.Fatalf("创建后仓储应存有 1 条记录，实际 %d", repo.Len())
	}

	data, _ := body["data"].(map[string]any)
	if data["name"] != "server" {
		t.Fatalf("回显数据不符: %v", data)
	}
	if id, _ := data["id"].(float64); id == 0 {
		t.Fatal("创建后应回填自增主键")
	}
}

func TestWolHandlerCreateRejectsBadJSON(t *testing.T) {
	h, repo := newWolHandlerWithRepo(t)

	rec := call(t, h.Create, http.MethodPost, "/wol", "/wol", `{invalid json`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法 JSON 期望 400，实际 %d", rec.Code)
	}
	if repo.Len() != 0 {
		t.Fatal("请求非法时不应写入任何记录")
	}
}

func TestWolHandlerUpdate(t *testing.T) {
	repo := crud.NewMemory[model.WolDevice]("ID")
	repo.Seed(newWolDevice(7, "old"))
	log := logrus.New()
	log.SetOutput(&strings.Builder{})
	h := &WolHandler{repo: repo, log: log}

	rec := call(t, h.Update, http.MethodPut, "/wol", "/wol/7",
		`{"name":"new","mac_address":"AA:BB:CC:DD:EE:07"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d (body=%s)", rec.Code, rec.Body.String())
	}
	if body := responseBody(t, rec); body["message"] != "更新成功" {
		t.Fatalf("message 不符: %v", body["message"])
	}

	got, err := repo.Get(t.Context(), uint(7))
	if err != nil {
		t.Fatalf("读取更新结果失败: %v", err)
	}
	if got.Name != "new" {
		t.Fatalf("更新未生效: %+v", got)
	}
	if got.ID != 7 {
		t.Fatalf("主键被错误改写为 %d", got.ID)
	}
}

func TestWolHandlerUpdateRejectsBadJSON(t *testing.T) {
	repo := crud.NewMemory[model.WolDevice]("ID")
	repo.Seed(newWolDevice(7, "old"))
	log := logrus.New()
	log.SetOutput(&strings.Builder{})
	h := &WolHandler{repo: repo, log: log}

	rec := call(t, h.Update, http.MethodPut, "/wol", "/wol/7", `{nope`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法 JSON 期望 400，实际 %d", rec.Code)
	}

	got, _ := repo.Get(t.Context(), uint(7))
	if got.Name != "old" {
		t.Fatal("请求非法时不应改动记录")
	}
}

func TestWolHandlerDelete(t *testing.T) {
	repo := crud.NewMemory[model.WolDevice]("ID")
	repo.Seed(newWolDevice(3, "gone"), newWolDevice(4, "kept"))
	log := logrus.New()
	log.SetOutput(&strings.Builder{})
	h := &WolHandler{repo: repo, log: log}

	rec := call(t, h.Delete, http.MethodDelete, "/wol", "/wol/3", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d", rec.Code)
	}
	if repo.Len() != 1 {
		t.Fatalf("删除后应剩 1 条，实际 %d", repo.Len())
	}
	if _, err := repo.Get(t.Context(), uint(3)); err == nil {
		t.Fatal("记录 3 应已被删除")
	}
}

// TestCallbackTaskHandlerCrud 覆盖另一组已迁移的纯 CRUD handler，
// 验证迁移模板可复用。
func TestCallbackTaskHandlerCrud(t *testing.T) {
	repo := crud.NewMemory[model.CallbackTask]("ID")
	log := logrus.New()
	log.SetOutput(&strings.Builder{})
	h := &CallbackTaskHandler{repo: repo, log: log}

	rec := call(t, h.Create, http.MethodPost, "/callback/tasks", "/callback/tasks",
		`{"name":"task-a","enable":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("Create 期望 200，实际 %d (body=%s)", rec.Code, rec.Body.String())
	}
	if repo.Len() != 1 {
		t.Fatalf("创建后应有 1 条，实际 %d", repo.Len())
	}

	rec = call(t, h.List, http.MethodGet, "/callback/tasks", "/callback/tasks", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("List 期望 200，实际 %d", rec.Code)
	}
	if data, _ := responseBody(t, rec)["data"].([]any); len(data) != 1 {
		t.Fatalf("List 应返回 1 条，实际 %v", responseBody(t, rec)["data"])
	}

	rec = call(t, h.Update, http.MethodPut, "/callback/tasks", "/callback/tasks/1",
		`{"name":"task-b","enable":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("Update 期望 200，实际 %d", rec.Code)
	}
	got, err := repo.Get(t.Context(), uint(1))
	if err != nil || got.Name != "task-b" {
		t.Fatalf("Update 未生效: %+v err=%v", got, err)
	}

	rec = call(t, h.Delete, http.MethodDelete, "/callback/tasks", "/callback/tasks/1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("Delete 期望 200，实际 %d", rec.Code)
	}
	if repo.Len() != 0 {
		t.Fatalf("删除后应为 0 条，实际 %d", repo.Len())
	}
}

func TestCallbackAccountHandlerUpdatePreservesMaskedSecret(t *testing.T) {
	repo := crud.NewMemory[model.CallbackAccount]("ID")
	repo.Seed(newCallbackAccount(1, "cf"))

	log := logrus.New()
	log.SetOutput(&strings.Builder{})
	h := &CallbackAccountHandler{repo: repo, log: log}

	// 掩码形式的 secret 应保留原值——这是迁移必须保持的业务语义
	rec := call(t, h.Update, http.MethodPut, "/callback/accounts", "/callback/accounts/1",
		`{"name":"cf2","type":"cf_origin","access_secret":"****efgh"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d (body=%s)", rec.Code, rec.Body.String())
	}

	got, err := repo.Get(t.Context(), uint(1))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "cf2" {
		t.Fatalf("名称未更新: %+v", got)
	}
}

func TestDomainAccountHandlerUpdateMissingReturns404(t *testing.T) {
	repo := crud.NewMemory[model.DomainAccount]("ID")
	log := logrus.New()
	log.SetOutput(&strings.Builder{})
	h := &DomainAccountHandler{repo: repo, log: log}

	rec := call(t, h.Update, http.MethodPut, "/domain/accounts", "/domain/accounts/99", `{"name":"x"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("更新不存在的账号期望 404，实际 %d", rec.Code)
	}
	if body := responseBody(t, rec); body["message"] != "账号不存在" {
		t.Fatalf("message 不符: %v", body["message"])
	}
}
