# AegisGuard 接口契约与协作指南

> 本文档遵循 **"接口先行、并行开发、PR 无冲突合并"** 的团队协作模式。
> 接口定义在**消费方**，实现在**生产方**，通过 Go 隐式接口实现自动完成编译期校验。

---

## 架构总览：控制平面与执行平面

代码采用 **逻辑分离、物理聚合** 的双平面架构。虽然是单进程单 exe，但两个平面在代码层面独立：

```
┌──────────────────────────────────────────────────────────────────┐
│                    http/（组合根/路由层）                         │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  只依赖 contract/ 接口，不直接 import 任何实现包           │   │
│  └──────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌──────────────────────────────────────────────────────────────────┐
│                    contract/（双平面通信契约）                    │
│                                                                  │
│  TokenIssuer  ← 控制平面暴露 | 执行平面暴露 →  GateEvaluator    │
│  TokenVerifier ← 控制平面暴露 | 执行平面暴露 →  SandboxManager  │
│  PolicyEngine  ← 控制平面暴露 | 执行平面暴露 →  GateQuery       │
│  AuditReader   ← 控制平面暴露 | 执行平面暴露 →  ContentFilter   │
│                                               TransferManager   │
└──────────────────────────────────────────────────────────────────┘
                     ▲                              ▲
                     │                              │
                     ▼                              ▼
┌─────────────────────────┐         ┌──────────────────────────────┐
│   控制平面（控制平面）      │         │    执行平面（执行平面）       │
│                         │         │                              │
│  auth/  → Token 签发    │         │  gates/  → Message/Action/  │
│  audit/ → 审计与归因     │         │            Return Gate       │
│  rules/ → 攻击规则       │         │  gateway/ → 反向代理 + 凭据  │
│                         │         │  sandbox/ → 记忆沙箱隔离      │
└─────────────────────────┘         └──────────────────────────────┘
```

### import 方向规则

```
控制平面包 ──→ contract/ ←── 执行平面包
     │                        │
     └──── 不跨平面 import ────┘
```

即：`auth/` 不能 import `gates/`，`gateway/` 不能 import `audit/`。`http/`（组合根）是唯一 allowed 的例外，它可以 import 所有包进行依赖注入。

---

## 协作原则

| 原则 | 说明 |
|---|---|
| **谁消费，谁定义** | 接口定义在调用该接口的包或 `contract/` 中，不在中央仓库 |
| **小接口** | 每个接口只包含 1-3 个方法，恰好满足消费方需求 |
| **隐式实现** | 实现方不需要 `implements` 关键字，直接拥有方法即满足接口 |
| **一人一个包** | 每个组员在自己的包内开发，不交叉编辑同一个文件 |

---

## 分工映射表（含平面标注）

### 分工1 — MessageGate / ActionGate / ReturnGate（执行平面）

| 你的任务 | 在哪个包 | 当前状态 | 平面 |
|---|---|---|---|
| MessageGate 风险检测/阻断/降级 | [`gates/message.go`](internal/gates/message.go) | 部分完成 | 执行平面 |
| ActionGate 注入检测/参数分析 | [`gates/action.go`](internal/gates/action.go) | 部分完成 | 执行平面 |
| ReturnGate PII/敏感数据过滤 | [`gates/return.go`](internal/gates/return.go) | 未实现 | 执行平面 |
| PolicyEngine 三门策略统一 | 新建 `gates/policy.go` | 未实现 | 控制平面（定义在 `contract/policy.go`） |
| 代理层的阻断/降级/拒绝执行 | [`gateway/proxy.go`](internal/gateway/proxy.go) 私有方法 | 仅打日志 | 执行平面 |

### 分工2 — RequireToken 与授权链路（跨平面）

