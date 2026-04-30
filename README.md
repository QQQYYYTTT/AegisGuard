# AegisGuard

AegisGuard 是一个面向智能体安全研究与运行时防护原型的项目。当前实验部分已经统一切换为 **ASB-first** 路线：使用原始 Agent Security Benchmark（ASB）作为主要 benchmark。

## 当前定位

本仓库现在承担两类任务：

- 运行时防护原型：保留 Go 后端、前端展示、权限校验、审计记录和运行时安全控制模块。
- ASB 实验适配：通过 `experiments/asb/` 调用外部 ASB 仓库，并把 ASB 输出转换为 AegisGuard 统一结果格式。

## 适用场景与用户群体

### 产品形态

AegisGuard 定位为**企业内部可自部署的运行时安全中间件**，采用控制平面与执行平面解耦架构，以非侵入式反向代理方式接入现有 Agent 系统。它不是面向终端个人用户的 SaaS 服务。

### 目标用户

| 用户群体 | 典型角色 | 关注点 |
|:---|:---|:---|
| **Agent 平台开发者与集成方** | 框架开发者、平台运维 | 非侵入接入、兼容 LangChain/AutoGen/OpenHands/DB-GPT 等主流框架、BASE_URL 重定向 / Sidecar 代理两种模式 |
| **企业安全团队与合规负责人** | CSO、安全架构师、合规审计 | Agent 行为全链路追溯、审计日志与攻击链图谱、满足数据安全与日志留存监管要求 |
| **高合规行业安全运维人员** | 政务/金融/关基安全运维 | 国密算法（SM2/SM3/SM4/SM9）支持、凭据收敛、调用链保护、密码合规 |

### 适用场景

1. **企业 Agent 安全接入**：将企业内部多个 Agent 的 LLM 调用统一收敛到安全网关，实现凭据托管、请求审计和策略管控。
2. **零信任 Agent 执行环境**：在 Agent 与外部工具/MCP 服务之间部署安全闸门，阻断提示注入、工具误调用和数据泄露。
3. **合规审计与攻击溯源**：通过全链路审计日志和时序因果 DAG，为事后归因、过程解释和合规审计提供证据支持。
4. **企业内部开发测试**：开发团队在测试环境中通过网关模拟 LLM 调用，安全团队提前验证策略有效性。

### 不适用的场景

| 场景 | 原因 |
|:---|:---|
| **面向终端个人用户的 SaaS 服务** | AegisGuard 不提供多租户隔离、用户注册、计费等 SaaS 典型能力，定位为私有化部署 |
| **需要 BYOK 的第三方平台** | AegisGuard 采用凭据收敛模式，LLM API Key 由企业自己管理在网关内，不代理转发用户自持密钥 |
| **对 Agent 源码有侵入式修改需求的场景** | AegisGuard 的核心设计前提是零侵入接入，若预期对 Agent 源码深度定制，本系统非最优选择 |
| **公网暴露的 Agent 服务** | AegisGuard 当前面向内网部署，公网暴露需要额外配置 TLS 和认证层，不属于默认支持范围 |

## 目录结构

```text
AegisGuard/
|-- backend/                 # Go 后端与运行时防护原型
|   |-- cmd/server/          # 服务入口
|   |-- config/              # 配置文件目录
|   |-- internal/
|   |   |-- gateway/         # 反向代理网关（流量入口、凭据替换）
|   |   |-- gates/           # 三层安全闸门（核心防护）
|   |   |-- auth/            # RequireToken 授权链路
|   |   |-- vkey/            # 网关密钥校验与凭据管理
|   |   |-- http/            # HTTP 路由与处理器
|   |   |-- sandbox/         # 记忆沙箱与上下文隔离
|   |   |-- audit/           # 审计日志与攻击链图谱
|   |   |-- control/         # 控制平面（策略/密钥）
|   |   `-- runtime/         # Agent 框架适配配置
|   `-- pkg/smcrypto/        # 国密算法封装（SM2/SM3/SM4）
|-- frontend/                # 前端演示页面
|-- experiments/
|   |-- asb/                 # 当前主实验入口：ASB runner 和结果转换器
|   |-- eval/                # 统一结果 schema 与指标统计
|   `-- aegisguard/          # 后续接入 AegisGuard 防护后的 ASB 实验记录
|-- go.mod
|-- package.json
`-- README.md
```

## 后端架构

### 核心特性

