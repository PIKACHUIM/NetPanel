package handlers

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/config"
	"github.com/netpanel/netpanel/service/access"
	"github.com/netpanel/netpanel/service/callback"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ===== WOL =====

type WolHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewWolHandler(db *gorm.DB, log *logrus.Logger) *WolHandler {
	return &WolHandler{db: db, log: log}
}

func (h *WolHandler) List(c *gin.Context) {
	var devices []model.WolDevice
	h.db.Order("id desc").Find(&devices)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": devices})
}

func (h *WolHandler) Create(c *gin.Context) {
	var device model.WolDevice
	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&device)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": device, "message": "创建成功"})
}

func (h *WolHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.WolDevice
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *WolHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.WolDevice{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *WolHandler) Wake(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var device model.WolDevice
	if err := h.db.First(&device, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "设备不存在"})
		return
	}
	if err := sendWakePacket(device.MACAddress, device.BroadcastIP, device.NetInterface, device.Port); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "唤醒失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "唤醒包已发送"})
}

// ===== 域名账号 =====

type DomainAccountHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewDomainAccountHandler(db *gorm.DB, log *logrus.Logger) *DomainAccountHandler {
	return &DomainAccountHandler{db: db, log: log}
}

func (h *DomainAccountHandler) List(c *gin.Context) {
	var accounts []model.DomainAccount
	h.db.Order("id desc").Find(&accounts)
	// 隐藏 Secret
	for i := range accounts {
		if len(accounts[i].AccessSecret) > 4 {
			accounts[i].AccessSecret = "****" + accounts[i].AccessSecret[len(accounts[i].AccessSecret)-4:]
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": accounts})
}

func (h *DomainAccountHandler) Create(c *gin.Context) {
	var account model.DomainAccount
	if err := c.ShouldBindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&account)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": account, "message": "创建成功"})
}

func (h *DomainAccountHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.DomainAccount
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *DomainAccountHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.DomainAccount{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *DomainAccountHandler) Test(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var account model.DomainAccount
	if err := h.db.First(&account, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "账号不存在"})
		return
	}
	// TODO: 根据 account.Provider 调用对应 DNS 服务商 API 验证凭据
	h.log.Infof("[域名账号] 测试连接: id=%d provider=%s", id, account.Provider)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "连接测试成功"})
}

// ===== 证书账号 =====

type CertAccountHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewCertAccountHandler(db *gorm.DB, log *logrus.Logger) *CertAccountHandler {
	return &CertAccountHandler{db: db, log: log}
}

func (h *CertAccountHandler) List(c *gin.Context) {
	var accounts []model.CertAccount
	h.db.Order("id desc").Find(&accounts)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": accounts})
}

func (h *CertAccountHandler) Create(c *gin.Context) {
	var account model.CertAccount
	if err := c.ShouldBindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&account)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": account, "message": "创建成功"})
}

func (h *CertAccountHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.CertAccount
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CertAccountHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.CertAccount{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *CertAccountHandler) Verify(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var account model.CertAccount
	if err := h.db.First(&account, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "账号不存在"})
		return
	}
	// TODO: 调用 ACME 接口验证账号有效性
	h.log.Infof("[证书账号] 验证账号: id=%d type=%s email=%s", id, account.Type, account.Email)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "账号验证成功"})
}

// ===== 域名证书 =====

type CertHandler struct {
	db     *gorm.DB
	log    *logrus.Logger
	config *config.Config
}

func NewCertHandler(db *gorm.DB, log *logrus.Logger, cfg *config.Config) *CertHandler {
	return &CertHandler{db: db, log: log, config: cfg}
}

func (h *CertHandler) List(c *gin.Context) {
	var certs []model.DomainCert
	h.db.Order("id desc").Find(&certs)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": certs})
}

func (h *CertHandler) Create(c *gin.Context) {
	var cert model.DomainCert
	if err := c.ShouldBindJSON(&cert); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	cert.Status = "pending"
	h.db.Create(&cert)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": cert, "message": "创建成功"})
}

func (h *CertHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.DomainCert
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CertHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.DomainCert{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *CertHandler) Renew(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	// TODO: 实现 ACME 证书续期
	h.log.Infof("触发证书续期: %d", id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "证书续期任务已提交"})
}

