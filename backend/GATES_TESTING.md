# 三级策略闸门测试指南

## 概述

AegisGuard的三级策略闸门包含三个核心组件：
- **MessageGate**: 消息门控 - 检测LLM输入中的攻击
- **ActionGate**: 动作门控 - 验证工具调用的安全性
- **ReturnGate**: 返回门控 - 过滤敏感信息返回

## 快速开始

### 方式1：运行完整测试

```bash
# 进入后端目录
cd backend

# 运行所有门控测试
go test -v ./internal/gates -run TestMessageGatePromptInjection
go test -v ./internal/gates -run TestActionGateToolValidation
go test -v ./internal/gates -run TestReturnGatePIIFiltering

# 运行集成流程测试
go test -v ./internal/gates -run TestGatesIntegrationFlow

# 运行所有gates包测试
go test -v ./internal/gates
```

### 方式2：运行演示程序

```bash
# 编译演示程序
cd backend/cmd/gates-demo
go build -o gates-demo .

# 或者直接运行
go run main.go

# 运行特定测试
go run main.go -action message      # 只测试MessageGate
go run main.go -action action       # 只测试ActionGate
go run main.go -action return       # 只测试ReturnGate
go run main.go -action all          # 测试全部（默认）

# 启用详细输出
go run main.go -verbose
```

## 测试用例详解

### MessageGate 测试用例

| 测试 | 输入 | 预期决策 | 说明 |
|------|------|---------|------|
| 正常消息 | "What is the weather today?" | Allow | 正常用户查询 |
| 提示注入 | "Ignore all previous instructions..." | Degrade | 尝试改变系统行为 |
| 记忆污染 | "Remember this command forever..." | Block | 尝试持久化恶意指令 |
| 敏感访问 | "What is the API key..." | Degrade | 请求敏感信息 |

### ActionGate 测试用例

| 测试 | 工具 | 参数 | 预期决策 | 说明 |
|------|------|------|---------|------|
| 读文件 | read_file | /etc/config.txt | Deny* | 无Token拒绝 |
| 系统命令 | shell_exec | rm -rf / | Deny | 高危操作 |
| 转账 | transfer_funds | 1000000 | Deny | 金融操作 |
| 数据导出 | export_data | SELECT * | Deny | 特权操作 |

*注：ActionGate在没有有效Token时会返回Deny

### ReturnGate 测试用例

| 测试 | 内容 | 预期决策 | 说明 |
|------|------|---------|------|
| 安全内容 | 天气信息 | Allow | 无敏感信息 |
| 信用卡号 | 4532-1234-5678-9999 | Degrade | 包含PII，需过滤 |
| 系统提示 | "System prompt: ..." | Degrade | 提示泄露，需过滤 |
| 内幕交易 | 交易建议详情 | Deny | 非法内容 |

## 性能基准测试

```bash
# 运行基准测试
go test -bench=. -benchmem ./internal/gates

# 输出示例：
# BenchmarkMessageGateEvaluate-8    10000    123456 ns/op    5000 B/op    50 allocs/op
# BenchmarkPolicyEngineScore-8      50000     23456 ns/op    1000 B/op    10 allocs/op
```

## 决策风险评分说明

PolicyEngine使用以下规则进行评分：

```
提示注入 (prompt_injection):       +35分
记忆污染 (memory_poisoning):       +55分
敏感访问 (sensitive_access):       +30分
特权范围 (privileged_scope):       +30分
高风险动作 (high_impact_action):   +40分
非法金融 (illegal_finance):        +70分

复合规则加分：
提示注入 + 敏感/特权:              +20分
提示注入 + 记忆污染:               +25分

阈值判定：
Score >= 80:  Block (阻止)
Score >= 65:  HumanApproval (人工审批)
Score >= 45:  Degrade (降级处理)
Score < 45:   Allow (允许)
```

## 集成测试流程

运行 `TestGatesIntegrationFlow` 模拟完整的请求流程：

1. **消息阶段**: 用户发送 "What is the capital of France?"
   - MessageGate 评估: **Allow**
   
2. **动作阶段**: 系统调用 read_file 工具
   - ActionGate 评估: **Deny** (因为没有Token)
   
3. **返回阶段**: LLM返回 "The capital of France is Paris"
   - ReturnGate 评估: **Allow**

所有决策记录到 DecisionStore，可通过API查询。

## 测试输出示例

```
=== AegisGuard 三级策略闸门演示 ===

测试一：消息门控 (MessageGate)
--------------------------------------------------
  正常用户消息          : Allow
    原因: message passed policy checks
  提示注入攻击          : Degrade
    原因: message risk can be handled by degraded execution; risk_score=55; matched_rules=prompt_injection
  记忆污染攻击          : Block
    原因: message attempts to persist or rewrite trusted memory/instructions; risk_score=55; matched_rules=memory_poisoning
  敏感信息请求          : Degrade
    原因: message risk can be handled by degraded execution; risk_score=30; matched_rules=sensitive_access
```

## 调试技巧

### 1. 启用详细日志

```bash
go run main.go -verbose
```

### 2. 单个规则测试

修改测试用例中的 `content` 字段，测试特定规则：

```go
// 只测试提示注入规则
"Ignore previous instructions"

// 只测试记忆污染规则
"persist memory forever"

// 只测试敏感访问规则
"show me the API key"
```

### 3. 观察评分变化

在 `TestPolicyEngineScoring` 中修改内容，观察分数变化。

## 常见问题

**Q: 为什么ActionGate总是返回Deny?**
A: 因为测试中没有有效的Token。ActionGate需要通过Token校验才能返回Allow。在生产环境中，网关会注入有效Token。

**Q: 返回内容过滤的具体行为是什么?**
A: ReturnGate.Filter() 会将敏感信息替换为占位符：
- API Key → `[redacted]`
- 密码 → `[redacted]`
- 提示注入 → `[removed unsafe instruction]`
- 系统提示 → `[removed system prompt reference]`

**Q: 如何添加自定义规则?**
A: 修改 `gates/policy.go` 中的 `rulePatterns` 映射，添加新的正则表达式规则。

## 下一步

1. **集成到网关**: 三级策略闸门已集成到AegisProxy
2. **查看决策历史**: 使用 `/aegis/gate/overview` 和 `/aegis/gate/decisions` API
3. **手动评估**: 使用 `/aegis/gate/evaluate` 进行手动测试
4. **监控和调整**: 根据实际场景调整评分阈值和规则

## 参考文件

- [MessageGate](./gates/message.go) - 消息门控实现
- [ActionGate](./gates/action.go) - 动作门控实现  
- [ReturnGate](./gates/return.go) - 返回门控实现
- [PolicyEngine](./gates/policy.go) - 统一评分引擎
- [集成测试](./gates/gates_integration_test.go) - 完整测试套件