| 特性 | 说明 |
|:---|:---|
| **零侵入接入** | Agent 通过配置 `BASE_URL` 和网关密钥接入，无需代码改动 |
| **凭据收敛** | 统一网关密钥(agk-xxx)验证身份，LLM API Key 由网关托管，Agent 无感知 |
| **三层安全闸门** | Message Gate → Action Gate → Return Gate，覆盖执行全链路 |
| **可信授权链路** | RequireToken 机制，基于国密 SM2/SM3/SM4 |
| **双流上下文隔离** | Trusted Core Context / Sandbox Context，阻断外部污染 |
| **攻击链审计** | 时序因果 DAG，支持阻断节点定位与路径回溯 |

### 架构设计：凭据收敛模型

AegisGuard 面向**企业内部自部署**场景设计。管理员在 `gateway.yaml` 中设置网关密钥和 LLM API Key，所有 Agent 使用统一的 `agk-` 前缀网关密钥接入，LLM 凭据由网关完全托管。

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

### 核心工作流程

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

**认证流程（凭据替换）：**

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

## Benchmark 来源

本项目不把 ASB 源码复制进仓库，而是将 ASB 作为外部 checkout 使用。

ASB 官方仓库：

```text
https://github.com/agiresearch/ASB
```

推荐本地目录结构：

```text
F:\2026信安赛\AegisGuard
F:\2026信安赛\ASB
```

## 运行 ASB

在 AegisGuard 仓库根目录执行：

```powershell
python .\experiments\asb\run_asb.py --asb-root F:\2026信安赛\ASB --attack opi --run-id asb-opi-v1
```

当前支持的 `--attack` 参数：

- `dpi`：Direct Prompt Injection，直接提示词注入
- `opi`：Observation Prompt Injection，外部观察 / 工具输出注入
- `mp`：Memory Poisoning，记忆污染
- `mixed`：混合攻击
- `pot`：Plan-of-Thought Backdoor，规划后门

## 转换结果

ASB 运行结束后，将 ASB 输出转换为 AegisGuard 统一结果表：

```powershell
python .\experiments\asb\collect_results.py --input F:\2026信安赛\ASB\logs --attack opi --run-id asb-opi-v1
```

转换后的文件写入：

```text
experiments/asb/results/
experiments/asb/results/traces/
experiments/asb/results/manifests/
```

结果表按 ASB 原始指标输出：`ASR`、`ASR-d`、`RR`、`PNA`、`PNA-d`、`BP`、`FNR`、`FPR`。`latency_ms` 仅作为工程分析字段保留。

## 结果表述

如果实验通过 `experiments/asb/` 调用原始 ASB 脚本并转换输出，可以在报告中表述为：

```text
我们通过适配器在原始 ASB benchmark 上评测 AegisGuard，保留了 ASB 的任务、工具、攻击配置和输出记录。
```

不要把旧本地 pilot 数值和 ASB 结果混在同一张表中。新的实验表格应以 `experiments/asb/results/` 下的转换结果为准。

## LangGraph ASB 测试路线

对于后续 `LangGraph` 智能体，推荐直接走 `ASB-native` 路线，而不是 adapter-based 路线。原因是 `LangGraph` 更容易固定 workflow、状态流转和工具调用，和 ASB 的 agent lifecycle 更匹配，结果也更适合放进统一 benchmark 主表。

当前仓库中已经补好了最小接入样例：

- `ASB/pyopenagi/agents/langgraph_agent.py`：LangGraph 原生 agent 基类
- `ASB/pyopenagi/agents/example/langgraph_financial_agent/`：最小金融分析 agent 示例
- `ASB/data/agent_task_langgraph_smoke.jsonl`：LangGraph smoke task
- `ASB/data/attack_tools_langgraph_smoke.jsonl`：LangGraph smoke attacker tool
- `ASB/config/DPI_langgraph_smoke.yml`：最小 smoke 配置

推荐按下面三个阶段推进测试。

### 第一阶段：最小 smoke

目标是先验证 `LangGraph agent` 能否被 ASB 原生调起，并正常完成一次工具调用和一次最终响应。

建议范围：

- 只跑 `DPI`
- 只跑 `1` 个 agent
- 只跑 `1` 个 task
- 只跑 `1` 个 attacker tool
- 只跑 `1` 个 attack type（建议先用 `fake_completion`）

推荐命令：

```powershell
$env:PYTHONIOENCODING='utf-8'
$env:PYTHONUTF8='1'
cd .\ASB
..\.venv-asb-openclaw\Scripts\python.exe .\main_attacker.py --agent_backend pyopenagi --llm_name gpt-4o-mini --attack_type fake_completion --attacker_tools_path data/attack_tools_langgraph_smoke.jsonl --tasks_path data/agent_task_langgraph_smoke.jsonl --tools_info_path data/all_normal_tools.jsonl --direct_prompt_injection --task_num 1 --res_file logs/langgraph_smoke/fake_completion.csv
```

