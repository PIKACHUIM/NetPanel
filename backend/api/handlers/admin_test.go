package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
)

// TestHasRoleToken 精确匹配逗号分隔角色串，不因子串误判。
// 回归 #90 的 P1：原实现用 strings.Contains，"notadmin" 会被误判为 admin。
func TestHasRoleToken(t *testing.T) {
	cases := []struct {
		roles, want string
		expect      bool
	}{
		{"admin", "admin", true},
		{"admin,editor", "admin", true},
		{"editor,admin", "admin", true},
		{" editor , admin ", "admin", true},
		{"editor,viewer", "admin", false},
		{"", "admin", false},
		{"admin", "editor", false},
		// 子串陷阱：Contains 会误判为 true
		{"notadmin", "admin", false},
		{"superadmin,editor", "admin", false},
		{"administrator", "admin", false},
	}
	for _, c := range cases {
		if got := hasRoleToken(c.roles, c.want); got != c.expect {
			t.Errorf("hasRoleToken(%q,%q) = %v，期望 %v", c.roles, c.want, got, c.expect)
		}
	}
}

// newTestUserHandler 构造带内存库的 UserHandler，并迁移 User 表。
func newTestUserHandler(t *testing.T) (*UserHandler, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("迁移 User 表失败: %v", err)
	}
	return NewUserHandler(db, logrus.New()), db
}

// doUpdateUser 以指定操作者身份调用 UpdateUser。
func doUpdateUser(t *testing.T, h *UserHandler, operatorID, targetID uint, roles string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/1",
		strings.NewReader(`{"roles":"`+roles+`"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: itoa(targetID)}}
	c.Set("user_id", operatorID)
	c.Set("username", "operator")
	c.Set("is_admin", true)
	h.UpdateUser(c)
	return w
}

func itoa(v uint) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

// TestUpdateUserSelfDemotionBlocked 管理员给自己降权必须被拒绝。
func TestUpdateUserSelfDemotionBlocked(t *testing.T) {
	h, db := newTestUserHandler(t)
	me := model.User{Username: "me", Roles: model.RoleAdmin, Enable: true}
	if err := db.Create(&me).Error; err != nil {
		t.Fatalf("创建用户: %v", err)
	}

	w := doUpdateUser(t, h, me.ID, me.ID, model.RoleViewer)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("自我降权返回 %d，期望 400；body=%s", w.Code, w.Body.String())
	}

	// 确认角色未被改动
	var after model.User
	db.First(&after, me.ID)
	if !after.HasRole(model.RoleAdmin) {
		t.Fatal("自我降权虽然返回 400，但角色已被改成非 admin")
	}
}

// TestUpdateUserSelfPromotionAllowed 原本非管理员，把自己改成 admin 不应被该规则拦截。
// 回归被反转的原逻辑：它对「升级」误报、对「降权」放行。
func TestUpdateUserSelfPromotionAllowed(t *testing.T) {
	h, db := newTestUserHandler(t)
	me := model.User{Username: "me", Roles: model.RoleEditor, Enable: true}
	if err := db.Create(&me).Error; err != nil {
		t.Fatalf("创建用户: %v", err)
	}

	w := doUpdateUser(t, h, me.ID, me.ID, model.RoleAdmin)
	if w.Code != http.StatusOK {
		t.Fatalf("自我升级被拦截，返回 %d，期望 200；body=%s", w.Code, w.Body.String())
	}

	var after model.User
	db.First(&after, me.ID)
	if !after.HasRole(model.RoleAdmin) {
		t.Fatal("自我升级返回 200，但角色未更新为 admin")
	}
}
