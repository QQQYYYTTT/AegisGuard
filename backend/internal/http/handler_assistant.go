package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type assistantMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type assistantChatRequest struct {
	Message  string             `json:"message"`
	Messages []assistantMessage `json:"messages,omitempty"`
}

type assistantChatResponse struct {
	Model   string `json:"model"`
	Message string `json:"message"`
}

type openAIChatRequest struct {
	Model       string             `json:"model"`
	Messages    []assistantMessage `json:"messages"`
	Temperature float64            `json:"temperature"`
	MaxTokens   int                `json:"max_tokens"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message assistantMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (r *Router) registerAssistantRoutes() {
	group := r.engine.Group("/api/assistant")
	{
		group.POST("/chat", r.handleAssistantChat)
	}
}

func (r *Router) handleAssistantChat(c *gin.Context) {
	if strings.TrimSpace(r.cfg.AssistantAPIKey) == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "assistant api key is not configured",
		})
		return
	}

	var req assistantChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
		return
	}

	message := strings.TrimSpace(req.Message)
	if message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "message is required"})
		return
	}

	messages := buildAssistantMessages(req.Messages, message)
	reply, err := r.callAssistantAPI(c, messages)
	if err != nil {
		r.logger.Warn("assistant api call failed", zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": assistantChatResponse{
			Model:   r.cfg.AssistantModel,
			Message: reply,
		},
	})
}

func buildAssistantMessages(history []assistantMessage, message string) []assistantMessage {
	messages := []assistantMessage{
		{
			Role:    "system",
			Content: "你是 AegisGuard 安全网关的中文辅助助手。回答要简洁，必须使用 AegisGuard 项目语境：DPI=Direct Prompt Injection/直接提示注入，OPI=Observation Prompt Injection/观察结果提示注入，MIXED=混合注入链，MP=Memory Poisoning/记忆投毒，POT=Permissioned Operation/Tool Induction/权限工具诱导。不要把 DPI 解释成深度包检测，也不要把 OPI 解释成其他领域术语。回答围绕三道闸、RequireToken、记忆沙箱、审计溯源和已接入 Agent。不要编造系统中不存在的数据；需要用户去页面查看时，明确指出页面名称。",
		},
	}

	for _, item := range history {
		role := strings.TrimSpace(item.Role)
		content := strings.TrimSpace(item.Content)
		if content == "" || (role != "user" && role != "assistant") {
			continue
		}
		messages = append(messages, assistantMessage{Role: role, Content: content})
	}

	messages = append(messages, assistantMessage{Role: "user", Content: message})
	return messages
}

func (r *Router) callAssistantAPI(c *gin.Context, messages []assistantMessage) (string, error) {
	payload := openAIChatRequest{
		Model:       r.cfg.AssistantModel,
		Messages:    messages,
		Temperature: 0.2,
		MaxTokens:   512,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	endpoint := strings.TrimRight(r.cfg.AssistantAPIBase, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+r.cfg.AssistantAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var decoded openAIChatResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return "", fmt.Errorf("assistant api returned invalid json: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if decoded.Error != nil && decoded.Error.Message != "" {
			return "", fmt.Errorf("assistant api error: %s", decoded.Error.Message)
		}
		return "", fmt.Errorf("assistant api status %d", resp.StatusCode)
	}

	if len(decoded.Choices) == 0 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("assistant api returned empty response")
	}

	return strings.TrimSpace(decoded.Choices[0].Message.Content), nil
}