成功标准：

- agent 能被 ASB 正常创建
- 能生成 workflow 或进入 fallback 执行
- 能至少调用一次 normal tool
- 结果文件成功写入 `ASB/logs/langgraph_smoke/`

### 第二阶段：3-case smoke

当单 case 跑通后，再把最小 smoke 扩成 `3` 个 case，用于验证 `LangGraph` 在最基础的 prompt injection 变体下是否稳定。

建议固定三种 attack type：

- `naive`
- `fake_completion`
- `escape_characters`

这一阶段的目标不是追求统计显著性，而是确认：

- workflow 是否稳定
- tool selection 是否稳定
- agent 是否会被明显提示注入带偏
- 原始任务成功率是否保持可接受水平

### 第三阶段：DPI 小样本与正式评测

3-case smoke 稳定之后，再扩成 `DPI` 小样本，然后再决定是否进入完整实验。

推荐顺序：

1. `DPI` 小样本
2. `OPI` 小样本
3. `MP` / `mixed` / `PoT`

建议实验设计：

- 每种 attack family 先跑小样本
- 每个 family 固定同一批 task 数
- 每个 agent 固定同一模型、同一工具集、同一 prompt 配置
- 结果统一写入 ASB 原生日志，再转换到统一 schema

这样后续多个 agent 才能真正做横向比较。

## OpenClaw ASB 测试路线

对于 `OpenClaw`，当前更推荐走 `adapter-based` 路线，而不是把它作为主结果表里的 `ASB-native` agent。原因是 `OpenClaw` 更依赖 CLI / gateway / session 运行链，和 ASB 默认的 pyopenagi agent lifecycle 不完全一致。当前仓库已经实现了可用的 OpenClaw 适配评测链，但在正式表述上应写成 `ASB-derived adapter-based evaluation`。

当前可直接使用的 OpenClaw 测试入口：

- `experiments/asb/openclaw/run_openclaw_cli.py`：OpenClaw CLI / gateway 适配器
- `experiments/asb/openclaw/judge_openclaw_raw.py`：OpenClaw raw 结果补标签
- `experiments/asb/collect_results.py`：统一指标汇总
- `experiments/asb/openclaw/asb_dpi_10_tasks.jsonl`：OpenClaw DPI 小样本任务

推荐按下面三个阶段推进。

### 第一阶段：OpenClaw 单条 smoke

目标是先确认：

- OpenClaw CLI 可用
- 模型 provider 可用
- 单条任务能正常返回
- raw CSV / trace 能成功写出

推荐命令：

```powershell
npm run openclaw:smoke
```

如果要手动运行：

```powershell
python .\experiments\asb\openclaw\run_openclaw_cli.py --message "Reply with exactly OK." --run-id openclaw-smoke-local --timeout 60 --fail-on-error
```

成功标准：

- 生成 `experiments/asb/results/openclaw-<run-id>-raw.csv`
- 生成 `experiments/asb/results/traces/` 下的 trace
- `stdout` 有可解析响应

### 第二阶段：OpenClaw 3-case 或小样本 smoke

单条 smoke 通过后，再扩到 `DPI` 小样本。建议先不要上 full benchmark，而是先跑少量 case，确认 OpenClaw 在 ASB 派生任务上的基本稳定性。

推荐命令：

```powershell
python .\experiments\asb\openclaw\run_openclaw_cli.py --input-jsonl .\experiments\asb\openclaw\asb_dpi_10_tasks.jsonl --max-cases 3 --timeout 180 --run-id openclaw-dpi-smoke --attack dpi
```

跑完后补标签：

```powershell
python .\experiments\asb\openclaw\judge_openclaw_raw.py --input .\experiments\asb\results\openclaw-openclaw-dpi-smoke-raw.csv
```

再汇总统一指标：

```powershell
python .\experiments\asb\collect_results.py --input .\experiments\asb\results\openclaw-openclaw-dpi-smoke-raw.csv --attack dpi --run-id openclaw-dpi-smoke --defense none --agent-name OpenClaw --agent-version npm-2026.4.20-beta.1 --output-prefix openclaw-dpi-smoke
```

### 第三阶段：OpenClaw 扩展测试

当 `DPI` 小样本稳定后，再逐步扩到：

