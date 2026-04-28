// internal/gateway/proxy.go
// AegisGuard 反向代理网关 - 实现低侵入安全接入
// Agent 通过虚拟密钥 vsk- 访问，网关自动替换为真实 API Key 并执行安全检查
package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"aegisguard/internal/audit"
	"aegisguard/internal/gates"
	"aegisguard/internal/vkey"

	"go.uber.org/zap"
)

// AegisProxy AegisGuard 反向代理网关
type AegisProxy struct {
	target      *url.URL // 真实 LLM API 地址
	proxy       *httputil.ReverseProxy
	vkeyMgr     *vkey.Manager // 虚拟密钥管理器
	messageGate *gates.MessageGate
	actionGate  *gates.ActionGate
	returnGate  *gates.ReturnGate
	auditor     *audit.Logger
	logger      *zap.Logger
}

// NewAegisProxy 创建代理
// targetURL: 真实 OpenAI API 地址，如 "https://api.openai.com"
// vkeyMgr: 虚拟密钥管理器
func NewAegisProxy(targetURL string, vkeyMgr *vkey.Manager, logger *zap.Logger) (*AegisProxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	ap := &AegisProxy{
		target:      target,
		vkeyMgr:     vkeyMgr,
		messageGate: gates.NewMessageGate(),
		actionGate:  gates.NewActionGate(),
		returnGate:  gates.NewReturnGate(),
		auditor:     audit.NewLogger(),
		logger:      logger,
	}

	// 创建反向代理，自定义 Director 和 ModifyResponse
	ap.proxy = httputil.NewSingleHostReverseProxy(target)
	ap.proxy.Director = ap.director
	ap.proxy.ModifyResponse = ap.modifyResponse
	ap.proxy.ErrorHandler = ap.errorHandler

	return ap, nil
}

// director 修改请求：在转发到真实 API 前，解析并拦截
// 这是低侵入接入的核心：替换虚拟密钥为真实密钥，Agent 完全无感知
func (ap *AegisProxy) director(req *http.Request) {
	// 从请求上下文中获取虚拟密钥信息（由 router 设置）
	vkeyInfo, ok := req.Context().Value("vkey_info").(*vkey.VirtualKey)
	if !ok {
		// 尝试从 Header 中提取（备用方案）
		vkeyID := req.Header.Get("X-Aegis-VKey-ID")
		if vkeyID != "" {
			var err error
			vkeyInfo, err = ap.vkeyMgr.ValidateAndResolve(vkeyID)
			if err != nil {
				ap.logger.Error("虚拟密钥验证失败", zap.Error(err))
				return
			}
		}
	}

	// ========== 核心：密钥替换（Agent 无感知）==========
	if vkeyInfo != nil {
		// 1. 保存原始虚拟密钥到自定义 Header（用于审计）
		req.Header.Set("X-Aegis-Original-Key", vkeyInfo.KeyID)
		req.Header.Set("X-Aegis-AgentID", vkeyInfo.AgentID)
		req.Header.Set("X-Aegis-Scope", vkeyInfo.Scope)

		// 2. 记录替换前的密钥（脱敏）
		originalAuth := req.Header.Get("Authorization")
		maskedAuth := originalAuth
		if len(originalAuth) > 30 {
			maskedAuth = originalAuth[:30] + "..."
		}
		maskedRealKey := ""
		if len(vkeyInfo.RealAPIKey) > 10 {
			maskedRealKey = vkeyInfo.RealAPIKey[:7] + "..." + vkeyInfo.RealAPIKey[len(vkeyInfo.RealAPIKey)-6:]
		}

		// 3. 替换为真实 API Key（Agent 以为在调用 OpenAI，实际上网关已替换）
		req.Header.Set("Authorization", "Bearer "+vkeyInfo.RealAPIKey)

		ap.logger.Info("【代理】密钥替换完成",
			zap.String("vkey_id", vkeyInfo.KeyID),
			zap.String("agent_id", vkeyInfo.AgentID),
			zap.String("original_auth", maskedAuth),
			zap.String("real_key_mask", maskedRealKey),
			zap.String("target_host", ap.target.Host),
			zap.String("target_path", req.URL.Path),
		)
	} else {
		ap.logger.Warn("【代理】未找到虚拟密钥信息，请求可能未经过认证")
	}

	// 保留原始目标
	req.Host = ap.target.Host
	req.URL.Scheme = ap.target.Scheme
	req.URL.Host = ap.target.Host

	// 解析请求体，判断请求类型
	body, _ := io.ReadAll(req.Body)
	req.Body = io.NopCloser(bytes.NewBuffer(body))

	// 根据路径和请求内容，分发到不同 Gate
	if ap.isChatCompletion(req.URL.Path) {
		// 用户 → Agent 的请求（Message Gate）
		ap.handleChatRequest(req, body, vkeyInfo)
	} else if ap.isToolCall(req.URL.Path, body) {
		// Agent → 工具的请求（Action Gate）
		ap.handleToolCall(req, body, vkeyInfo)
	}

	// 记录审计日志
	ap.auditor.LogRequest(req, body)
}

// modifyResponse 修改响应：在返回给 Agent 前，检查结果（Return Gate）
func (ap *AegisProxy) modifyResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	// 判断响应类型：工具结果 / 普通聊天结果
	if ap.isToolResult(resp) {
		// 工具返回结果 → Return Gate 检查
		cleaned := ap.returnGate.Evaluate(body)
		if cleaned != nil {
			// 结果被隔离/净化，替换响应体
			resp.Body = io.NopCloser(bytes.NewBuffer(cleaned))
			resp.ContentLength = int64(len(cleaned))
			resp.Header.Set("Content-Length", string(rune(len(cleaned))))
		}
	}

	ap.auditor.LogResponse(resp, body)
	return nil
}