| 你的任务 | 在哪个包 | 当前状态 | 平面 |
|---|---|---|---|
| Token 签发（Issue/Revoke/List/GetByID） | [`auth/token.go`](internal/auth/token.go) | 部分完成 | **控制平面**（接口定义见 `contract/token.go`） |
| Token 校验（Verify/Nonce/Binding/Scope） | [`auth/verifier.go`](internal/auth/verifier.go) | 部分完成 | **执行平面**（接口定义见 `contract/token.go`） |
| 替换 placeholder-token 为真实签发 | [`gateway/proxy.go`](internal/gateway/proxy.go) `injectToken` | 实现有误 | 执行平面 |

### 分工3 — 后端 API 与前后端联调（组合根）

> 分工3 的核心任务：**把前端页面接上真实后端**。每个 API 需要同时完成后端 handler 和前端联调验证。

| 你的任务 | 后端实现 | 前端 API 文件 | 前端页面 | 依赖的 contract 接口 | 当前状态 |
|---|---|---|---|---|---|
| 健康检查 `/api/health` | [`http/router.go`](internal/http/router.go) | — | 系统总览 | 无 | 已完成 |
| 签发 Token `POST /aegis/auth/token` | 新建 `http/handler_auth.go` | [`auth.ts`](../frontend/src/api/auth.ts) `issueToken()` | 国密授权中心 | `contract.TokenIssuer` | 未实现 |
| 查询 Token `GET /aegis/auth/token` | 新建 `http/handler_auth.go` | [`auth.ts`](../frontend/src/api/auth.ts) `getTokenInfo()` | 国密授权中心 | `contract.TokenIssuer` | 未实现 |
| 验证 Token `POST /aegis/auth/verify` | 新建 `http/handler_auth.go` | [`auth.ts`](../frontend/src/api/auth.ts) `verifyToken()` | 国密授权中心 | `contract.TokenVerifier` | 未实现 |
| 授权状态 `GET /aegis/auth/status` | 新建 `http/handler_auth.go` | [`auth.ts`](../frontend/src/api/auth.ts) `getAuthStatus()` | 国密授权中心 | `contract.TokenIssuer` | 未实现 |
| 三门概览 `GET /aegis/gate/overview` | 新建 `http/handler_gate.go` | [`gate.ts`](../frontend/src/api/gate.ts) `getGateOverview()` | 阻断决策中心 | `contract.GateQuery` | 未实现 |
| 决策历史 `GET /aegis/gate/decisions` | 新建 `http/handler_gate.go` | [`gate.ts`](../frontend/src/api/gate.ts) `getGateDecisions()` | 阻断决策中心 | `contract.GateQuery` | 未实现 |
| 手动评估 `POST /aegis/gate/evaluate` | 新建 `http/handler_gate.go` | [`gate.ts`](../frontend/src/api/gate.ts) `evaluateGate()` | 阻断决策中心 | `contract.GateEvaluator` | 未实现 |
| 审计日志 `GET /audit/logs` | [`http/router.go`](internal/http/router.go) | [`audit.ts`](../frontend/src/api/audit.ts) `getAuditLogs()` | 审计追踪 | 无（已直连 Store） | 已完成 |
| 攻击链 `GET /aegis/audit/chains` | 新建 `http/handler_audit.go` | [`audit.ts`](../frontend/src/api/audit.ts) `getAttackChains()` | 审计追踪 | `contract.AuditReader` | 未实现 |
| 审计统计 `GET /aegis/audit/stats` | 新建 `http/handler_audit.go` | [`audit.ts`](../frontend/src/api/audit.ts) `getAuditStats()` | 审计追踪 | `contract.AuditReader` | 未实现 |
| 沙箱上下文 `GET /aegis/sandbox/context` | 新建 `http/handler_sandbox.go` | [`sandbox.ts`](../frontend/src/api/sandbox.ts) `getSandboxContext()` | 记忆沙箱 | `contract.SandboxManager` | 未实现 |
| 转移记录 `GET /aegis/sandbox/transfers` | 新建 `http/handler_sandbox.go` | [`sandbox.ts`](../frontend/src/api/sandbox.ts) `getTransferRecords()` | 记忆沙箱 | `contract.TransferManager` | 未实现 |
| 上下文隔离 `POST /aegis/sandbox/isolate` | 新建 `http/handler_sandbox.go` | [`sandbox.ts`](../frontend/src/api/sandbox.ts) `isolateContext()` | 记忆沙箱 | `contract.SandboxManager` | 未实现 |
| 策略配置 `GET /aegis/policy/config` | 新建 `http/handler_policy.go` | [`policy.ts`](../frontend/src/api/policy.ts) `getPolicyConfig()` | 策略中心 | `contract.PolicyEngine` | 未实现 |
| 策略规则 `GET /aegis/policy/rules` | 新建 `http/handler_policy.go` | [`policy.ts`](../frontend/src/api/policy.ts) `getPolicyRules()` | 策略中心 | `contract.PolicyEngine` | 未实现 |
| 更新策略 `PUT /aegis/policy/rules` | 新建 `http/handler_policy.go` | [`policy.ts`](../frontend/src/api/policy.ts) `updatePolicyRule()` | 策略中心 | `contract.PolicyEngine` | 未实现 |