1. `OPI` 小样本
2. `MP` 小样本
3. `mixed`
4. `PoT`

不建议一开始直接跑 full tasks。更合理的顺序是先确认：

- CLI / gateway 是否稳定
- timeout 是否合理
- heuristic judge 是否能正确打标签
- `ASR / RR / PNA / latency` 是否具备基本可读性

再决定是否进入完整批量测试。

## 多 agent 统一评测建议

如果后续要同时测试多个 agent，建议统一采用下面的分层策略：

- `ASB-native`：用于 `LangGraph` 这类可稳定接入 ASB agent lifecycle 的 agent
- `adapter-based`：用于当前还不适合原生接入的 agent，例如 `OpenClaw` 这类 CLI/gateway 型 agent

推荐报告写法：

- 主结果表：只放 `ASB-native` agent
- 补充结果表：放 `adapter-based` agent
- 明确标注 `integration_mode`

不要把 `ASB-native` 和 `adapter-based` 结果直接写成"完全同口径"。更稳妥的表述是：

- `ASB-native evaluation`
- `ASB-derived adapter-based evaluation`

推荐落表方式：

- 主结果表：`LangGraph` 等 `ASB-native` agent
- 补充结果表：`OpenClaw` 等 `adapter-based` agent
- 在表中单独保留 `integration_mode`、`judge_type` 或 `label_scope`

这样可以保证：

- `LangGraph` 的结果适合做主 benchmark 对比
- `OpenClaw` 的结果仍可用于安全趋势分析和工程验证
- 报告中不会把两类结果误写成严格同口径

## LangGraph 环境建议

建议为 `LangGraph` 单独保留一组 provider 环境变量，避免和其他实验链路相互影响。当前 `LangGraph agent` 支持以下优先级：

1. `LANGGRAPH_OPENAI_*`
2. `CUSTOM_*`
3. `OPENAI_*`
4. `OFOXAI_*`

推荐至少配置：

- `LANGGRAPH_OPENAI_API_KEY`
- `LANGGRAPH_OPENAI_BASE_URL`
- `LANGGRAPH_OPENAI_MODEL`

这样可以避免 `LangGraph` smoke 与 `OpenClaw` 或其他实验共享同一组 provider 配置时互相干扰。

## 后端快速开始

### 环境要求

- Go 1.25+
- （可选）Docker，用于容器化部署

### 配置网关凭据

```bash
# 复制模板文件（gateway.yaml 已被 .gitignore 忽略，不会提交到仓库）
cp config/gateway.yaml.example config/gateway.yaml
```

编辑 `config/gateway.yaml`，填入你的真实凭据：

```yaml
gateway_key: agk-dev-001
target_url: https://api.openai.com
llm_api_key: sk-your-real-llm-api-key-here
```

> ⚠️ `gateway.yaml` 包含真实 API Key，已在 `.gitignore` 中排除。生产环境建议通过环境变量 `AEGIS_TARGET_URL` 和 `AEGIS_LLM_API_KEY` 注入敏感字段，避免明文写入文件。

### 启动网关

```bash
cd backend
go run cmd/server/main.go
```

服务启动后监听 `:8090`（可通过 `PORT` 环境变量修改）。

### Agent 接入（零代码改动）

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

参考模板 `config/gateway.yaml.example`：

```yaml
# AegisGuard 网关凭据配置
gateway_key: agk-dev-001
target_url: https://api.openai.com
llm_api_key: sk-your-real-llm-api-key-here
```

> ⚠️ `gateway.yaml` 已在 `.gitignore` 中，不会提交到仓库。首次使用请从模板复制：`cp config/gateway.yaml.example config/gateway.yaml`

- `gateway_key`：Agent 侧配置的统一网关密钥，必须以 `agk-` 开头
- `target_url`：真实的 LLM API 地址，网关将请求转发到此地址
- `llm_api_key`：网关托管的真实 LLM API Key，仅存在于网关内存中
- 生产环境可通过 `AEGIS_TARGET_URL` 和 `AEGIS_LLM_API_KEY` 环境变量覆盖对应字段

## 网关凭据管理

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

### 凭据轮换

当真实 LLM API Key 需要更新时：

1. 编辑 `config/gateway.yaml`，修改 `llm_api_key` 字段（参考 `gateway.yaml.example` 模板）
2. 重启网关
3. **Agent 侧无需任何修改**，`agk-xxx` 保持不变

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
llm_api_key: "sk-xxxxxxxxxxxxxxxxxxxx"
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

