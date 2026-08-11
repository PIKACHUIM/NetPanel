package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/ai"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// AiHandler AI 模块处理器
type AiHandler struct {
	db    *gorm.DB
	log   *logrus.Logger
	aiMgr *ai.Manager
}

// NewAiHandler 创建 AI 处理器
func NewAiHandler(db *gorm.DB, log *logrus.Logger, aiMgr *ai.Manager) *AiHandler {
	return &AiHandler{db: db, log: log, aiMgr: aiMgr}
}

// ===== Provider（API 来源）=====

func (h *AiHandler) ListProviders(c *gin.Context) {
	list, err := h.aiMgr.ListProviders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

func (h *AiHandler) CreateProvider(c *gin.Context) {
	var p model.AiProvider
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.aiMgr.CreateProvider(&p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": p})
}

func (h *AiHandler) UpdateProvider(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var p model.AiProvider
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	p.ID = uint(id)
	if err := h.aiMgr.UpdateProvider(&p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": p})
}

func (h *AiHandler) DeleteProvider(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.aiMgr.DeleteProvider(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

func (h *AiHandler) FetchModels(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	provider, err := h.aiMgr.GetProvider(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "来源不存在"})
		return
	}
	models, err := h.aiMgr.FetchModels(provider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	// 更新到数据库
	modelsJSON, _ := json.Marshal(models)
	h.db.Model(&model.AiProvider{}).Where("id = ?", id).Update("models", string(modelsJSON))
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": models})
}

func (h *AiHandler) TestProvider(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	provider, err := h.aiMgr.GetProvider(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "来源不存在"})
		return
	}
	if err := h.aiMgr.TestConnection(provider); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "连接失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "连接成功"})
}

// ===== Conversation（对话）=====

func (h *AiHandler) ListConversations(c *gin.Context) {
	list, err := h.aiMgr.ListConversations()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

func (h *AiHandler) CreateConversation(c *gin.Context) {
	var conv model.AiConversation
	if err := c.ShouldBindJSON(&conv); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if conv.Title == "" {
		conv.Title = "新对话"
	}
	if err := h.aiMgr.CreateConversation(&conv); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": conv})
}

func (h *AiHandler) UpdateConversation(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var conv model.AiConversation
	if err := c.ShouldBindJSON(&conv); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	conv.ID = uint(id)
	if err := h.aiMgr.UpdateConversation(&conv); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": conv})
}

func (h *AiHandler) DeleteConversation(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.aiMgr.DeleteConversation(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

func (h *AiHandler) ListMessages(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	msgs, err := h.aiMgr.ListMessages(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": msgs})
}

// SendMessage 发送消息（非流式）
func (h *AiHandler) SendMessage(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "content 不能为空"})
		return
	}

	msg, err := h.aiMgr.SendMessage(uint(id), body.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": msg})
}

// StreamMessage SSE 流式发送消息
func (h *AiHandler) StreamMessage(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "content 不能为空"})
		return
	}

	conv, err := h.aiMgr.GetConversation(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "对话不存在"})
		return
	}

	provider, err := h.aiMgr.GetProvider(conv.ProviderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取 API 来源失败"})
		return
	}

	chatMsgs, err := h.aiMgr.BuildChatMessages(uint(id), conv.AssistantID, body.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	// 保存用户消息
	userMsg := &model.AiMessage{
		ConversationID: uint(id),
		Role:           "user",
		Content:        body.Content,
	}
	h.aiMgr.AddMessage(userMsg)

	// 设置 SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	req := &ai.ChatCompletionRequest{
		Model:    conv.ModelName,
		Messages: chatMsgs,
	}

	chunkCh, errCh := h.aiMgr.StreamChatCompletion(provider, req)

	var fullContent strings.Builder
	var tokensPrompt, tokensCompletion int

	flusher, _ := c.Writer.(http.Flusher)

	for {
		select {
		case chunk, ok := <-chunkCh:
			if !ok {
				// 流结束，保存助手消息
				assistantMsg := &model.AiMessage{
					ConversationID:   uint(id),
					Role:             "assistant",
					Content:          fullContent.String(),
					TokensPrompt:     tokensPrompt,
					TokensCompletion: tokensCompletion,
				}
				h.aiMgr.AddMessage(assistantMsg)

				// 发送完成事件
				fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
				if flusher != nil {
					flusher.Flush()
				}
				return
			}
			if chunk != nil {
				if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
					fullContent.WriteString(chunk.Choices[0].Delta.Content)
				}
				if chunk.Usage != nil {
					tokensPrompt = chunk.Usage.PromptTokens
					tokensCompletion = chunk.Usage.CompletionTokens
				}
				data, _ := json.Marshal(chunk)
				fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
				if flusher != nil {
					flusher.Flush()
				}
			}
		case err, ok := <-errCh:
			if ok && err != nil {
				errData, _ := json.Marshal(gin.H{"error": err.Error()})
				fmt.Fprintf(c.Writer, "data: %s\n\n", string(errData))
				if flusher != nil {
					flusher.Flush()
				}
				return
			}
		}
	}
}