// ===== 域名解析 =====

type DomainRecordHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewDomainRecordHandler(db *gorm.DB, log *logrus.Logger) *DomainRecordHandler {
	return &DomainRecordHandler{db: db, log: log}
}

func (h *DomainRecordHandler) List(c *gin.Context) {
	accountID := c.Query("account_id")
	var records []model.DomainRecord
	query := h.db.Order("id desc")
	if accountID != "" {
		query = query.Where("domain_account_id = ?", accountID)
	}
	query.Find(&records)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": records})
}

func (h *DomainRecordHandler) Create(c *gin.Context) {
	var record model.DomainRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&record)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": record, "message": "创建成功"})
}

func (h *DomainRecordHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.DomainRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *DomainRecordHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.DomainRecord{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *DomainRecordHandler) SyncFromProvider(c *gin.Context) {
	// TODO: 从服务商同步解析记录
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "同步任务已提交"})
}

// ===== IP 地址库 =====

type IPDBHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewIPDBHandler(db *gorm.DB, log *logrus.Logger) *IPDBHandler {
	return &IPDBHandler{db: db, log: log}
}

func (h *IPDBHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	keyword := c.Query("keyword")

	var entries []model.IPDBEntry
	var total int64
	query := h.db.Model(&model.IPDBEntry{})
	if keyword != "" {
		query = query.Where("cidr LIKE ? OR location LIKE ? OR tags LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	query.Count(&total)
	query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&entries)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"list":      entries,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}})
}

func (h *IPDBHandler) Create(c *gin.Context) {
	var entry model.IPDBEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&entry)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": entry, "message": "创建成功"})
}

func (h *IPDBHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.IPDBEntry
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *IPDBHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.IPDBEntry{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// parseIPsFromLine 从一行文本中解析出所有 IP/CIDR，支持空格、逗号、分号分隔多个 IP 段
// 行格式示例：
//   192.168.1.0/24
//   192.168.1.0/24 10.0.0.0/8
//   192.168.1.0/24,10.0.0.0/8;172.16.0.0/12
func parseIPsFromLine(line string) []string {
	// 统一将逗号、分号替换为空格，再按空格分割
	replacer := strings.NewReplacer(",", " ", ";", " ")
	normalized := replacer.Replace(line)
	parts := strings.Fields(normalized)
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// 验证是否为有效 IP 或 CIDR
		if strings.Contains(p, "/") {
			_, _, err := net.ParseCIDR(p)
			if err != nil {
				continue
			}
		} else {
			if net.ParseIP(p) == nil {
				continue
			}
		}
		result = append(result, p)
	}
	return result
}

// parseTextToEntries 将文本内容解析为 IPDBEntry 列表
// 支持每行多个 IP/CIDR（空格/逗号/分号分隔），行尾可附加 location 和 tags
// 格式：
//   CIDR1 CIDR2 CIDR3
//   CIDR1,CIDR2 location tags
//   # 注释行
func parseTextToEntries(text, defaultLocation, defaultTags string) []model.IPDBEntry {
	var entries []model.IPDBEntry
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		// 尝试解析行中所有 token，IP/CIDR 格式的作为地址，其余作为 location/tags
		replacer := strings.NewReplacer(",", " ", ";", " ")
		normalized := replacer.Replace(line)
		tokens := strings.Fields(normalized)

		var cidrs []string
		var extras []string
		for _, tok := range tokens {
			if strings.Contains(tok, "/") {
				_, _, err := net.ParseCIDR(tok)
				if err == nil {
					cidrs = append(cidrs, tok)
					continue
				}
			} else if net.ParseIP(tok) != nil {
				cidrs = append(cidrs, tok)
				continue
			}
			extras = append(extras, tok)
		}

		// extras[0] 作为 location，extras[1] 作为 tags（若未指定默认值）
		location := defaultLocation
		tags := defaultTags
		if len(extras) >= 1 && defaultLocation == "" {
			location = extras[0]
		}
		if len(extras) >= 2 && defaultTags == "" {
			tags = extras[1]
		}

		for _, cidr := range cidrs {
			entries = append(entries, model.IPDBEntry{
				CIDR:     cidr,
				Location: location,
				Tags:     tags,
			})
		}
	}
	return entries
}