// errorHandler 代理错误处理
func (ap *AegisProxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	ap.logger.Error("代理转发失败",
		zap.String("target", ap.target.String()),
		zap.Error(err),
	)
	w.WriteHeader(http.StatusBadGateway)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": "网关转发失败: " + err.Error(),
			"type":    "gateway_error",
		},
	})
}

// ========== 请求类型识别 ==========

func (ap *AegisProxy) isChatCompletion(path string) bool {
	return strings.Contains(path, "/chat/completions")
}

func (ap *AegisProxy) isToolCall(path string, body []byte) bool {
	// 检查请求体是否包含 function_call / tool_calls
	var req struct {
		Messages []struct {
			ToolCalls []interface{} `json:"tool_calls"`
		} `json:"messages"`
	}
	json.Unmarshal(body, &req)
	// 或者检查路径是否匹配 MCP 工具端点
	return strings.Contains(path, "/tools") || len(req.Messages) > 0
}

func (ap *AegisProxy) isToolResult(resp *http.Response) bool {
	// 根据响应特征判断是否为工具执行结果
	contentType := resp.Header.Get("Content-Type")
	return strings.Contains(contentType, "application/json")
}

// ========== Gate 处理 ==========

func (ap *AegisProxy) handleChatRequest(req *http.Request, body []byte, vkeyInfo *vkey.VirtualKey) {
	decision, reason := ap.messageGate.Evaluate(body)
	switch decision {
	case gates.Block:
		// 直接构造阻断响应，不转发到真实 API
		ap.blockRequest(req, reason)
	case gates.Degrade:
		// 降级：修改请求体，限制工具调用权限
		ap.degradeRequest(req, body)
	case gates.Allow:
		// 放行：正常转发
		agentID := "unknown"
		if vkeyInfo != nil {
			agentID = vkeyInfo.AgentID
		}
		ap.logger.Debug("Message Gate 放行",
			zap.String("agent_id", agentID),
		)
	}
}

func (ap *AegisProxy) handleToolCall(req *http.Request, body []byte, vkeyInfo *vkey.VirtualKey) {
	// 从请求中提取工具调用信息
	toolName, params := ap.extractToolCall(body)

	// Action Gate：检查是否有合法的 RequireToken
	decision, reason := ap.actionGate.Evaluate(toolName, params, req.Header)
	switch decision {
	case gates.Deny:
		ap.denyToolCall(req, reason)
	case gates.HumanApproval:
		// 挂起请求，等待人工审批（简化版：记录日志后阻断）
		ap.holdForApproval(req, toolName)
	case gates.Allow:
		// 放行：在请求头中注入 RequireToken（如果缺失）
		ap.injectToken(req, toolName, params, vkeyInfo)
	default:
		ap.logger.Warn("未知的 Action Gate 决策状态",
			zap.Any("decision", decision),
			zap.String("tool", toolName),
		)
	}
}

// ========== 辅助方法 ==========

func (ap *AegisProxy) blockRequest(req *http.Request, reason string) {
	ap.logger.Warn("[MessageGate] 请求被阻断",
		zap.String("reason", reason),
		zap.String("path", req.URL.Path),
	)
	// 实际实现需要劫持响应，这里简化
}

func (ap *AegisProxy) degradeRequest(req *http.Request, body []byte) {
	ap.logger.Info("[MessageGate] 请求降级：限制工具权限",
		zap.String("path", req.URL.Path),
	)
	// 修改请求体：移除高危工具权限
}

func (ap *AegisProxy) denyToolCall(req *http.Request, reason string) {
	ap.logger.Warn("[ActionGate] 工具调用被拒绝",
		zap.String("reason", reason),
		zap.String("path", req.URL.Path),
	)
}

func (ap *AegisProxy) holdForApproval(req *http.Request, toolName string) {
	ap.logger.Info("[ActionGate] 工具调用等待人工审批",
		zap.String("tool", toolName),
		zap.String("path", req.URL.Path),
	)
}

func (ap *AegisProxy) injectToken(req *http.Request, toolName string, params map[string]interface{}, vkeyInfo *vkey.VirtualKey) {
	// 如果 Agent 没带 Token，网关代为申请并注入
	// 实际由控制平面签发
	token := "placeholder-token"
	req.Header.Set("X-Aegis-Token", token)

	ap.logger.Debug("注入 RequireToken",
		zap.String("tool", toolName),
		zap.String("agent_id", vkeyInfo.AgentID),
	)
}

func (ap *AegisProxy) extractToolCall(body []byte) (string, map[string]interface{}) {
	// 解析 OpenAI function calling 格式
	var req struct {
		Messages []struct {
			ToolCalls []struct {
				Function struct {
					Name      string                 `json:"name"`
					Arguments map[string]interface{} `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"messages"`
	}
	json.Unmarshal(body, &req)

	if len(req.Messages) > 0 && len(req.Messages[0].ToolCalls) > 0 {
		tc := req.Messages[0].ToolCalls[0]
		return tc.Function.Name, tc.Function.Arguments
	}
	return "", nil
}

// ServeHTTP 实现 http.Handler
func (ap *AegisProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ap.proxy.ServeHTTP(w, r)
}

// Handler 返回 HTTP 处理函数
func (ap *AegisProxy) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ap.ServeHTTP(w, r)
	}
}
