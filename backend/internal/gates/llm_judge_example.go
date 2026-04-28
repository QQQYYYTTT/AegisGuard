// backend/internal/gates/llm_judge_example.go
// Package gates 提供 LLM 判定函数的示例实现
package gates

import (
	"fmt"
	"strings"
)

// SimpleLLMJudge 简单的 LLM 判定函数示例
// 在实际应用中，这里应该调用真实的 LLM API
// 该函数模拟 TrinityGuard 的 Judge 系统
func SimpleLLMJudge(summary *WindowSummary) (Decision, []string, string) {
	// 构建分析提示（实际应用中发送给 LLM）
	prompt := buildLLMPrompt(summary)
	
	// 模拟 LLM 响应（实际应用中解析 LLM 返回）
	decision, monitors, reason := analyzeWithLLM(prompt, summary)
	
	fmt.Printf("[LLM Judge] Analyzed window %s: %s\n", summary.WindowID, reason)
	
	return decision, monitors, reason
}

// buildLLMPrompt 构建 LLM 分析提示
func buildLLMPrompt(summary *WindowSummary) string {
	var sb strings.Builder
	
	sb.WriteString("Analyze the following agent activity window for security risks:\n\n")
	sb.WriteString(fmt.Sprintf("Window ID: %s\n", summary.WindowID))
	sb.WriteString(fmt.Sprintf("Time Range: %v to %v\n", summary.StartTime, summary.EndTime))
	sb.WriteString(fmt.Sprintf("Total Events: %d\n", summary.EventCount))
	sb.WriteString(fmt.Sprintf("High Risk Events: %d\n\n", summary.HighRiskCount))
	
	sb.WriteString("Events:\n")
	for i, event := range summary.Events {
		sb.WriteString(fmt.Sprintf("%d. [%s] Agent: %s, Tool: %s, Risk: %d/100\n",
			i+1, event.Timestamp.Format("15:04:05"), event.AgentID, event.ToolName, event.RiskLevel))
		if event.Content != "" {
			sb.WriteString(fmt.Sprintf("   Content: %s\n", event.Content))
		}
	}
	
	sb.WriteString(fmt.Sprintf("\nActive Monitors: %v\n", summary.ActiveMonitors))
	sb.WriteString("\nDetermine if this pattern indicates malicious behavior.")
	
	return sb.String()
}

// analyzeWithLLM 模拟 LLM 分析（实际应用中调用真实 LLM API）
// 这里实现简单的规则引擎作为示例
func analyzeWithLLM(prompt string, summary *WindowSummary) (Decision, []string, string) {
	// 规则 1：高风险事件聚集（可能为协同攻击）
	if summary.HighRiskCount >= 3 {
		return Block, []string{"jailbreak", "prompt_injection"}, 
			fmt.Sprintf("High risk event cluster detected: %d high-risk events in window", summary.HighRiskCount)
	}
	
	// 规则 2：事件频率异常（可能 DoS）
	if summary.EventCount >= 10 {
		return HumanApproval, []string{"rate_limit"}, 
			fmt.Sprintf("Unusual activity frequency: %d events in window", summary.EventCount)
	}
	
	// 规则 3：检测特定工具组合（横向移动尝试）
	if hasSuspiciousToolCombination(summary.Events) {
		return HumanApproval, []string{"lateral_movement"}, 
			"Suspicious tool combination detected, possible lateral movement"
	}
	
	// 规则 4：风险等级递增模式（探测行为）
	if isRiskEscalating(summary.Events) {
		return HumanApproval, []string{"privilege_escalation"}, 
			"Risk escalation pattern detected, possible privilege escalation attempt"
	}
	
	// 默认：允许
	return Allow, []string{}, "No suspicious patterns detected"
}

// hasSuspiciousToolCombination 检测可疑工具组合
func hasSuspiciousToolCombination(events []WindowEvent) bool {
	toolSet := make(map[string]bool)
	for _, event := range events {
		toolSet[event.ToolName] = true
	}
	
	// 检测危险组合：shell 执行 + 文件读取 + 网络请求
	hasShell := toolSet["shell_exec"] || toolSet["exec"]
	hasFileRead := toolSet["read_file"] || toolSet["file_read"]
	hasNetwork := toolSet["http_request"] || toolSet["network"]
	
	return hasShell && hasFileRead && hasNetwork
}

// isRiskEscalating 检测风险等级是否递增
func isRiskEscalating(events []WindowEvent) bool {
	if len(events) < 3 {
		return false
	}
	
	// 检查最近 3 个事件的风险趋势
	increasing := true
	for i := 1; i < len(events) && i < 3; i++ {
		if events[i].RiskLevel <= events[i-1].RiskLevel {
			increasing = false
			break
		}
	}
	
	// 且最后风险等级较高
	return increasing && events[len(events)-1].RiskLevel >= 50
}

// AdvancedLLMJudge 高级 LLM 判定函数（支持自定义配置）
type AdvancedLLMJudge struct {
	apiKey       string
	apiEndpoint  string
	model        string
	threshold    float64 // 判定阈值
}

// NewAdvancedLLMJudge 创建高级 LLM 判定器
func NewAdvancedLLMJudge(apiKey, endpoint, model string, threshold float64) *AdvancedLLMJudge {
	return &AdvancedLLMJudge{
		apiKey:      apiKey,
		apiEndpoint: endpoint,
		model:       model,
		threshold:   threshold,
	}
}

// Judge LLM 判定方法（实际调用 API）
func (j *AdvancedLLMJudge) Judge(summary *WindowSummary) (Decision, []string, string) {
	// TODO: 实现真实的 LLM API 调用
	// 这里仅提供接口定义，实际实现需要：
	// 1. 构建 API 请求
	// 2. 调用 LLM 服务
	// 3. 解析响应
	// 4. 根据阈值做出决策
	
	return SimpleLLMJudge(summary)
}
