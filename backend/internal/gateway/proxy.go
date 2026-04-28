// backend/internal/gateway/proxy.go
package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"aegisguard/internal/audit"
	"aegisguard/internal/gates"
)

// AegisProxy AegisGuard 反向代理网关
type AegisProxy struct {
	target      *url.URL // 真实 LLM API 地址
	proxy       *httputil.ReverseProxy
	messageGate *gates.MessageGate
	actionGate  *gates.ActionGate
	returnGate  *gates.ReturnGate
	auditor     *audit.Logger
}

// NewAegisProxy 创建代理
// targetURL: 真实 OpenAI API 地址，如 "https://api.openai.com"
func NewAegisProxy(targetURL string) (*AegisProxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	ap := &AegisProxy{
		target:      target,
		messageGate: gates.NewMessageGate(),
		actionGate:  gates.NewActionGate(),
		returnGate:  gates.NewReturnGate(),
		auditor:     audit.NewLogger(),
	}

	// 创建反向代理，自定义 Director 和 ModifyResponse
	ap.proxy = httputil.NewSingleHostReverseProxy(target)
	ap.proxy.Director = ap.director
	ap.proxy.ModifyResponse = ap.modifyResponse

	return ap, nil
}

// director 修改请求：在转发到真实 API 前，解析并拦截
func (ap *AegisProxy) director(req *http.Request) {
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
		ap.handleChatRequest(req, body)
	} else if ap.isToolCall(req.URL.Path, body) {
		// Agent → 工具的请求（Action Gate）
		ap.handleToolCall(req, body)
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
			resp.Header.Set("Content-Length", string(len(cleaned)))
		}
	}

	ap.auditor.LogResponse(resp, body)
	return nil
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

func (ap *AegisProxy) handleChatRequest(req *http.Request, body []byte) {
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
	}
}

func (ap *AegisProxy) handleToolCall(req *http.Request, body []byte) {
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
		ap.injectToken(req, toolName, params)
	}
}

// ========== 辅助方法 ==========

func (ap *AegisProxy) blockRequest(req *http.Request, reason string) {
	// 构造 403 响应
	// 实际实现需要劫持响应，这里简化
	log.Printf("[MessageGate] BLOCK: %s", reason)
}

func (ap *AegisProxy) degradeRequest(req *http.Request, body []byte) {
	// 修改请求体：移除高危工具权限
	log.Println("[MessageGate] DEGRADE: limiting tool scope")
}

func (ap *AegisProxy) denyToolCall(req *http.Request, reason string) {
	log.Printf("[ActionGate] DENY: %s", reason)
}

func (ap *AegisProxy) holdForApproval(req *http.Request, toolName string) {
	log.Printf("[ActionGate] HOLD: %s pending approval", toolName)
}

func (ap *AegisProxy) injectToken(req *http.Request, toolName string, params map[string]interface{}) {
	// 如果 Agent 没带 Token，网关代为申请并注入
	// 实际由控制平面签发
	token := "placeholder-token"
	req.Header.Set("X-Aegis-Token", token)
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
