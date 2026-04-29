# AegisGuard Backend

AegisGuard 后端——面向 Agent 执行链路的动态防护与行为管控系统。采用**非侵入式反向代理架构**，Agent 仅需修改 `BASE_URL` 配置即可接入，无需改动任何业务代码。

---

## 核心特性

| 特性 | 状态 | 说明 |
|:---|:---|:---|
| **零侵入接入** | ✅ 已完成 | Agent 通过配置 `BASE_URL` 指向网关，无需代码改动 |
| **虚拟密钥签发** | 🔧 开发中 | 动态签发 `vsk-` 前缀虚拟密钥，网关自动替换为真实 API Key |
| **三层安全闸门** | ✅ 已完成 | Message Gate → Action Gate → Return Gate，覆盖执行全链路 |
| **可信授权链路** | 🔧 工程化中 | RequireToken 机制，基于国密 SM2/SM3/SM4 |
| **双流上下文隔离** | ✅ 已完成 | Trusted Core Context / Sandbox Context，阻断外部污染 |
| **攻击链审计** | ✅ 已完成 | 时序因果 DAG，支持阻断节点定位与路径回溯 |

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

服务启动后监听 `:8090`（可通过 `PORT` 环境变量修改）。

### 3. Agent 接入（零代码改动）

修改你的 Agent 配置文件或环境变量：

```bash
# 原来
OPENAI_BASE_URL="https://api.openai.com/v1"
OPENAI_API_KEY="sk-your-real-key"

# 接入 AegisGuard 后（仅改这两行）
OPENAI_BASE_URL="http://localhost:8090/v1"
OPENAI_API_KEY="vsk-dev-001"  # 使用虚拟密钥，网关自动替换为真实密钥
```

**完成。** Agent 所有流量自动经过安全网关。

> **注意**：虚拟密钥需通过 `/admin/vkeys` API 或配置文件预先创建。详见[虚拟密钥管理](#虚拟密钥管理)章节。

---

## 目录结构

```
backend/
├── cmd/server/           # 服务入口
├── config/               # 配置文件目录
│   └── vkeys.yaml        # 虚拟密钥配置文件
├── internal/
│   ├── gateway/          # 反向代理网关（流量入口）
│   ├── gates/            # 三层安全闸门（核心防护）
│   ├── auth/             # RequireToken 授权链路
│   ├── vkey/             # 虚拟密钥管理（动态签发）
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
| `gateway/` | HTTP 反向代理，请求解析与转发，拦截点挂载 | 2.2.2 执行平面模块 |
| `gates/` | Message/Action/Return Gate 策略决策 | 2.2.3 三层安全闸门 |
| `auth/` | RequireToken 签发、校验、SM2 签名 | 2.4.2 可信授权与密码学机制 |
| `vkey/` | 虚拟密钥签发、验证、吊销、生命周期管理 | 虚拟密钥管理 |
| `http/` | HTTP 路由注册、请求处理、管理 API | - |
| `sandbox/` | 可信/不可信上下文隔离、摘要回传 | 2.4.3 上下文污染防护机制 |
| `audit/` | 结构化日志、时序因果 DAG、攻击链重建 | 2.4.4 审计日志与攻击路径重建 |
| `control/` | 策略中心、密钥管理（控制平面信任根） | 2.2.1 控制平面模块 |

---

## 虚拟密钥管理

### 概述

虚拟密钥（Virtual Key）是 AegisGuard 的核心接入机制。Agent 使用 `vsk-` 前缀的虚拟密钥访问网关，网关自动替换为真实的 LLM API Key，实现**零侵入安全接入**。

### 密钥格式

```
vsk-{uuid前8位}-{uuid前4位}
```

示例：`vsk-a1b2c3d4-e5f6`

### 密钥结构

| 字段 | 说明 | 示例 |
|:---|:---|:---|
| `key_id` | 虚拟密钥 ID | `vsk-dev-001` |
| `agent_id` | Agent 唯一标识 | `agent-dev-001` |
| `real_api_key` | 真实 LLM API Key | `sk-xxxxx` |
| `scope` | 权限范围 | `read` / `write` / `admin` |
| `rate_limit` | 每分钟最大请求数 | `60` |
| `expires_at` | 过期时间 | `2026-12-31T23:59:59Z` |
| `enabled` | 是否启用 | `true` / `false` |

### 管理 API

| 端点 | 方法 | 说明 |
|:---|:---|:---|
| `/admin/vkeys` | GET | 列出所有虚拟密钥（脱敏） |
| `/admin/vkeys` | POST | 创建虚拟密钥 |
| `/admin/vkeys/:id` | GET | 获取虚拟密钥详情 |
| `/admin/vkeys/:id` | DELETE | 吊销虚拟密钥 |

#### 创建虚拟密钥示例

```bash
curl -X POST http://localhost:8090/admin/vkeys \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "my-agent-001",
    "real_api_key": "sk-your-real-openai-key",
    "scope": "write",
    "rate_limit": 60
  }'
