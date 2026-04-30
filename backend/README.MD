# AegisGuard Backend

AegisGuard 后端——面向 Agent 执行链路的运行时安全防护与行为管控系统。采用**非侵入式反向代理架构**，Agent 仅需修改 `BASE_URL` 和 `API_KEY` 配置即可接入，无需改动任何业务代码。

---

## 核心特性

| 特性 | 状态 | 说明 |
|:---|:---|:---|
| **零侵入接入** | ✅ 已完成 | Agent 通过配置 `BASE_URL` 和网关密钥接入，无需代码改动 |
| **凭据收敛** | ✅ 已完成 | 统一网关密钥(agk-xxx)验证身份，LLM API Key 由网关托管，Agent 无感知 |
| **凭据保险箱** | ✅ 已完成 | 管理员在 `gateway.yaml` 中配置真实 LLM Key，Agent 永远看不到 |
| **三层安全闸门** | ✅ 已完成 | Message Gate → Action Gate → Return Gate，覆盖执行全链路 |
| **可信授权链路** | 🔧 工程化中 | RequireToken 机制，基于国密 SM2/SM3/SM4 |
| **双流上下文隔离** | ✅ 已完成 | Trusted Core Context / Sandbox Context，阻断外部污染 |
| **攻击链审计** | ✅ 已完成 | 时序因果 DAG，支持阻断节点定位与路径回溯 |

---

## 架构设计：凭据收敛模型

AegisGuard 面向**企业内部自部署**场景设计。管理员在 `gateway.yaml` 中设置网关密钥和 LLM
API Key，所有 Agent 使用统一的`agk-`前缀网关密钥接入，LLM 凭据由网关完全托管。

```
┌──────────────────────────────────────────────────────────────┐
│  Layer 1: Gateway Key (网关身份凭据)                           │
│  - agk-dev-001，管理员在 gateway.yaml 中统一配置                │
│  - Agent 的 .env 中配置 OPENAI_API_KEY=agk-dev-001            │
│  - 仅用于证明"此 Agent 是该网关的合法调用方"                     │
│  - 泄露后管理员修改 gateway.yaml 即可，所有 Agent 同步更新       │
└──────────────────────────────────────────────────────────────┘
                              │ 每次请求携带
                              ▼
┌──────────────────────────────────────────────────────────────┐
│  Layer 2: RequireToken (短期授权令牌)                          │
│  - 由控制平面签发，TTL = 5 分钟                                │
│  - 包含: ToolName, Scope, RiskLevel, MaxCalls, Nonce          │
│  - SM2 签名防篡改，Nonce 防重放                                │
│  - 过期后网关自动重新签发，Agent 无感知                         │
└──────────────────────────────────────────────────────────────┘
                              │ 网关验证后
                              ▼
┌──────────────────────────────────────────────────────────────┐
│  Layer 3: LLM API Key (后端 LLM 凭据)                         │
│  - 仅存在于网关进程内存中，Agent 永远不会接触到                  │
│  - 管理员在 gateway.yaml 中配置，支持环境变量注入               │
│  - 密钥轮换仅需修改 gateway.yaml 后重启网关，Agent 侧零变更     │
└──────────────────────────────────────────────────────────────┘
```

**核心原则：**
- **凭据不落地**：真实 LLM API Key 仅存在于网关受控内存，Agent 只持有网关密钥
- **Agent 保持"愚钝"**：`.env` 只配一次 `agk-xxx`，永远不改
- **网关承担所有安全责任**：身份校验、密钥替换、安全闸门全部封死在网关背后
- **管理员统一管控**：所有密钥配置收敛到 `gateway.yaml`，无需逐个管理 Agent 凭据

---

## 快速开始

### 1. 环境要求

- Go 1.25+
- （可选）Docker，用于容器化部署

### 2. 配置网关凭据

编辑 `config/gateway.yaml`：

```yaml
gateway_key: agk-dev-001
target_url: https://api.openai.com
llm_api_key: sk-your-real-llm-api-key-here
```

生产环境建议通过环境变量 `AEGIS_TARGET_URL` 和 `AEGIS_LLM_API_KEY` 注入敏感字段，避免明文写入文件。

### 3. 启动网关

```bash
cd backend

# 启动服务
go run cmd/server/main.go
```

服务启动后监听 `:8090`（可通过 `PORT` 环境变量修改）。

### 4. Agent 接入（零代码改动）

修改你的 Agent 配置文件或环境变量：

