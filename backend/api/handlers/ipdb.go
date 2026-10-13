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
	"github.com/netpanel/netpanel/pkg/logger"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

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
	logger.WriteLog("info", "ipdb", fmt.Sprintf("创建IP地址库条目 [%d] %s", entry.ID, entry.CIDR))
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
	logger.WriteLog("info", "ipdb", fmt.Sprintf("更新IP地址库条目 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *IPDBHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.IPDBEntry{}, id)
	logger.WriteLog("info", "ipdb", fmt.Sprintf("删除IP地址库条目 [%d]", id))
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

// parseTextToCIDRs 将文本内容解析为 IP/CIDR 列表
// 支持每行多个 IP/CIDR（空格/逗号/分号分隔），行尾可附加 location 和 tags（会被忽略）
// 格式：
//   CIDR1 CIDR2 CIDR3
//   CIDR1,CIDR2 location tags
//   # 注释行
func parseTextToCIDRs(text string) []string {
	var cidrs []string
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		replacer := strings.NewReplacer(",", " ", ";", " ")
		normalized := replacer.Replace(line)
		tokens := strings.Fields(normalized)

		for _, tok := range tokens {
			if strings.Contains(tok, "/") {
				_, _, err := net.ParseCIDR(tok)
				if err == nil {
					cidrs = append(cidrs, tok)
					continue
				}
			} else if net.ParseIP(tok) != nil {
				cidrs = append(cidrs, tok)
			}
		}
	}
	return cidrs
}

// createEntry 创建一条 IP 地址库条目，返回是否成功
func (h *IPDBHandler) createEntry(entry *model.IPDBEntry) bool {
	if entry.CIDR == "" {
		return false
	}
	return h.db.Create(entry).Error == nil
}

// Import 批量导入（手动输入文本，同一次导入的所有 IP/CIDR 合并为一条记录）
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

	imported := 0

	// 文本导入：所有 IP/CIDR 合并为一条记录
	if req.Text != "" {
		cidrs := parseTextToCIDRs(req.Text)
		if len(cidrs) > 0 {
			entry := model.IPDBEntry{
				CIDR:     strings.Join(cidrs, ","),
				Location: req.Location,
				Tags:     req.Tags,
			}
			if h.createEntry(&entry) {
				imported++
			}
		}
	}

	// 直接传入的条目逐条创建
	for i := range req.Entries {
		if h.createEntry(&req.Entries[i]) {
			imported++
		}
	}

	if imported == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "没有可导入的条目"})
		return
	}

	logger.WriteLog("info", "ipdb", fmt.Sprintf("批量导入IP地址库 共%d条", imported))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "导入成功", "data": gin.H{"count": imported}})
}

// downloadAndParseCIDRs 下载 URL 内容并解析为 IP/CIDR 列表
func (h *IPDBHandler) downloadAndParseCIDRs(url string) ([]string, error) {
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

	cidrs := parseTextToCIDRs(string(body))
	return cidrs, nil
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

	cidrs, err := h.downloadAndParseCIDRs(req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if len(cidrs) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 200, "message": "文件中没有找到有效的 IP/CIDR 条目", "data": gin.H{"count": 0}})
		return
	}

	if req.ClearFirst {
		h.db.Where("1 = 1").Delete(&model.IPDBEntry{})
	}

	// 同一个 URL 下载的所有 IP/CIDR 合并为一条记录
	entry := model.IPDBEntry{
		CIDR:     strings.Join(cidrs, ","),
		Location: req.Location,
		Tags:     req.Tags,
		Remark:   fmt.Sprintf("从 %s 导入", req.URL),
	}
	imported := 0
	if h.createEntry(&entry) {
		imported = 1
	}

	logger.WriteLog("info", "ipdb", fmt.Sprintf("从URL导入IP地址库 共%d个IP/CIDR url=%s", len(cidrs), req.URL))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "导入成功", "data": gin.H{
		"count":    imported,
		"ip_count": len(cidrs),
		"url":      req.URL,
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
	logger.WriteLog("info", "ipdb", fmt.Sprintf("创建IP地址库订阅 [%d] %s", sub.ID, sub.URL))
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
	logger.WriteLog("info", "ipdb", fmt.Sprintf("更新IP地址库订阅 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

// DeleteSubscription 删除订阅
func (h *IPDBHandler) DeleteSubscription(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.IPDBSubscription{}, id)
	logger.WriteLog("info", "ipdb", fmt.Sprintf("删除IP地址库订阅 [%d]", id))
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

	cidrs, err := h.downloadAndParseCIDRs(sub.URL)
	now := time.Now()
	if err != nil {
		sub.LastSyncTime = &now
		sub.LastSyncError = err.Error()
		h.db.Save(&sub)
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if len(cidrs) == 0 {
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

	// 同一个订阅 URL 的所有 IP/CIDR 合并为一条记录
	entry := model.IPDBEntry{
		CIDR:     strings.Join(cidrs, ","),
		Location: sub.Location,
		Tags:     sub.Tags,
		Remark:   fmt.Sprintf("订阅 [%s] 同步", sub.Name),
	}
	h.createEntry(&entry)

	sub.LastSyncTime = &now
	sub.LastSyncCount = len(cidrs)
	sub.LastSyncError = ""
	h.db.Save(&sub)

	logger.WriteLog("info", "ipdb", fmt.Sprintf("刷新IP地址库订阅 [%d] 共%d个IP/CIDR", id, len(cidrs)))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "刷新成功", "data": gin.H{"count": len(cidrs)}})
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

	// 遍历所有条目，每条记录的 CIDR 字段可能包含逗号分隔的多个 IP/CIDR
	var allEntries []model.IPDBEntry
	h.db.Find(&allEntries)

	for _, e := range allEntries {
		for _, cidr := range strings.Split(e.CIDR, ",") {
			cidr = strings.TrimSpace(cidr)
			if cidr == "" {
				continue
			}
			if strings.Contains(cidr, "/") {
				_, ipNet, err := net.ParseCIDR(cidr)
				if err == nil && ipNet.Contains(netIP) {
					c.JSON(http.StatusOK, gin.H{"code": 200, "data": e})
					return
				}
			} else {
				if parsed := net.ParseIP(cidr); parsed != nil && parsed.Equal(netIP) {
					c.JSON(http.StatusOK, gin.H{"code": 200, "data": e})
					return
				}
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