### 分工4 — Sandbox 与返回隔离（执行平面）

| 你的任务 | 在哪个包 | 当前状态 | 平面 |
|---|---|---|---|
| Memory Sandbox 上下文隔离 | [`sandbox/sandbox.go`](internal/sandbox/sandbox.go) | 未实现 | 执行平面 |
| 可信/不可信数据转移 | 新建 `sandbox/transfer.go` | 未实现 | 执行平面 |
| 安全摘要/工具返回过滤 | 新建 `sandbox/filter.go` | 未实现 | 执行平面 |

---

## 包依赖关系

```
                          ┌──────────────────┐
                          │   http/（路由层）  │
                          │   组合根，负责     │
                          │   依赖注入和路由   │
                          └────────┬─────────┘
                                   │ 依赖 contract/ 接口
                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                     contract/（双平面通信契约）                    │
│  TokenIssuer / TokenVerifier / PolicyEngine / AuditReader       │
│  GateQuery / GateEvaluator / SandboxManager / TransferManager   │
│  ContentFilter                                                  │
└─────┬───────────────────────────────────────────┬───────────────┘
      │ 隐式实现                                    │ 隐式实现
      ▼                                            ▼
┌──────────────┐                          ┌──────────────────────┐
│ 控制平面       │                          │ 执行平面              │
│              │                          │                      │
│ auth/       │←── 不跨平面 ──→ 不跨平面 ──→│ gates/              │
│ audit/      │                          │ gateway/ (自身接口)    │
│ rules/      │                          │ sandbox/             │
└──────────────┘                          └──────────────────────┘
                      ▲                     │
                      │         ┌───────────┘
                      │         │
               interfaces/（纯 DTO，所有包均可引用）
```

**关键约束：**
- 控制平面包（`auth/`, `audit/`, `rules/`）**不 import** 任何执行平面包
- 执行平面包（`gates/`, `gateway/`, `sandbox/`）**不 import** 任何控制平面包
- `contract/` 只定义接口和 DTO，**不包含任何业务逻辑**
- `interfaces/` 只包含结构体，**不包含任何接口定义**
- `http/`（组合根）是唯一可引用所有具体包的例外

---

## 如何编写测试

### 分工1 测试（gates 包）

直接测试具体类型，无需 mock：

```go
// gates/message_test.go
func TestMessageGate_Evaluate(t *testing.T) {
    gate := &MessageGate{}
    decision, reason := gate.Evaluate([]byte("test input"))
}
```

### 分工2 测试（auth 包）

直接测试具体类型：

```go
// auth/token_test.go
func TestToken_Issue(t *testing.T) {
    token, err := NewToken(...)
}
```

### 分工3 测试（http 包）

利用 `contract/` 接口做轻量 mock，不依赖任何实现包：