// ExportConversation 导出对话
func (h *AiHandler) ExportConversation(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	data, err := h.aiMgr.ExportConversation(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=conversation_%d.json", id))
	c.Data(http.StatusOK, "application/json", data)
}

// ImportConversation 导入对话
func (h *AiHandler) ImportConversation(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "读取请求体失败"})
		return
	}
	conv, err := h.aiMgr.ImportConversation(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": conv})
}

// ===== Assistant（助理）=====

func (h *AiHandler) ListAssistants(c *gin.Context) {
	list, err := h.aiMgr.ListAssistants()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

func (h *AiHandler) CreateAssistant(c *gin.Context) {
	var a model.AiAssistant
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.aiMgr.CreateAssistant(&a); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": a})
}

func (h *AiHandler) UpdateAssistant(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var a model.AiAssistant
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	a.ID = uint(id)
	if err := h.aiMgr.UpdateAssistant(&a); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": a})
}

func (h *AiHandler) DeleteAssistant(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.aiMgr.DeleteAssistant(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

// ===== CronTask（AI 定时任务）=====

func (h *AiHandler) ListCronTasks(c *gin.Context) {
	list, err := h.aiMgr.ListCronTasks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

func (h *AiHandler) CreateCronTask(c *gin.Context) {
	var t model.AiCronTask
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.aiMgr.CreateCronTask(&t); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": t})
}

func (h *AiHandler) UpdateCronTask(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var t model.AiCronTask
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	t.ID = uint(id)
	if err := h.aiMgr.UpdateCronTask(&t); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": t})
}

func (h *AiHandler) DeleteCronTask(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.aiMgr.DeleteCronTask(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

func (h *AiHandler) EnableCronTask(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.aiMgr.EnableCronTask(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

func (h *AiHandler) DisableCronTask(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.aiMgr.DisableCronTask(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

func (h *AiHandler) RunCronTask(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.aiMgr.RunCronTaskNow(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已触发执行"})
}

func (h *AiHandler) ListCronLogs(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	limit := 20
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	logs, err := h.aiMgr.ListCronLogs(uint(id), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": logs})
}

// ===== Plugin（插件）=====

func (h *AiHandler) ListPlugins(c *gin.Context) {
	pluginType := c.Query("type")
	list, err := h.aiMgr.ListPlugins(pluginType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

func (h *AiHandler) CreatePlugin(c *gin.Context) {
	var p model.AiPlugin
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.aiMgr.CreatePlugin(&p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": p})
}

func (h *AiHandler) UpdatePlugin(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var p model.AiPlugin
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	p.ID = uint(id)
	if err := h.aiMgr.UpdatePlugin(&p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": p})
}

func (h *AiHandler) DeletePlugin(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.aiMgr.DeletePlugin(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

func (h *AiHandler) TogglePlugin(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		IsActive bool `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.aiMgr.TogglePlugin(uint(id), body.IsActive); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}
