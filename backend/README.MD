 我来为 AegisGuard 后端编写一份清晰、实用的 README，帮助团队快速理解和接入。

---

# AegisGuard Backend

AegisGuard 后端——面向 Agent 执行链路的动态防护与行为管控系统。采用**非侵入式反向代理架构**，Agent 仅需修改 `BASE_URL` 配置即可接入，无需改动任何业务代码。

---

## 核心特性

| 特性 | 说明 |
|:---|:---|
| **零侵入接入** | Agent 通过配置 `BASE_URL` 指向网关，无需代码改动 |
| **三层安全闸门** | Message Gate → Action Gate → Return Gate，覆盖执行全链路 |
| **可信授权链路** | RequireToken 机制，基于国密 SM2/SM3/SM4（工程化中） |
| **双流上下文隔离** | Trusted Core Context / Sandbox Context，阻断外部污染 |
| **攻击链审计** | 时序因果 DAG，支持阻断节点定位与路径回溯 |

---

## 快速开始

### 1. 环境要求

- Go 1.25+
- （可选）Docker，用于容器化部署

### 2. 启动网关

```bash
cd backend

# 设置转发目标（真实 LLM API）
export AEGIS_TARGET_URL="https://api.openai.com"

# 启动服务
go run cmd/server/main.go
```

服务启动后监听 `:8080`。

### 3. Agent 接入（零代码改动）

修改你的 Agent 配置文件或环境变量：

```bash
# 原来
OPENAI_BASE_URL="https://api.openai.com/v1"

# 接入 AegisGuard 后（仅改这一行）
OPENAI_BASE_URL="http://localhost:8080/v1"
```

**完成。** Agent 所有流量自动经过安全网关。

---

## 目录结构

```
backend/
├── cmd/server/           # 服务入口
├── internal/
│   ├── gateway/          # 反向代理网关（流量入口）
│   ├── gates/            # 三层安全闸门（核心防护）
│   ├── auth/             # RequireToken 授权链路
│   ├── sandbox/          # 记忆沙箱与上下文隔离
│   ├── audit/            # 审计日志与攻击链图谱
│   ├── control/          # 控制平面（策略/密钥）
│   └── runtime/          # Agent 框架适配配置
└── pkg/
    └── smcrypto/         # 国密算法封装（SM2/SM3/SM4）
```

### 模块职责

| 目录 | 职责 | 对应文档章节 |
|:---|:---|:---|
| `gateway/` | HTTP 反向代理，请求解析与转发，拦截点挂载 | 2.2.2 执行平面模块 |
| `gates/` | Message/Action/Return Gate 策略决策 | 2.2.3 三层安全闸门 |
| `auth/` | RequireToken 签发、校验、SM2 签名 | 2.4.2 可信授权与密码学机制 |
| `sandbox/` | 可信/不可信上下文隔离、摘要回传 | 2.4.3 上下文污染防护机制 |
| `audit/` | 结构化日志、时序因果 DAG、攻击链重建 | 2.4.4 审计日志与攻击路径重建 |
| `control/` | 策略中心、密钥管理（控制平面信任根） | 2.2.1 控制平面模块 |

---

## 核心工作流程

```
┌─────────┐     ┌──────────────┐     ┌─────────────┐     ┌─────────────┐
│  用户请求 │────→│ Message Gate │────→│  Agent 推理  │────→│ Action Gate │
│ (已拦截) │     │ 输入风险检测  │     │             │     │ 执行前校验   │
└─────────┘     └──────────────┘     └─────────────┘     └──────┬──────┘
                                                                  │
                                                                  ▼
┌─────────┐     ┌──────────────┐     ┌─────────────┐     ┌─────────────┐
│  用户响应 │←────│ Return Gate  │←────│  Agent 决策  │←────│   工具执行   │
│ (已净化) │     │ 结果隔离检测  │     │             │     │             │
└─────────┘     └──────────────┘     └─────────────┘     └─────────────┘
```

**关键设计**：所有控制逻辑内联于代理链路，非旁路检测，确保危险动作在执行前被强制阻断。

---

## 配置说明

通过环境变量配置：

| 变量 | 必填 | 默认值 | 说明 |
|:---|:---|:---|:---|
| `AEGIS_TARGET_URL` | 是 | - | 真实 LLM API 地址 |
| `AEGIS_LISTEN_ADDR` | 否 | `:8080` | 网关监听地址 |
| `AEGIS_LOG_LEVEL` | 否 | `info` | 日志级别：debug/info/warn/error |
| `AEGIS_POLICY_MODE` | 否 | `balanced` | 策略模式：loose/balanced/strict |

---

## 开发指南

### 添加新的 Gate 决策逻辑

在 `internal/gates/` 下编辑对应文件，实现 `Evaluate` 方法：

```go
// gates/action_gate.go
func (ag *ActionGate) Evaluate(toolName string, params map[string]interface{}, headers http.Header) (ActionDecision, string) {
    // 1. 提取 Token
    // 2. 全字段校验
    // 3. 权限范围检查
    // 4. 返回决策：Allow/Deny/Quarantine/HumanApproval/Degrade
}
```

### 接入新的 Agent 框架

在 `internal/runtime/config.go` 添加框架识别逻辑，无需改动 Agent 代码：

```go
func DetectFramework(req *http.Request) FrameworkType {
    // 根据请求特征识别 LangGraph / AutoGen / OpenHands 等
    // 返回对应的安全策略配置
}
```

---

## 测试验证

### 本地快速验证

```bash
# 1. 启动网关
go run cmd/server/main.go

# 2. 测试连通性
curl http://localhost:8080/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"

# 3. 观察日志输出，确认请求经过 Message Gate
```

### 与前端联调

前端演示服务启动后访问 `http://localhost:8080`，可查看：
- 实时阻断决策
- 国密授权中心
- 记忆沙箱状态
- 审计追踪与攻击链图谱

---


---

## 注意事项

1. **当前阶段**：RequireToken 的 SM2 签名使用占位符实现，后续替换为 `pkg/smcrypto` 国密库
2. **Nonce 防重放**：当前为内存缓存，生产环境需接入 Redis
3. **性能优化**：审计日志写入可改为异步，降低关键路径延迟

---

## 许可证

本项目为竞赛作品，代码仅供学习研究使用。

---