```go
// http/handler_test.go
type mockTokenIssuer struct{}

func (m *mockTokenIssuer) Issue(...) (*auth.RequireToken, error) {
    return &auth.RequireToken{TokenID: "test-token"}, nil
}
// ... 只需实现 handler 用到的 1-2 个方法

func TestIssueTokenHandler(t *testing.T) {
    // 注入 mock，不依赖 auth/ 或 gateway/
    router := NewRouter(WithTokenIssuer(&mockTokenIssuer{}))
}
```

### 分工4 测试（sandbox 包）

直接测试具体类型：

```go
// sandbox/sandbox_test.go
func TestCreateContext(t *testing.T) {
    mgr := &SandboxManager{}
    ctx, err := mgr.CreateContext(...)
}
```

---

## 国密算法使用指南

国密算法封装在 [`pkg/smcrypto/`](pkg/smcrypto/) 下，所有函数以 `SM2`、`SM3`、`SM4`、`SM9` 前缀命名，调用方直接导入 `aegisguard/pkg/smcrypto` 即可使用。

| 算法 | 函数 | 用途 | 谁需要导入 |
|---|---|---|---|
| **SM2** | `GenerateKeyPair()`, `SignMessage()`, `VerifySignature()` | RequireToken 数字签名与验签 | 分工2（`auth/token.go` 签发时签名，`auth/verifier.go` 校验时验签） |
| **SM3** | `SM3Sum()`, `SM3SumTruncated()`, `SM3Hex()` | 工具 Schema 指纹（防 Rug Pull）、审计日志摘要 | 分工2（`auth/token.go` 生成 schema_hash）、分工3（`audit/` 生成日志摘要） |
| **SM4** | `SM4GenerateKey()`, `SM4EncryptCBC()`, `SM4DecryptCBC()` | 沙箱敏感数据加密（工具返回的数据库明细、文件内容、API 响应等） | 分工4（`sandbox/filter.go` 中加密敏感返回，需要时才解密回传摘要） |
| **SM9** | `SM9GenerateSignMasterKey()`, `SM9GenerateSignKey()`, `SM9Sign()`, `SM9Verify()` | 多 Agent 身份绑定，无需证书，用 agent_id 作为标识公钥 | 分工2（`auth/token.go` 可选：多 Agent 场景下用 SM9 替代传统证书） |

### 使用示例

导入方式：

```go
import "aegisguard/pkg/smcrypto"
```

SM4 加密沙箱敏感数据：

```go
key, _ := smcrypto.SM4GenerateKey()
encrypted, _ := smcrypto.SM4EncryptCBC(key, []byte("敏感数据"))
decrypted, _ := smcrypto.SM4DecryptCBC(key, encrypted)
```

SM9 派生 Agent 签名密钥：

```go
masterPriv, masterPub, _ := smcrypto.SM9GenerateSignMasterKey()
agentKey, _ := smcrypto.SM9GenerateSignKey(masterPriv, []byte("agent-001@domain"))
sig, _ := smcrypto.SM9Sign(agentKey, smcrypto.SM3Sum([]byte("消息")))
ok := smcrypto.SM9Verify(masterPub, []byte("agent-001@domain"), smcrypto.SM3Sum([]byte("消息")), sig)
```


## PR 合并流程

1. **各自在自己的包开发**，不修改他人包内的文件
2. **PR 互不冲突**，因为每个组员操作的是不同的目录
3. **唯一需协调的点**：分工2 完成 `TokenInjector` 实现后，需要修改 [`gateway/proxy.go`](internal/gateway/proxy.go) 的 `injectToken` 方法——这一行改动应在分工2 的 PR 中附带

---

## 状态标记速查

| 标记 | 含义 | 行动 |
|---|---|---|
| 已完成 | 已实现且已被调用 | 无需操作 |
| 部分完成 | 已实现但只完成部分 | 补齐剩余逻辑 |
| 实现有误 | 已实现但逻辑有误 | 修正实现 |
| 未实现 | 完全未实现 | 从零实现 |
| 未接入 | 实现无误但未被调用 | 接入调用方 |
