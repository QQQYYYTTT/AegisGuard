// backend/examples/security_enhanced/main.go
// 演示如何使用 AegisGuard 的安全增强功能
package main

import (
	"aegisguard/internal/auth"
	"aegisguard/internal/gates"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func main() {
	fmt.Println("=== AegisGuard Security Enhanced Demo ===\n")

	// ========== 1. 初始化签名密钥 ==========
	if err := auth.InitSigningKey(""); err != nil {
		panic(err)
	}
	fmt.Println("[OK] Signing key initialized")

	// ========== 2. 创建带批量判定的 ActionGate ==========
	actionGate := gates.NewActionGateWithBatch(
		10,                   // 窗口大小：10 个事件
		5,                    // 摘要事件数：最多 5 个
		5*time.Second,        // 判定间隔：每 5 秒判定一次
		gates.SimpleLLMJudge, // 使用内置规则引擎
	)
	defer actionGate.Close()
	fmt.Println("[OK] Batch window judge enabled")

	// ========== 3. 签发带预算的令牌 ==========
	token, err := auth.NewToken(
		"shell_exec",      // 工具名称
		"workspace.write", // 权限范围
		"agent-001",       // Agent ID
		"session-123",     // 会话 ID
		"task-456",        // 任务 ID
		5*time.Minute,     // 有效期：5 分钟
		10,                // 预算：最多调用 10 次（防 DoS）
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("[OK] Token created with max_calls=%d\n\n", token.MaxCalls)

	// ========== 4. 模拟工具调用请求 ==========
	fmt.Println("=== Simulating Tool Calls ===")

	// 准备请求头
	tokenJSON, _ := json.Marshal(token)
	headers := http.Header{}
	headers.Set("X-Aegis-Token", string(tokenJSON))

	// 模拟 5 次正常调用
	for i := 1; i <= 5; i++ {
		decision, reason := actionGate.Evaluate("shell_exec", map[string]interface{}{
			"command": fmt.Sprintf("echo 'Call %d'", i),
		}, headers)

		fmt.Printf("[Call %d] Decision: %s - %s (CallCount: %d)\n",
			i, decision, reason, token.CallCount)
		time.Sleep(100 * time.Millisecond)
	}

	// 模拟高风险调用
	fmt.Println("\n=== Simulating High Risk Call ===")
	decision, reason := actionGate.Evaluate("shell_exec", map[string]interface{}{
		"command": "rm -rf /tmp/*",
	}, headers)
	fmt.Printf("[High Risk] Decision: %s - %s\n", decision, reason)

	// ========== 5. 测试预算耗尽 ==========
	fmt.Println("\n=== Testing Budget Exhaustion ===")

	// 创建预算为 3 次的令牌
	limitedToken, _ := auth.NewToken(
		"read_file",
		"read",
		"agent-001",
		"session-123",
		"task-456",
		5*time.Minute,
		3, // 预算：3 次
	)

	limitedTokenJSON, _ := json.Marshal(limitedToken)
	limitedHeaders := http.Header{}
	limitedHeaders.Set("X-Aegis-Token", string(limitedTokenJSON))

	// 调用 4 次（第 4 次应该失败）
	for i := 1; i <= 4; i++ {
		decision, reason := actionGate.Evaluate("read_file", map[string]interface{}{
			"path": "/tmp/test.txt",
		}, limitedHeaders)

		fmt.Printf("[Budget Test %d] Decision: %s - %s (CallCount: %d/%d)\n",
			i, decision, reason, limitedToken.CallCount, limitedToken.MaxCalls)
	}

	// ========== 6. 监控窗口状态 ==========
	fmt.Println("\n=== Window Status ===")
	if actionGate.BatchJudge() != nil {
		status := actionGate.BatchJudge().GetWindowStatus()
		statusJSON, _ := json.MarshalIndent(status, "", "  ")
		fmt.Printf("Window Status: %s\n", string(statusJSON))
	}

	fmt.Println("\n=== Demo Complete ===")
}

// BatchJudge 获取批量判定器（用于测试）
func (ag *ActionGate) BatchJudge() *gates.BatchWindowJudge {
	return ag.batchJudge
}