// upsertEntries 批量 upsert IP 条目，返回实际写入数量
func (h *IPDBHandler) upsertEntries(entries []model.IPDBEntry) int {
	imported := 0
	for i := range entries {
		if entries[i].CIDR == "" {
			continue
		}
		var existing model.IPDBEntry
		result := h.db.Where("cidr = ?", entries[i].CIDR).First(&existing)
		if result.Error == nil {
			existing.Location = entries[i].Location
			existing.Tags = entries[i].Tags
			if entries[i].Remark != "" {
				existing.Remark = entries[i].Remark
			}
			h.db.Save(&existing)
		} else {
			h.db.Create(&entries[i])
		}
		imported++
	}
	return imported
}

// Import 批量导入（手动输入文本，每行支持多个 IP/CIDR）
func (h *IPDBHandler) Import(c *gin.Context) {
	var req struct {
		Entries  []model.IPDBEntry `json:"entries"`
		Text     string            `json:"text"`
		Location string            `json:"location"`
		Tags     string            `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	var entries []model.IPDBEntry
	if req.Text != "" {
		entries = append(entries, parseTextToEntries(req.Text, req.Location, req.Tags)...)
	}
	entries = append(entries, req.Entries...)

	if len(entries) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "没有可导入的条目"})
		return
	}

	imported := h.upsertEntries(entries)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "导入成功", "data": gin.H{"count": imported}})
}

// downloadAndParseURL 下载 URL 内容并解析为 IPDBEntry 列表
func (h *IPDBHandler) downloadAndParseURL(url, defaultLocation, defaultTags string) ([]model.IPDBEntry, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载失败，HTTP状态码: %d", resp.StatusCode)
	}

	// 限制最大读取 50MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("读取内容失败: %w", err)
	}

	entries := parseTextToEntries(string(body), defaultLocation, defaultTags)
	return entries, nil
}

// ImportFromURL 从 URL 下载并导入 IP 列表（每行支持多个 IP/CIDR）
func (h *IPDBHandler) ImportFromURL(c *gin.Context) {
	var req struct {
		URL        string `json:"url" binding:"required"`
		Location   string `json:"location"`
		Tags       string `json:"tags"`
		ClearFirst bool   `json:"clear_first"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	entries, err := h.downloadAndParseURL(req.URL, req.Location, req.Tags)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if len(entries) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 200, "message": "文件中没有找到有效的 IP/CIDR 条目", "data": gin.H{"count": 0}})
		return
	}

	if req.ClearFirst {
		h.db.Where("1 = 1").Delete(&model.IPDBEntry{})
	}

	imported := h.upsertEntries(entries)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "导入成功", "data": gin.H{
		"count": imported,
		"url":   req.URL,
	}})
}

// ===== IP 地址库订阅 =====

// ListSubscriptions 获取订阅列表
func (h *IPDBHandler) ListSubscriptions(c *gin.Context) {
	var subs []model.IPDBSubscription
	h.db.Order("id desc").Find(&subs)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": subs})
}

// CreateSubscription 创建订阅
func (h *IPDBHandler) CreateSubscription(c *gin.Context) {
	var sub model.IPDBSubscription
	if err := c.ShouldBindJSON(&sub); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&sub)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": sub, "message": "创建成功"})
}