```bash
# 原来
OPENAI_BASE_URL="https://api.openai.com/v1"
OPENAI_API_KEY="sk-your-real-key"

# 接入 AegisGuard 后（仅改这两行）
OPENAI_BASE_URL="http://localhost:8090/v1"
OPENAI_API_KEY="agk-dev-001"  # 使用网关密钥，网关自动替换为真实 LLM Key
```

**完成。** Agent 所有流量自动经过安全网关。

---

## 目录结构

```
backend/
├── cmd/server/           # 服务入口
├── config/               # 配置文件目录
│   └── gateway.yaml      # 网关凭据配置（网关密钥 + LLM API Key）
├── internal/
│   ├── gateway/          # 反向代理网关（流量入口、凭据替换）
│   ├── gates/            # 三层安全闸门（核心防护）
│   ├── auth/             # RequireToken 授权链路
│   ├── vkey/             # 网关密钥校验与凭据管理
│   ├── http/             # HTTP 路由与处理器
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
| `gateway/` | HTTP 反向代理，请求转发，凭据替换（agk → sk） | 2.2.2 执行平面模块 |
| `gates/` | Message/Action/Return Gate 策略决策 | 2.2.3 三层安全闸门 |
| `auth/` | RequireToken 签发、校验、SM2 签名 | 2.4.2 可信授权与密码学机制 |
| `vkey/` | 网关密钥校验、LLM API Key 托管与注入 | 网关凭据管理 |
| `http/` | HTTP 路由注册、请求处理、管理 API | - |
| `sandbox/` | 可信/不可信上下文隔离、摘要回传 | 2.4.3 上下文污染防护机制 |
| `audit/` | 结构化日志、时序因果 DAG、攻击链重建 | 2.4.4 审计日志与攻击路径重建 |
| `control/` | 策略中心、密钥管理（控制平面信任根） | 2.2.1 控制平面模块 |

---

## 网关凭据管理

### 概述

AegisGuard 采用**凭据收敛**设计。管理员在 `config/gateway.yaml` 中统一配置以下凭据：

| 凭据 | 说明 | 谁持有 | 示例 |
|:---|:---|:---|:---|
| `gateway_key` | 网关身份凭据，Agent 配置在 `.env` 中用于接入网关 | Agent + 网关 | `agk-dev-001` |
| `target_url` | 真实 LLM API 地址，网关将请求转发到此地址 | 仅网关（管理员配置） | `https://api.openai.com` |
| `llm_api_key` | 真实 LLM API Key，由网关托管注入到请求中 | 仅网关（管理员配置） | `sk-xxxxx` |

### 设计哲学

- **凭据收敛**：所有凭据管理集中到 `gateway.yaml`，管理员只需维护一个配置文件
- **凭据不落地**：LLM API Key 仅存在于网关进程内存，Agent 永远接触不到
- **单点管控**：真实 Key 轮换只需修改 `gateway.yaml` 重启网关，所有 Agent 侧无需任何变更
- **身份与凭据分离**：`gateway_key` 仅用于身份校验，`llm_api_key` 是后端调用凭据，两者完全解耦

### 密钥格式

```
gateway_key: agk-{名称或标识}
```

示例：`agk-dev-001`、`agk-prod-01`、`agk-default`

### Agent 配置方式

Agent 的 `.env` 文件中只需配置统一的网关密钥：

```bash
OPENAI_BASE_URL=http://aegis-gateway:8090/v1
OPENAI_API_KEY=agk-dev-001
```

多个 Agent 可共用同一个网关密钥。如需区分不同团队，可配置多个 `gateway_key`（详见配置文件）。

### 凭据轮换

当真实 LLM API Key 需要更新时：

1. 编辑 `config/gateway.yaml`，修改 `llm_api_key` 字段
2. 重启网关
3. **Agent 侧无需任何修改**，`agk-xxx` 保持不变

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

### 认证流程（凭据替换）

```
Agent 请求:  Authorization: Bearer agk-dev-001
                │
                ▼
        1. 提取网关密钥 agk-dev-001
        2. 校验是否匹配 gateway.yaml 中的 gateway_key
        3. 验证通过后，从 gateway.yaml 读取 llm_api_key
        4. 替换 Authorization 头: Bearer sk-xxxxx
        5. 转发请求到真实 LLM API
                │
                ▼
        LLM API 收到真实凭据，正常处理
```

---

## 配置说明

### 环境变量

