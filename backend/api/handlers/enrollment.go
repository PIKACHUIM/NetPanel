package handlers

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/service/enrollment"
)

// EnrollmentHandler enrollment handler
type EnrollmentHandler struct {
	mgr *enrollment.Manager
}

// NewEnrollmentHandler 创建enrollment handler
func NewEnrollmentHandler(mgr *enrollment.Manager) *EnrollmentHandler {
	return &EnrollmentHandler{mgr: mgr}
}

// GenerateTokenRequest enrollment token生成请求
type GenerateTokenRequest struct {
	NodeName    string   `json:"node_name" binding:"required"`
	DNSNames    []string `json:"dns_names"`
	IPAddresses []string `json:"ip_addresses"`
}

// GenerateToken 生成一次性token
// POST /api/v1/admin/enrollment/tokens
func (h *EnrollmentHandler) GenerateToken(c *gin.Context) {
	var req GenerateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	var ipAddresses []net.IP
	for _, ipStr := range req.IPAddresses {
		if ip := net.ParseIP(ipStr); ip != nil {
			ipAddresses = append(ipAddresses, ip)
		}
	}

	token, err := h.mgr.GenerateToken(req.NodeName, req.DNSNames, ipAddresses)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"token": token.ID,
			"expires_at": token.ExpiresAt.Format("2006-01-02T15:04:05Z"),
		},
		"message": "token generated",
	})
}

// ExchangeCertificateRequest certificate交换请求
type ExchangeCertificateRequest struct {
	TokenID     string   `json:"token_id" binding:"required"`
	DNSNames    []string `json:"dns_names"`
	IPAddresses []string `json:"ip_addresses"`
}

// ExchangeCertificate 用token换取证书
// POST /api/v1/enrollment/exchange
func (h *EnrollmentHandler) ExchangeCertificate(c *gin.Context) {
	var req ExchangeCertificateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	var ipAddresses []net.IP
	for _, ipStr := range req.IPAddresses {
		if ip := net.ParseIP(ipStr); ip != nil {
			ipAddresses = append(ipAddresses, ip)
		}
	}

	certPEM, keyPEM, err := h.mgr.ExchangeCertificate(req.TokenID, req.DNSNames, ipAddresses)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"certificate": string(certPEM),
			"private_key": string(keyPEM),
		},
		"message": "certificate issued",
	})
}

// ListTokens 列出所有token
// GET /api/v1/admin/enrollment/tokens
func (h *EnrollmentHandler) ListTokens(c *gin.Context) {
	tokens := h.mgr.ListTokens()
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": tokens,
	})
}

// RevokeToken 吊销token
// DELETE /api/v1/admin/enrollment/tokens/:id
func (h *EnrollmentHandler) RevokeToken(c *gin.Context) {
	tokenID := c.Param("id")
	if err := h.mgr.RevokeToken(tokenID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "token revoked"})
}