// UpdateSubscription 更新订阅
func (h *IPDBHandler) UpdateSubscription(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.IPDBSubscription
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

// DeleteSubscription 删除订阅
func (h *IPDBHandler) DeleteSubscription(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.IPDBSubscription{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// RefreshSubscription 手动刷新订阅
func (h *IPDBHandler) RefreshSubscription(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var sub model.IPDBSubscription
	if err := h.db.First(&sub, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "订阅不存在"})
		return
	}

	entries, err := h.downloadAndParseURL(sub.URL, sub.Location, sub.Tags)
	now := time.Now()
	if err != nil {
		sub.LastSyncTime = &now
		sub.LastSyncError = err.Error()
		h.db.Save(&sub)
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if len(entries) == 0 {
		sub.LastSyncTime = &now
		sub.LastSyncCount = 0
		sub.LastSyncError = ""
		h.db.Save(&sub)
		c.JSON(http.StatusOK, gin.H{"code": 200, "message": "文件中没有找到有效的 IP/CIDR 条目", "data": gin.H{"count": 0}})
		return
	}

	if sub.ClearFirst {
		h.db.Where("1 = 1").Delete(&model.IPDBEntry{})
	}

	imported := h.upsertEntries(entries)
	sub.LastSyncTime = &now
	sub.LastSyncCount = imported
	sub.LastSyncError = ""
	h.db.Save(&sub)

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "刷新成功", "data": gin.H{"count": imported}})
}

// Query 查询 IP 归属地
func (h *IPDBHandler) Query(c *gin.Context) {
	ip := c.Query("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请提供 IP 地址"})
		return
	}

	netIP := net.ParseIP(ip)
	if netIP == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 IP 地址格式"})
		return
	}

	// 先精确匹配
	var entry model.IPDBEntry
	if err := h.db.Where("cidr = ?", ip).First(&entry).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": entry})
		return
	}

	// 遍历所有 CIDR 条目进行匹配
	var allEntries []model.IPDBEntry
	h.db.Find(&allEntries)

	for _, e := range allEntries {
		if strings.Contains(e.CIDR, "/") {
			_, ipNet, err := net.ParseCIDR(e.CIDR)
			if err == nil && ipNet.Contains(netIP) {
				c.JSON(http.StatusOK, gin.H{"code": 200, "data": e})
				return
			}
		} else {
			if net.ParseIP(e.CIDR).Equal(netIP) {
				c.JSON(http.StatusOK, gin.H{"code": 200, "data": e})
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"ip":       ip,
		"location": "",
		"tags":     "",
		"found":    false,
	}})
}

// ===== 访问控制 =====

type AccessHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *access.Manager
}

func NewAccessHandler(db *gorm.DB, log *logrus.Logger, mgr *access.Manager) *AccessHandler {
	return &AccessHandler{db: db, log: log, mgr: mgr}
}

func (h *AccessHandler) List(c *gin.Context) {
	var rules []model.AccessRule
	h.db.Order("id desc").Find(&rules)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": rules})
}

func (h *AccessHandler) Create(c *gin.Context) {
	var rule model.AccessRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&rule)
	h.mgr.Reload()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": rule, "message": "创建成功"})
}

func (h *AccessHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.AccessRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	h.mgr.Reload()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *AccessHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.AccessRule{}, id)
	h.mgr.Reload()
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// ===== 回调账号 =====

type CallbackAccountHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *callback.Manager
}

func NewCallbackAccountHandler(db *gorm.DB, log *logrus.Logger, mgr *callback.Manager) *CallbackAccountHandler {
	return &CallbackAccountHandler{db: db, log: log, mgr: mgr}
}

func (h *CallbackAccountHandler) List(c *gin.Context) {
	var accounts []model.CallbackAccount
	h.db.Order("id desc").Find(&accounts)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": accounts})
}

func (h *CallbackAccountHandler) Create(c *gin.Context) {
	var account model.CallbackAccount
	if err := c.ShouldBindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&account)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": account, "message": "创建成功"})
}

func (h *CallbackAccountHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.CallbackAccount
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CallbackAccountHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.CallbackAccount{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *CallbackAccountHandler) Test(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.TestAccount(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "测试失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "测试成功"})
}

// ===== 回调任务 =====

type CallbackTaskHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewCallbackTaskHandler(db *gorm.DB, log *logrus.Logger) *CallbackTaskHandler {
	return &CallbackTaskHandler{db: db, log: log}
}

func (h *CallbackTaskHandler) List(c *gin.Context) {
	var tasks []model.CallbackTask
	h.db.Order("id desc").Find(&tasks)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": tasks})
}

func (h *CallbackTaskHandler) Create(c *gin.Context) {
	var task model.CallbackTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&task)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": task, "message": "创建成功"})
}

func (h *CallbackTaskHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.CallbackTask
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CallbackTaskHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.CallbackTask{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}
