package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/ai"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func newAITestHandler(t *testing.T) (*AiHandler, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "ai.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get test database connection: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.AiConversation{}, &model.AiMessage{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	logger := logrus.New()
	return NewAiHandler(db, logger, ai.NewManager(db, logger)), db
}

func newAIRequestContext(t *testing.T, method, path string, userID uint, admin bool) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, path, nil)
	ctx.Set("user_id", userID)
	ctx.Set("is_admin", admin)
	return ctx, recorder
}

func TestAIConversationAccessIsScopedToOwner(t *testing.T) {
	handler, db := newAITestHandler(t)
	owned := model.AiConversation{UserID: 1, Title: "mine"}
	other := model.AiConversation{UserID: 2, Title: "other"}
	if err := db.Create(&owned).Error; err != nil {
		t.Fatalf("create owned conversation: %v", err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("create other conversation: %v", err)
	}
	if err := db.Create(&model.AiMessage{ConversationID: other.ID, Role: "user", Content: "private"}).Error; err != nil {
		t.Fatalf("create private message: %v", err)
	}

	ctx, recorder := newAIRequestContext(t, http.MethodGet, "/api/v1/ai/conversations", 1, false)
	handler.ListConversations(ctx)
	var response struct {
		Data []model.AiConversation `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode conversation list: %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].ID != owned.ID {
		t.Fatalf("user 1 conversation list = %#v, want only conversation %d", response.Data, owned.ID)
	}

	ctx, recorder = newAIRequestContext(t, http.MethodGet, "/api/v1/ai/conversations/"+strconv.Itoa(int(other.ID))+"/messages", 1, false)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(other.ID))}}
	handler.ListMessages(ctx)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("cross-owner messages status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	var count int64
	if err := db.Model(&model.AiMessage{}).Where("conversation_id = ?", other.ID).Count(&count).Error; err != nil {
		t.Fatalf("count private messages: %v", err)
	}
	if count != 1 {
		t.Fatalf("cross-owner read changed message count to %d", count)
	}
}

func TestAIConversationOwnerCannotBeChangedByRequest(t *testing.T) {
	handler, db := newAITestHandler(t)
	conv := model.AiConversation{UserID: 1, Title: "original"}
	if err := db.Create(&conv).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	ctx, recorder := newAIRequestContext(t, http.MethodPut, "/api/v1/ai/conversations/"+strconv.Itoa(int(conv.ID)), 1, false)
	ctx.Request = httptest.NewRequest(http.MethodPut, ctx.Request.URL.String(), strings.NewReader(`{"title":"updated","user_id":2}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(conv.ID))}}
	handler.UpdateConversation(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var got model.AiConversation
	if err := db.First(&got, conv.ID).Error; err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if got.UserID != 1 || got.Title != "updated" {
		t.Fatalf("conversation after update = owner %d, title %q; want owner 1 and updated title", got.UserID, got.Title)
	}
}
