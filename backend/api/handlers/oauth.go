package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/api/middleware"
	"github.com/netpanel/netpanel/model"
	oauthsvc "github.com/netpanel/netpanel/service/oauth"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// OAuthHandler OAuth 认证处理器
type OAuthHandler struct {
	db      *gorm.DB
	log     *logrus.Logger
	service *oauthsvc.Service
}

// NewOAuthHandler 创建 OAuth 处理器
func NewOAuthHandler(db *gorm.DB, log *logrus.Logger) *OAuthHandler {
	return &OAuthHandler{
		db:      db,
		log:     log,
		service: oauthsvc.NewService(db),
	}
}

// ListPublicProviders 获取公开的已启用 Provider 列表（供登录页展示）
func (h *OAuthHandler) ListPublicProviders(c *gin.Context) {
	providers, err := h.service.GetEnabledProviders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取 Provider 列表失败"})
		return
	}

	// 只返回前端需要的字段，隐藏敏感信息
	type PublicProvider struct {
		ID           uint   `json:"id"`
		Name         string `json:"name"`
		Type         string `json:"type"`
		Icon         string `json:"icon"`
		DisplayOrder int    `json:"display_order"`
	}

	result := make([]PublicProvider, 0, len(providers))
	for _, p := range providers {
		result = append(result, PublicProvider{
			ID:           p.ID,
			Name:         p.Name,
			Type:         p.Type,
			Icon:         p.Icon,
			DisplayOrder: p.DisplayOrder,
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

// Authorize 生成授权 URL 并重定向
func (h *OAuthHandler) Authorize(c *gin.Context) {
	providerName := c.Param("provider")
	provider, err := h.service.GetProviderByName(providerName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Provider 不存在或未启用"})
		return
	}

	cfg, err := h.service.GetOAuth2Config(provider)
	if err != nil {
		h.log.Errorf("[OAuth] 获取 OAuth2 配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "OAuth 配置错误"})
		return
	}

	// 生成随机 state 防止 CSRF
	state := generateState()
	// 将 state 存入 cookie 用于验证（30分钟有效）
	c.SetCookie("oauth_state", state, 1800, "/", "", false, true)

	authURL := cfg.AuthCodeURL(state)
	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// Callback 处理 OAuth 回调
func (h *OAuthHandler) Callback(c *gin.Context) {
	providerName := c.Param("provider")
	provider, err := h.service.GetProviderByName(providerName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Provider 不存在或未启用"})
		return
	}

	// 验证 state
	state := c.Query("state")
	cookieState, _ := c.Cookie("oauth_state")
	if state == "" || state != cookieState {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "OAuth state 验证失败"})
		return
	}
	// 清除 state cookie
	c.SetCookie("oauth_state", "", -1, "/", "", false, true)

	// 检查错误
	if errMsg := c.Query("error"); errMsg != "" {
		desc := c.Query("error_description")
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": fmt.Sprintf("OAuth 授权失败: %s - %s", errMsg, desc)})
		return
	}

	// 交换 Token
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "缺少授权码"})
		return
	}

	token, err := h.service.ExchangeToken(provider, code)
	if err != nil {
		h.log.Errorf("[OAuth] Token 交换失败 [%s]: %v", providerName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Token 交换失败"})
		return
	}

	// 获取用户信息
	userInfo, err := h.service.GetUserInfo(provider, token)
	if err != nil {
		h.log.Errorf("[OAuth] 获取用户信息失败 [%s]: %v", providerName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取用户信息失败"})
		return
	}

	// 查找或创建用户
	user, err := h.service.FindOrCreateUser(provider.Name, userInfo)
	if err != nil {
		h.log.Errorf("[OAuth] 用户绑定失败 [%s]: %v", providerName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "用户绑定失败"})
		return
	}

	if !user.Enable {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "用户已被禁用"})
		return
	}

	// 签发 JWT Token
	jwtToken, err := middleware.GenerateToken(user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Token 生成失败"})
		return
	}

	// 设置平台访问控制 Cookie
	middleware.SetSessionCookie(c, user.Username)

	// 重定向到前端 OAuth 回调页面，携带 token
	redirectURL := fmt.Sprintf("/oauth/callback?token=%s&username=%s", jwtToken, user.Username)
	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

// generateState 生成随机 state
func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ===== Provider CRUD 管理 =====

// ListProviders 列出所有 Provider（管理员）
func (h *OAuthHandler) ListProviders(c *gin.Context) {
	var providers []model.OAuthProviderConfig
	if err := h.db.Order("display_order ASC, id ASC").Find(&providers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "查询失败"})
		return
	}
	// 补充 client_secret 脱敏
	type ProviderResp struct {
		model.OAuthProviderConfig
		ClientSecret string `json:"client_secret"`
	}
	result := make([]ProviderResp, 0, len(providers))
	for _, p := range providers {
		resp := ProviderResp{OAuthProviderConfig: p}
		if p.ClientSecret != "" {
			resp.ClientSecret = "******"
		}
		result = append(result, resp)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

// CreateProvider 创建 Provider
func (h *OAuthHandler) CreateProvider(c *gin.Context) {
	var input struct {
		Name         string `json:"name" binding:"required"`
		Type         string `json:"type"`
		ClientID     string `json:"client_id" binding:"required"`
		ClientSecret string `json:"client_secret" binding:"required"`
		AuthURL      string `json:"auth_url"`
		TokenURL     string `json:"token_url"`
		UserInfoURL  string `json:"userinfo_url"`
		IssuerURL    string `json:"issuer_url"`
		Scopes       string `json:"scopes"`
		RedirectURI  string `json:"redirect_uri"`
		Icon         string `json:"icon"`
		DisplayOrder int    `json:"display_order"`
		Enable       bool   `json:"enable"`
		Remark       string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	if input.Type == "" {
		input.Type = "oidc"
	}
	if input.Scopes == "" {
		input.Scopes = "openid profile email"
	}

	provider := model.OAuthProviderConfig{
		Name:         input.Name,
		Type:         input.Type,
		ClientID:     input.ClientID,
		ClientSecret: input.ClientSecret,
		AuthURL:      input.AuthURL,
		TokenURL:     input.TokenURL,
		UserInfoURL:  input.UserInfoURL,
		IssuerURL:    input.IssuerURL,
		Scopes:       input.Scopes,
		RedirectURI:  input.RedirectURI,
		Icon:         input.Icon,
		DisplayOrder: input.DisplayOrder,
		Enable:       input.Enable,
		Remark:       input.Remark,
	}

	if err := h.db.Create(&provider).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "创建失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": provider})
}

// UpdateProvider 更新 Provider
func (h *OAuthHandler) UpdateProvider(c *gin.Context) {
	id := c.Param("id")
	var provider model.OAuthProviderConfig
	if err := h.db.First(&provider, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Provider 不存在"})
		return
	}

	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	// 如果 client_secret 是脱敏值，则不更新
	if secret, ok := input["client_secret"]; ok {
		if secret == "******" || secret == "" {
			delete(input, "client_secret")
		}
	}

	// 删除不允许更新的字段
	delete(input, "id")
	delete(input, "created_at")
	delete(input, "updated_at")

	if err := h.db.Model(&provider).Updates(input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功"})
}

// DeleteProvider 删除 Provider
func (h *OAuthHandler) DeleteProvider(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.Delete(&model.OAuthProviderConfig{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "删除成功"})
}
