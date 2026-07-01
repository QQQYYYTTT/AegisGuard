// backend/internal/gates/provenance.go
package gates

import (
	"encoding/json"
	"strings"
)

// ParamSourcePolicy 声明某个工具的某个高危参数的合法取值来源。
//
// Phase 3（借鉴 AuthGraph · ParamPolicy 参数溯源思想）：仅覆盖预先声明的安全关键参数
// （如转账收款人、删除对象 ID），不对通用业务文本做语义提取，详见 PLAN.md Phase 3 实现说明。
type ParamSourcePolicy struct {
	Param       string   // 参数名，仅支持顶层字段（对应 tool_calls.function.arguments 里的 key）
	SourceTools []string // SourceType 为 observation_direct 时，允许作为取值来源的工具名列表
	SourceType  string   // "user_prompt"（必须来自用户原始输入）| "observation_direct"（必须来自指定工具的历史回执）
}

// paramSourceRegistry 仅覆盖高频高危工具的关键参数，避免过度设计。
var paramSourceRegistry = map[string][]ParamSourcePolicy{
	"transfer_funds": {
		{Param: "recipient", SourceType: "user_prompt"},
		{Param: "amount", SourceType: "user_prompt"},
	},
	"delete_record": {
		{Param: "record_id", SourceTools: []string{"search_records", "list_records"}, SourceType: "observation_direct"},
	},
}

// ProvenanceViolation 描述一次参数溯源校验失败。
type ProvenanceViolation struct {
	ToolName string
	Param    string
	Value    string
	Reason   string
}

// provenanceMessage 是从请求体里解析历史对话所需的最小结构，
// 与 gateway.computeTraceID / extractToolCall 使用的解析形状保持一致。
type provenanceMessage struct {
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	ToolCalls []struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	} `json:"tool_calls"`
}

// CheckProvenance 校验当前工具调用声明的高危参数是否有合法来源。
//
// 原计划设想通过 director 在出站请求上打溯源 Header、在 modifyResponse 里回收比对
// （即"打标签-> 转发到下游 -> 从下游响应里回收标签"的跨请求关联）。但审计 Phase 2 已经
// 核实过的传输模型（见 gateway.computeTraceID 的实现说明与 gateway/trace_test.go 的请求体样例）
// 发现该假设不成立：/v1/chat/completions 是无状态协议，Agent 每次工具调用都会把包含历史
// 工具调用及其 "tool" 角色回执的完整对话重新放进**当前这一次**请求体，而且本网关的转发目标
// 是对话补全接口本身，并不是一个会把请求头透传回响应体的独立工具执行后端——director 打的
// Header 根本没有对应的回收点。
//
// 换言之，做参数溯源完全不需要跨请求关联状态：当前请求体里已经自带了本次任务此前全部的
// 工具调用与回执，直接在转发前对同一个请求体做一次静态扫描即可，比原计划的两段式设计更简单、
// 更可靠，也天然不依赖 LLM。
func CheckProvenance(toolName string, params map[string]interface{}, body []byte) []ProvenanceViolation {
	policies, ok := paramSourceRegistry[toolName]
	if !ok || len(policies) == 0 {
		return nil
	}

	var parsed struct {
		Messages []provenanceMessage `json:"messages"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil
	}

	userText, toolOutputs := collectProvenanceSources(parsed.Messages)

	var violations []ProvenanceViolation
	for _, policy := range policies {
		raw, ok := params[policy.Param]
		if !ok {
			continue
		}
		value := strings.TrimSpace(stringifyProvenanceParam(raw))
		if value == "" {
			continue
		}

		switch policy.SourceType {
		case "user_prompt":
			if !strings.Contains(userText, value) {
				violations = append(violations, ProvenanceViolation{
					ToolName: toolName, Param: policy.Param, Value: value,
					Reason: "value not found in any user message of this conversation",
				})
			}
		case "observation_direct":
			if !valueSeenInSourceTools(value, policy.SourceTools, toolOutputs) {
				violations = append(violations, ProvenanceViolation{
					ToolName: toolName, Param: policy.Param, Value: value,
					Reason: "value not found in prior output of declared source tools (" + strings.Join(policy.SourceTools, ",") + ")",
				})
			}
		}
	}
	return violations
}

// collectProvenanceSources 按消息出现顺序，汇总所有 user 消息正文，并将每条 tool 消息回执
// 关联到紧邻的前一条带 tool_calls 的 assistant 消息所命名的工具。
//
// 关联方式沿用本代码库既有约定（见 gateway.extractToolCall）：不依赖 tool_call_id 字段
// （现有请求体样例里也确实不带这个字段），而是按"assistant 的 tool_calls 之后紧跟的下一条
// tool 消息"做位置配对，与全代码库"单条消息只处理一个工具调用"的既有假设保持一致。
func collectProvenanceSources(messages []provenanceMessage) (string, map[string][]string) {
	var userTextParts []string
	toolOutputs := make(map[string][]string)
	pendingTool := ""

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			if len(msg.Content) > 0 {
				userTextParts = append(userTextParts, string(msg.Content))
			}
		case "tool":
			if pendingTool != "" && len(msg.Content) > 0 {
				toolOutputs[pendingTool] = append(toolOutputs[pendingTool], string(msg.Content))
			}
			pendingTool = ""
		}
		if len(msg.ToolCalls) > 0 {
			pendingTool = msg.ToolCalls[0].Function.Name
		}
	}
	return strings.Join(userTextParts, "\n"), toolOutputs
}

func valueSeenInSourceTools(value string, sourceTools []string, toolOutputs map[string][]string) bool {
	for _, tool := range sourceTools {
		for _, output := range toolOutputs[tool] {
			if strings.Contains(output, value) {
				return true
			}
		}
	}
	return false
}

// stringifyProvenanceParam 把任意 JSON 参数值转换为用于子串匹配的裸文本，
// 去掉字符串类型自带的引号，使其能跟对话正文/工具回执里的裸值对上。
func stringifyProvenanceParam(raw interface{}) string {
	switch v := raw.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return strings.Trim(string(b), `"`)
	}
}

// ProvenanceSettings 是 Phase 3 参数溯源校验的开关配置。
type ProvenanceSettings struct {
	Enabled bool
	Mode    string // "log-only" | "enforce"
}

// NormalizeProvenanceMode 将任意大小写/空白的模式字符串归一化为 "enforce" 或 "log-only"，
// 未识别的取值保守回退到 "log-only"（先记录后阻断的分阶段上线原则）。
func NormalizeProvenanceMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "enforce":
		return "enforce"
	default:
		return "log-only"
	}
}