在 `internal/runtime/config.go` 添加框架识别逻辑：

```go
func DetectFramework(req *http.Request) FrameworkType {
    // 根据请求特征识别 LangGraph / AutoGen / OpenHands 等
    // 返回对应的安全策略配置
}
```

## 测试验证

### 本地快速验证

```bash
# 1. 从模板复制配置文件
#    cp config/gateway.yaml.example config/gateway.yaml
# 2. 编辑 config/gateway.yaml，设置网关密钥和真实 LLM Key
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

## 前端访问 langgraph_financial_agent 与查看运行日志

当前主控制台左侧功能框中已经新增 `langgraph_financial_agent` 模块。该模块通过 AegisGuard Go 后端代理访问 LangGraph 金融智能体服务：

```text
浏览器前端 -> AegisGuard Go 后端 -> LangGraph chat_server.py -> langgraph_financial_agent
```

如果希望在终端实时看到后端代理过程和 agent 运行过程，不要使用后台隐藏启动方式，建议打开两个 PowerShell 终端分别前台启动。

### 1. 启动 LangGraph 金融智能体服务

在第一个 PowerShell 终端执行：

```powershell
.\.venv-asb-openclaw\Scripts\python.exe .\experiments\asb\langgraph\chat_server.py --port 8765
```

启动成功后会看到类似输出：

```text
[2026-04-28 16:23:50] [langgraph_financial_agent] chat server running at http://127.0.0.1:8765/chat
```

当前 `chat_server.py` 会打印：

- 收到 `/api/chat` 请求
- agent 开始运行时使用的模型和输入摘要
- agent 运行耗时、消息数、thinking 数、tool/action 数
- workflow 摘要
- tool/action 摘要
- 异常信息

### 2. 启动 AegisGuard 主控制台后端

在第二个 PowerShell 终端执行：

```powershell
$env:GOCACHE=(Join-Path (Get-Location) '.gocache')
$env:PORT='18080'
go run .\backend\cmd\server
```

然后访问：

```text
http://localhost:18080
```

进入系统后，点击左侧功能框中的：

```text
langgraph_financial_agent
```

即可进入金融智能体聊天界面。

Go 后端会打印代理日志，例如：

```text
[langgraph-proxy] POST /api/langgraph-financial/chat -> http://127.0.0.1:8765/api/chat body_bytes=...
[langgraph-proxy] completed status=200 response_bytes=... elapsed=...
```

### 3. 端口占用处理

如果启动 Go 后端时出现：

```text
listen tcp :18080: bind: Only one usage of each socket address (protocol/network address/port) is normally permitted.
```

说明 `18080` 已经被其他进程占用。最简单的处理方式是换一个端口，例如：

```powershell
$env:GOCACHE=(Join-Path (Get-Location) '.gocache')
$env:PORT='18081'
go run .\backend\cmd\server
```

然后浏览器访问：

```text
http://localhost:18081
```

如果必须使用 `18080`，可以先查找占用端口的进程：

```powershell
netstat -ano | findstr :18080
```

找到 `LISTENING` 行最后一列的 PID 后结束进程：

```powershell
taskkill /PID <进程号> /F
```

再重新启动：

```powershell
$env:GOCACHE=(Join-Path (Get-Location) '.gocache')
$env:PORT='18080'
go run .\backend\cmd\server
```

### 4. 为什么有时终端看不到 agent 过程

如果使用 `Start-Process -WindowStyle Hidden` 或后台启动脚本，日志通常会被重定向到 `.tmp/*.log`，不会显示在当前终端中。要实时观察过程，请使用上面的两个前台命令启动。

另外，AegisGuard Go 后端只是代理请求；真正的 agent workflow、tool/action 过程由 `experiments/asb/langgraph/chat_server.py` 调用 `langgraph_financial_agent` 时打印。因此需要同时观察两个终端：

- Go 后端终端：查看前端请求是否到达、是否成功转发、耗时和状态码。
- LangGraph 终端：查看 agent 是否开始运行、生成 workflow、调用工具以及最终返回。

### 5. 服务地址配置

默认情况下，Go 后端会代理到：

```text
http://127.0.0.1:8765
```

如果 LangGraph chat server 使用了其他地址，可以通过环境变量覆盖：

```powershell
$env:LANGGRAPH_CHAT_URL='http://127.0.0.1:8766'
$env:GOCACHE=(Join-Path (Get-Location) '.gocache')
$env:PORT='18080'
go run .\backend\cmd\server
```

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

## 许可证

本项目为竞赛作品，代码仅供学习研究使用。
