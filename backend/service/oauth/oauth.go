package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/netpanel/netpanel/model"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

// UserInfo OAuth2 用户信息
type UserInfo struct {
	Sub               string `json:"sub"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	// EmailVerified 由 IdP 声明邮箱是否已验证。
	// 保留该字段用于后续策略收紧（例如要求邮箱已验证才允许自动开通账号）。
	EmailVerified bool   `json:"email_verified"`
	Picture       string `json:"picture"`
}

// oauthUsernameInvalidRe 本地用户名中不允许出现的字符
var oauthUsernameInvalidRe = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// Service OAuth2/OIDC 服务
type Service struct {
	db *gorm.DB
}

// NewService 创建 OAuth 服务实例
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// GetEnabledProviders 获取所有已启用的 Provider
func (s *Service) GetEnabledProviders() ([]model.OAuthProviderConfig, error) {
	var providers []model.OAuthProviderConfig
	err := s.db.Where("enable = ?", true).Order("display_order ASC").Find(&providers).Error
	return providers, err
}

// GetProvider 获取单个 Provider
func (s *Service) GetProvider(id uint) (*model.OAuthProviderConfig, error) {
	var provider model.OAuthProviderConfig
	err := s.db.First(&provider, id).Error
	return &provider, err
}

// GetProviderByName 根据名称获取 Provider
func (s *Service) GetProviderByName(name string) (*model.OAuthProviderConfig, error) {
	var provider model.OAuthProviderConfig
	err := s.db.Where("name = ? AND enable = ?", name, true).First(&provider).Error
	return &provider, err
}

// GetOAuth2Config 构建 OAuth2 配置
func (s *Service) GetOAuth2Config(provider *model.OAuthProviderConfig) (*oauth2.Config, error) {
	scopes := strings.Split(provider.Scopes, " ")

	if provider.Type == "oidc" && provider.IssuerURL != "" {
		// 使用 OIDC Discovery 自动获取 endpoints
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		oidcProvider, err := oidc.NewProvider(ctx, provider.IssuerURL)
		if err != nil {
			return nil, fmt.Errorf("OIDC Discovery 失败: %w", err)
		}

		return &oauth2.Config{
			ClientID:     provider.ClientID,
			ClientSecret: provider.ClientSecret,
			RedirectURL:  provider.RedirectURI,
			Scopes:       scopes,
			Endpoint:     oidcProvider.Endpoint(),
		}, nil
	}

	// 使用自定义 endpoints
	return &oauth2.Config{
		ClientID:     provider.ClientID,
		ClientSecret: provider.ClientSecret,
		RedirectURL:  provider.RedirectURI,
		Scopes:       scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  provider.AuthURL,
			TokenURL: provider.TokenURL,
		},
	}, nil
}

// ExchangeToken 用授权码交换 Token
func (s *Service) ExchangeToken(provider *model.OAuthProviderConfig, code string) (*oauth2.Token, error) {
	cfg, err := s.GetOAuth2Config(provider)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("token 交换失败: %w", err)
	}
	return token, nil
}

// GetUserInfo 获取用户信息
func (s *Service) GetUserInfo(provider *model.OAuthProviderConfig, token *oauth2.Token) (*UserInfo, error) {
	if provider.Type == "oidc" && provider.IssuerURL != "" {
		return s.getOIDCUserInfo(provider, token)
	}
	return s.getOAuth2UserInfo(provider, token)
}

// getOIDCUserInfo 通过 OIDC ID Token 获取用户信息
func (s *Service) getOIDCUserInfo(provider *model.OAuthProviderConfig, token *oauth2.Token) (*UserInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	oidcProvider, err := oidc.NewProvider(ctx, provider.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("OIDC Provider 初始化失败: %w", err)
	}

	verifier := oidcProvider.Verifier(&oidc.Config{ClientID: provider.ClientID})

	// 尝试从 ID Token 获取信息
	rawIDToken, ok := token.Extra("id_token").(string)
	if ok && rawIDToken != "" {
		idToken, err := verifier.Verify(ctx, rawIDToken)
		if err == nil {
			var info UserInfo
			if err := idToken.Claims(&info); err == nil && info.Sub != "" {
				return &info, nil
			}
		}
	}

	// Fallback: 使用 UserInfo endpoint
	userInfoResp, err := oidcProvider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		return nil, fmt.Errorf("获取 UserInfo 失败: %w", err)
	}

	var info UserInfo
	if err := userInfoResp.Claims(&info); err != nil {
		return nil, fmt.Errorf("解析 UserInfo 失败: %w", err)
	}
	return &info, nil
}

// getOAuth2UserInfo 通过 UserInfo URL 获取用户信息
func (s *Service) getOAuth2UserInfo(provider *model.OAuthProviderConfig, token *oauth2.Token) (*UserInfo, error) {
	if provider.UserInfoURL == "" {
		return nil, fmt.Errorf("未配置 UserInfo URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", provider.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 UserInfo 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("UserInfo 请求返回 %d: %s", resp.StatusCode, string(body))
	}

	var info UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("解析 UserInfo 失败: %w", err)
	}
	return &info, nil
}

// FindOrCreateUser 查找或创建 OAuth 用户
func (s *Service) FindOrCreateUser(providerName string, userInfo *UserInfo) (*model.User, error) {
	var user model.User

	// 先尝试通过 oauth_provider + oauth_sub 查找
	err := s.db.Where("oauth_provider = ? AND oauth_sub = ?", providerName, userInfo.Sub).First(&user).Error
	if err == nil {
		return &user, nil
	}

	// 不再按用户名自动绑定本地已有账号。
	//
	// 原实现用 IdP 返回的 preferred_username 去匹配本地同名用户并直接绑定，
	// 而 preferred_username 完全由 IdP（可自建）控制：只要 IdP 声称用户名是
	// "admin"，即可把本地面板管理员账号与外部身份绑定，随后以该身份登录，
	// 构成账号接管。现改为：身份只能通过 (provider, sub) 精确匹配复用，
	// 否则为其单独创建一个非管理员账号，绝不触碰任何已有账号。
	if strings.TrimSpace(userInfo.Sub) == "" {
		return nil, fmt.Errorf("OAuth 用户标识（sub）为空，无法建立身份绑定")
	}

	user = model.User{
		Username:      s.uniqueUsername(userInfo),
		Email:         userInfo.Email,
		Enable:        true,
		IsAdmin:       false,
		OAuthProvider: providerName,
		OAuthSub:      userInfo.Sub,
		Remark:        fmt.Sprintf("OAuth2 用户（%s）", providerName),
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, fmt.Errorf("创建 OAuth 用户失败: %w", err)
	}
	return &user, nil
}

// uniqueUsername 依据 IdP 信息生成一个不与现有账号冲突的本地用户名。
// 显式避开内置 admin，避免外部身份占用保留用户名。
func (s *Service) uniqueUsername(userInfo *UserInfo) string {
	base := sanitizeUsername(userInfo.PreferredUsername)
	if base == "" {
		base = sanitizeUsername(userInfo.Name)
	}
	if base == "" {
		base = sanitizeUsername(userInfo.Email)
	}
	if base == "" {
		base = sanitizeUsername(userInfo.Sub)
	}
	if base == "" {
		base = "oauth-user"
	}
	if len(base) > 40 {
		base = base[:40]
	}
	if strings.EqualFold(base, "admin") {
		base = "admin-oauth"
	}

	candidate := base
	for i := 1; i <= 50; i++ {
		var cnt int64
		s.db.Model(&model.User{}).Where("username = ?", candidate).Count(&cnt)
		if cnt == 0 {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
	return fmt.Sprintf("%s-%d", base, time.Now().UnixNano())
}

// sanitizeUsername 清洗用户名：邮箱取 @ 前缀，剔除不安全字符。
func sanitizeUsername(raw string) string {
	v := strings.TrimSpace(raw)
	if i := strings.Index(v, "@"); i > 0 {
		v = v[:i]
	}
	v = oauthUsernameInvalidRe.ReplaceAllString(v, "-")
	return strings.Trim(v, "-._")
}