```

响应：
```json
{
  "virtual_key": {
    "key_id": "vsk-a1b2c3d4-e5f6",
    "agent_id": "my-agent-001",
    "scope": "write",
    "rate_limit": 60,
    "enabled": true
  },
  "message": "虚拟密钥创建成功，请妥善保管"
}
```

### 配置文件

虚拟密钥可通过配置文件静态定义（`config/vkeys.yaml`）：

```yaml
virtual_keys:
  - key_id: vsk-dev-001
    agent_id: agent-dev-001
    real_api_key: sk-your-real-openai-key-here
    scope: write
    rate_limit: 60
    expires_at: "2026-12-31T23:59:59Z"
    enabled: true
```

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
| `AEGIS_TARGET_URL` | 是 | `https://api.openai.com` | 真实 LLM API 地址 |
| `AEGIS_LISTEN_ADDR` | 否 | `:8090` | 网关监听地址 |
| `AEGIS_LOG_LEVEL` | 否 | `info` | 日志级别：debug/info/warn/error |
| `AEGIS_POLICY_MODE` | 否 | `balanced` | 策略模式：loose/balanced/strict |
| `AEGIS_VKEY_CONFIG` | 否 | `config/vkeys.yaml` | 虚拟密钥配置文件路径 |
| `AEGIS_VKEY_STORE_DSN` | 否 | `sqlite:data/vkeys.db` | 虚拟密钥存储后端（sqlite/memory） |

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

# 2. 创建虚拟密钥
curl -X POST http://localhost:8090/admin/vkeys \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "test-agent",
    "real_api_key": "sk-your-real-key",
    "scope": "write"
  }'

# 3. 使用虚拟密钥测试连通性
curl http://localhost:8090/v1/models \
  -H "Authorization: Bearer vsk-xxxx-xxxx"

# 4. 查看所有虚拟密钥
curl http://localhost:8090/admin/vkeys

# 5. 观察日志输出，确认密钥替换和安全检查
```

### 与前端联调

前端演示服务启动后访问 `http://localhost:8090`，可查看：
- 实时阻断决策
- 国密授权中心
- 记忆沙箱状态
- 审计追踪与攻击链图谱

---


---

## 注意事项

### 当前状态

1. **虚拟密钥存储**：✅ SQLite 持久化存储，支持内存缓存加速，重启后数据不丢失
2. **RequireToken SM2 签名**：已集成 `pkg/smcrypto` 国密库
3. **Nonce 防重放**：当前为内存缓存，生产环境需接入 Redis
4. **性能优化**：审计日志写入可改为异步，降低关键路径延迟

### 设计理念：竞赛作品 × 工业雏形

本项目虽源于全国大学生信息安全竞赛，但设计之初即以**可发展为工业产品**为目标。核心原则：

- **接口抽象先行**：存储层、缓存层、消息层均设计为可插拔接口，SQLite → PostgreSQL、内存 → Redis 可无缝切换
- **领域驱动设计**：模块职责边界清晰，auth/vkey/audit/rules 四大领域独立演进，不相互渗透
- **性能与持久化解耦**：高频读写走内存（sync.Map），冷数据落数据库，零吹牛皮的实用主义

> 评委既考察安全性，也会评估架构是否有「从宿舍到工厂」的能力。

### 开发目标（虚拟密钥签发）

| 功能 | 当前状态 | 目标 | 工业价值 |
|:---|:---|:---|:---|
| 持久化存储 | ✅ SQLite 已完成 | PostgreSQL 双模式 | 赛道一：密钥持久化是生产环境刚需 |
| 动态签发 API | ✅ 已完成 | REST API 创建/吊销/轮换 | 赛道二：密钥生命周期管理是企业标配 |
| 权限模型 | ⚠️ 仅 scope | RBAC + ABAC 混合权限 | 赛道三：细粒度权限是商业化核心 |
| 调用追踪 | ⚠️ 内存计数 | 分布式计数器 + 审计日志 | 赛道四：可观测性是 SRE 核心诉求 |
| 密钥轮换 | ❌ 无 | 新旧密钥并行 + 自动失效 | 赛道五：密钥轮换是安全合规要求 |
| 自动清理 | ⚠️ 惰性检查 | 定时任务 + 事件触发双机制 | 赛道六：自动化运维减少人工干预 |

---

## 许可证

本项目为竞赛作品，代码仅供学习研究使用。

---