| 变量 | 必填 | 默认值 | 说明 |
|:---|:---|:---|:---|
| `AEGIS_GATEWAY_CONFIG` | 否 | `config/gateway.yaml` | 网关凭据配置文件路径 |
| `AEGIS_TARGET_URL` | 否 | 读取 `gateway.yaml` | 覆盖配置文件中的真实 LLM API 地址 |
| `AEGIS_LLM_API_KEY` | 否 | 读取 `gateway.yaml` | 覆盖配置文件中的真实 LLM API Key |
| `PORT` | 否 | `8090` | 网关监听端口 |
| `AEGIS_LOG_LEVEL` | 否 | `info` | 日志级别：debug/info/warn/error |
| `AEGIS_POLICY_MODE` | 否 | `balanced` | 策略模式：loose/balanced/strict |

### 网关凭据配置文件 (`config/gateway.yaml`)

```yaml
# AegisGuard 网关凭据配置
gateway_key: agk-dev-001
target_url: https://api.openai.com
llm_api_key: sk-your-real-llm-api-key-here
```

- `gateway_key`：Agent 侧配置的统一网关密钥，必须以 `agk-` 开头
- `target_url`：真实的 LLM API 地址，网关将请求转发到此地址
- `llm_api_key`：网关托管的真实 LLM API Key，仅存在于网关内存中
- 生产环境可通过 `AEGIS_TARGET_URL` 和 `AEGIS_LLM_API_KEY` 环境变量覆盖对应字段

---

## 场景与接入指南

### 企业内部自建 LLM（vLLM / Ollama）

```yaml
# gateway.yaml
gateway_key: agk-internal-001
target_url: "http://vllm-internal:8000"
llm_api_key: ""                    # 内网服务通常不需要 API Key
```

### 调用商业 LLM API（OpenAI / DeepSeek / Qwen）

```yaml
# gateway.yaml
gateway_key: agk-default-001
target_url: "https://api.openai.com"
llm_api_key: "sk-xxxxxxxxxxxxxxxxxxxx"    # 企业统一的 LLM API Key
```

### 多团队、多模型场景

对于需要管理多个 LLM API Key 的企业（少量 Key，如 3~5 个），可启动多个网关实例：

```
网关实例A（研发团队）：
  gateway_key: agk-rd-001
  target_url: https://api.deepseek.com
  llm_api_key: sk-deepseek-key

网关实例B（数据团队）：
  gateway_key: agk-data-001
  target_url: https://dashscope.aliyuncs.com
  llm_api_key: sk-qwen-key
```

---

## 开发指南

### 添加新的 Gate 决策逻辑

在 `internal/gates/` 下编辑对应文件，实现 `Evaluate` 方法：

```go
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
# 1. 编辑 config/gateway.yaml，设置网关密钥和真实 LLM Key
#    gateway_key: agk-dev-001
#    llm_api_key: sk-your-real-key

# 2. 启动网关
go run cmd/server/main.go

# 3. 使用网关密钥测试连通性
curl http://localhost:8090/v1/models \
  -H "Authorization: Bearer agk-dev-001"

# 4. 使用错误的密钥测试（应返回 401）
curl http://localhost:8090/v1/models \
  -H "Authorization: Bearer agk-wrong-key"

# 5. 观察日志输出，确认凭据替换和安全检查
```

### 前端展示

前端演示服务启动后访问 `http://localhost:8090`（需启动前端服务），可查看：
- 实时阻断决策
- 国密授权中心
- 记忆沙箱状态
- 审计追踪与攻击链图谱

---

## 注意事项

### 安全建议

1. **生产环境 `llm_api_key` 通过环境变量注入**，避免明文写入配置文件：
   ```bash
   export AEGIS_LLM_API_KEY="sk-xxxxxxxx"
   ```
2. **网关密钥定期轮换**，修改 `gateway.yaml` 后重启网关即可
3. **TLS 加密**：生产环境必须在网关前端配置 HTTPS，防止网关密钥传输中泄露
4. **最小权限**：LLM API Key 应配置企业账号下的最小必要权限（如仅限特定模型、用量限制）

### 设计理念：竞赛作品 × 工业雏形

本项目虽源于全国大学生信息安全竞赛，但设计之初即以**可发展为工业产品**为目标。核心原则：

- **接口抽象先行**：核心模块均设计为可插拔接口，便于扩展
- **领域驱动设计**：模块职责边界清晰，独立演进
- **实用主义**：平衡开发效率与架构质量

---

## 许可证

本项目为竞赛作品，代码仅供学习研究使用。
