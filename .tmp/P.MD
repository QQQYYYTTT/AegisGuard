# AegisGuard 最终改进计划

> 说明：本文件是对 `.tmp/P2.MD` 和 `.tmp/P3.MD` 的核查、去重和重排结果。只保留与 **Agent 安全** 直接相关、且能从当前 `backend/` 实现中验证出来的真阳性改进项。

---

## 一、核查结论

### 确认真阳性，保留

1. **SchemaHash 防线当前基本未真正生效**
   - `RequireToken` 里有 `SchemaHash` 字段，但签发、校验、Action Gate 强制比对链路没有闭合。
   - 这是标准的 tool poisoning / rug pull 防御缺口。

2. **RequireToken 绑定不完整**
   - `ExpiresAt` 未进入签名消息。
   - verifier 缓存路径会弱化过期/签名校验的严格性。
   - `SessionID` / `TaskID` 已签名，但未与当前执行上下文做强制一致性校验。
   - 这是授权重放、跨任务复用、令牌篡改链路中的真缺口。

3. **Action Gate 还不是“真实工具执行前”的强制控制点**
   - 当前更像从 LLM 请求体中识别潜在 tool call。
   - 如果 agent 本地 executor 直接执行业务工具，后端不一定能强制拦截。
   - 这是当前后端最大的 agent-security 盲区之一。

4. **Nonce 防重放状态是内存态且无明确容量防护**
   - 有可用性风险，也有重启后状态丢失带来的重放窗口问题。

5. **参数溯源覆盖面过窄**
   - 目前只覆盖极少量工具和参数。
   - 对“间接注入最终污染高危参数”的防护解释还不够成立。

6. **Return Gate 对编码/非结构化载体覆盖不足**
   - 压缩响应、HTML、非 JSON 文本、不可解析二进制都存在盲区。

7. **Memory write 还没有被提升为显式受控动作**
   - 现在更像“返回后隔离”，还不是“长期记忆写入前强制审核”。

8. **多 Agent / MCP / 委派链最小权限模型缺失**
   - 这是事实上的边界缺口，但考虑时间，应后置。

### 部分真阳性，但不进入核心安全开发优先级

1. **增加更贴近真实攻击链的实验与演示**
   - 这对证明有效性很重要，但属于验证层，不是第一优先级的防御实现。

### 剔除，不纳入最终安全计划

1. `AGENT_INTEGRATION.md`
   - 有价值，但本质是集成文档，不是安全防护项。

2. 清理 `scripts/` 死代码
   - 影响仓库整洁度，不直接提升 agent 安全。

---

## 二、排序原则

采用以下排序规则：

1. **先修“安全边界是否真实存在”**
   - 也就是先解决“系统是不是强制执行点”的问题。

2. **再修“授权链是否可信”**
   - 如果授权令牌本身不严，后面的越权防护说服力不足。

3. **最后补“检测覆盖面”**
   - regex、溯源、返回净化属于第三层收益。

4. **在同等安全收益下，优先改动小、依赖少、提交前可落地的项**

---

## 三、最终改进计划

## P0：提交前必须优先完成

### P0-1. 打通并强制执行 SchemaHash 校验链----已完成

**目标**

让 `SchemaHash` 从“概念字段”变成真正的 tool integrity 控制。

**需要改动**

1. `backend/internal/auth/token.go`
   - 签发 token 时写入 `SchemaHash`
2. `backend/internal/auth/verifier.go`
   - 明确 schema 校验责任边界
3. `backend/internal/gates/action.go`
   - 在 `ActionGate.Evaluate()` 中强制对实际 tool schema 调用 `CompareSchemaHash()`
4. 新增最小工具注册表
   - `toolName -> schema`
   - 先覆盖实验里最常用和最高风险的工具

**为什么排第 1**

1. 改动集中，收益极高
2. 直接补上 tool poisoning / rug pull 的核心缺口
3. 答辩时最容易展示“防御链闭合”

---

### P0-2. 修复 RequireToken 绑定缺口----已完成

**目标**

让授权令牌真正具备“不可篡改、不可跨任务复用、不可弱校验缓存绕过”的可信性。

**需要改动**

1. `backend/internal/auth/token.go`
   - `buildSignMessage()` 纳入 `ExpiresAt`
   - 明确 `CallCount` 不能由客户端回传决定
2. `backend/internal/auth/verifier.go`
   - 重新设计缓存 key，基于完整签名消息和签名值
   - 缓存命中不允许绕过过期检查
   - 明确签名有效缓存和 nonce/预算动态检查的边界
3. `backend/internal/gates/action.go`
   - 强制校验 token 的 `SessionID` / `TaskID` 与当前执行上下文一致
4. `backend/internal/gateway/proxy.go`
   - 为当前请求注入可校验的 session/task 元数据

**为什么排第 2**

1. 这是越权、防重放、防跨任务复用的根基
2. 和 P0-1 依赖关系强，应该一起做

---

### P0-3. 让 Action Gate 成为更多“低入侵可代理链路”的强制控制点----已完成（最小 HTTP Proxy 版本）

> 状态更新：在剩余 2 天约束下，这一项需要**重定义**，不能再按“完整 runtime 覆盖”推进，而应改成“最小 Runtime Adapter / Proxy 补盲”。

**目标**

把系统从“仅 LLM Gateway 防护”推进到“Gateway + Bridge/Proxy 增强防护”。

**需要改动**

1. 在现有 HTTP 网关上增加**统一业务代理入口**
   - 不新增大量零散端口
   - 复用同一业务端口，按路径分流
2. 只做“低入侵原则下真正可落地”的代理链路
   - `/mcp/proxy`：代理 **HTTP 型 MCP Server**
   - `/api/proxy`：代理 **OpenAPI / REST / 传统 HTTP RPC 工具**
   - 可选：`/a2a/proxy`：代理 **A2A 远程 Agent endpoint**
   - 可选：`stdio bridge`：桥接 **MCP STDIO**
3. 所有接入该入口的调用执行前统一经过：
   - token 校验
   - scope 校验
   - schema hash 校验
   - human approval / deny 决策
4. 这两天内**不追求**数据库透明代理、Memory 全链路拦截、同进程本地函数 Hook

**为什么排第 3**

1. 这是最大安全收益项
2. 讨论文件已经明确：纯 Gateway 的理论上限有限，必须补一个新的观测点
3. 但剩余时间不足以做完整 SDK，因此应收缩成 **Gateway + Bridge/Proxy** 的最小版本

**最终收缩方案**

1. 不做“完整 Runtime SDK”
2. 不做“数据库透明代理”
3. 不做“所有本地函数调用 Hook”
4. 不做“通用 shell/codeact 本地宿主拦截”
5. 只做：
   - 一个 **HTTP 型 MCP Proxy**
   - 一个 **OpenAPI / REST 工具 Proxy**
   - 若时间允许，再加一个 **STDIO MCP Bridge**
6. 论文和答辩统一表述为：
   - `Gateway` 是默认接入模式
   - `Bridge / Proxy` 是增强模式
   - 当前版本优先覆盖：
     - LLM 请求/响应
     - HTTP MCP
     - OpenAPI / REST / HTTP RPC 工具
     - 可桥接的 STDIO MCP

**本轮已完成内容**

1. 复用现有业务端口，无新增端口
2. 新增 `/mcp/proxy` 路由
3. 新增 `/api/proxy` 路由
4. HTTP 工具调用转发前进入 `ActionGate`
5. HTTP 工具回执返回前进入 `ReturnGate`
6. 与既有 `RequireToken`、`SchemaHash`、`SessionID/TaskID` 绑定链路打通

**本轮未完成内容**

1. `STDIO MCP Bridge` ----已完成（最小 bridge-stdio 版本）
2. 数据库透明代理
3. 同进程 Function Calling / 框架内部 dispatch / Shell-CodeAct 宿主内直调拦截

**`STDIO MCP Bridge` 建议实现形态（单 exe，多模式）**

1. 整个后端仍可只交付一个 `aegisguard.exe`
2. 但运行形态应区分为两个子命令，而不是强行塞进一个常驻进程里同时做所有事：
   - `aegisguard.exe server`
   - `aegisguard.exe bridge-stdio --backend http://127.0.0.1:PORT -- <real mcp command...>`
3. `server` 模式负责：
   - Web 后端 API
   - LLM gateway
   - `/api/proxy`
   - `/mcp/proxy`
4. `bridge-stdio` 模式负责：
   - 占用当前 MCP 会话的 `stdin/stdout`
   - 拉起真实 MCP STDIO server 子进程
   - 透明转发 agent 与真实 server 间的 JSON-RPC 消息
   - 在 `tools/list`、`tools/call`、可选 `resources/read` / `prompts/get` 上调用 `server` 模式的安全判定能力
5. 这仍满足“只交付一个 exe”，同时不额外增加新的长期监听端口
6. 实际运行时通常是：
   - 1 个 `aegisguard.exe server`
   - N 个 `aegisguard.exe bridge-stdio`
7. 这属于“一个二进制，多进程实例”，不是“一个常驻进程直接接管所有 stdio 会话”

**为什么不建议把 bridge 硬塞进单常驻进程**

1. Web 服务是监听 socket 的服务模型
2. STDIO bridge 是独占单次会话 `stdin/stdout` 的管道模型
3. 一个常驻服务进程天然不适合直接承载多个外部 agent 的 stdio 会话
4. 每个 bridge 会话独立成子进程，隔离性、调试性和稳定性都更好

**单 exe 目录与模块建议**

1. `cmd/aegisguard/main.go`
   - CLI 入口
   - 解析 `server` / `bridge-stdio` 子命令
2. `internal/http`
   - 保持现有 Web 后端、gateway、HTTP proxy
3. `internal/mcpbridge`
   - `bridge.go`：bridge 主流程
   - `framing.go`：STDIO 消息 framing 解析与编码
   - `proxy.go`：JSON-RPC 双向中继
   - `inspect.go`：识别 `initialize` / `tools/list` / `tools/call`
   - `backend_client.go`：回调本机 `server` 的 HTTP 接口
   - `schema_registry.go`：缓存 `toolName -> schemaHash`

**前后端分离部署说明**

1. 这里的“单 exe”仅指 `backend` 可独立交付为一个 `aegisguard.exe`
2. `frontend` 仍保持独立 Vue 工程，独立构建、独立部署
3. `server` 模式不负责承载前端静态资源
4. 因此这里不采用 `go:embed frontend/dist` 的一体化打包方式

**bridge-stdio 的最小数据流**

1. Agent 启动命令原本是：
   - `real-mcp-server ...`
2. 接入后改为：
   - `aegisguard.exe bridge-stdio --backend http://127.0.0.1:PORT -- real-mcp-server ...`
3. bridge 启动真实 MCP server 子进程
4. Agent 发来的 MCP 请求先进入 bridge
5. bridge 识别消息类型：
   - `tools/list`
   - `tools/call`
   - 可选：`resources/read`
   - 可选：`prompts/get`
6. `tools/list`
   - 提取 tool schema
   - 计算并缓存 `SchemaHash`
7. `tools/call`
   - bridge 组装 `tool_name / args / agent_id / session_id / task_id / schema_hash`
   - 调 backend 做 `ActionGate` 判定
   - 允许后才把调用转发给真实 MCP server
8. 真实 server 返回结果后：
   - bridge 调 backend 做 `ReturnGate` 判定
   - 再决定透传 / 降级 / 阻断

**端口与部署约束**

1. 对外长期监听端口仍建议保持为 1 个
2. `server` 模式复用现有业务端口
3. `/api/proxy`、`/mcp/proxy` 继续复用该端口
4. `bridge-stdio` 不开新端口，只主动请求本机 `server`

**协议实现注意点**

1. `STDIO MCP` 不能只假设一种消息 framing
2. bridge 至少应兼容两类常见实现：
   - `Content-Length` 风格 framing
   - 按行分隔的 JSON / NDJSON
3. 否则会出现“理论支持 STDIO MCP，但只能代理部分实现”的兼容性问题

**建议开发方式（如何落地实现）**

1. 先做“透明中继”，不要一上来重写完整 MCP 协议栈
   - 第一阶段只保证：
     - 能拉起真实 MCP server
     - 能双向透传消息
     - 能识别 `tools/list` / `tools/call`
2. bridge 内部分成 4 层，避免后面性能和逻辑耦合失控
   - `transport 层`
     - 负责 `stdin/stdout` 与子进程 `stdin/stdout` 的 framing 解析
   - `rpc inspect 层`
     - 负责识别 JSON-RPC request / response、`id`、`method`、`params`
   - `security hook 层`
     - 只在命中 `tools/list` / `tools/call` / 可选 `resources/read` 时触发安全逻辑
   - `backend client 层`
     - 负责把判定请求发给现有 backend
3. 不要让 bridge 自己实现完整策略引擎
   - 策略、token、审计、schema 校验都尽量复用现有 backend
   - bridge 只做：
     - 观测
     - 最小缓存
     - 转发前阻断 / 转发后阻断
4. `tools/list` 和 `tools/call` 要拆开实现
   - `tools/list` 负责建立 `toolName -> schemaHash`
   - `tools/call` 负责真正执行前的安全判定
   - 不要在 `tools/call` 时临时现算全部 schema，避免每次重复开销
5. 先只支持最小安全闭环
   - `initialize`：透传
   - `tools/list`：提取 schema + 建立缓存
   - `tools/call`：ActionGate + ReturnGate
   - 其他 method 默认透传并打审计标记
6. CLI 设计要保持低侵入
   - 保持用户只改启动命令，不改 agent 源码
   - 建议参数形态：
     - `aegisguard.exe bridge-stdio --backend http://127.0.0.1:PORT --agent-id xxx --session-id yyy -- <real command...>`
7. Windows 进程管理要单独处理
   - 正确传递退出码
   - 父进程退出时及时清理真实 MCP server 子进程
   - stderr 默认透传或镜像到日志，避免调试困难

**如何实现“既能防御，又不明显降低延迟/性能”**

1. 判定只挂在高价值节点，不做全量深度解析
   - 必拦：`tools/list`、`tools/call`
   - 可后置：`resources/read`、`prompts/get`
   - 不要对每一条普通通知、心跳、初始化细节都跑完整安全链
2. `SchemaHash` 只在 schema 变化时重算
   - `tools/list` 后缓存 `toolName -> schemaHash`
   - 后续 `tools/call` 直接读缓存
   - 避免每次调用重新序列化整个 schema
3. token 与上下文绑定信息要按会话缓存
   - 可按 `(agent_id, session_id, task_id, tool_name)` 做短 TTL 缓存
   - 避免同一会话内每个调用都重新构造完整授权上下文
4. backend 判定接口要做“轻量快速路径”
   - 常规 allow 路径只走：
     - token 校验
     - scope 校验
     - schema hash 比对
     - 规则命中判定
   - 不要在常规路径同步做重型分析
5. 返回内容检查采用“大小分层”
   - 小响应：同步全检
   - 大响应：限制检查窗口或做分段检查
   - 流式响应：优先保留 action 前拦截，return 侧只做轻量策略或打标
6. bridge 内尽量零拷贝或少拷贝
   - framing 解析后直接复用字节切片
   - 只在命中安全检查时做 JSON 反序列化
   - 非目标 method 尽量原样透传
7. 审计写入不要阻塞主转发路径
   - 审计异步化
   - 高风险阻断结果可同步落一条最小记录
   - 详细日志异步补写
8. 子进程和 backend HTTP 连接都要复用
   - backend client 使用长连接
   - 统一超时，避免 bridge 被慢判定拖死
9. 给判定链加硬超时
   - 例如 action 判定超时上限 100~300ms
   - 超时后的默认策略要明确：
     - 高危工具默认拒绝
     - 低危工具可按 fail-open / fail-close 配置
10. 演示版不要过早引入数据库、全文检测、复杂持久化
   - 这会明显增加尾延迟和实现风险
   - 提交前版本优先保证：
     - 可拦截
     - 可审计
     - 可解释
     - 延迟可控

**建议性能目标**

1. 常规 `tools/call` 额外延迟目标：
   - 本机 backend 场景尽量控制在 `5~20ms`
2. 命中复杂规则或需要降级过滤时：
   - 额外延迟尽量控制在 `20~80ms`
3. 不把流式大响应的深度检查放进本轮最小版本承诺
4. 论文表述应写成：
   - 当前版本优先在高风险工具调用点施加最小必要检测，而非对所有 runtime 数据做全量同步扫描

**两天版最小可交付范围**

1. 先只做 `bridge-stdio`
2. 先只拦：
   - `tools/list`
   - `tools/call`
3. `resources/read` / `prompts/get` 若时间不足可后置
4. 先只支持最常见的本地文件型、Shell 型、HTTP 请求型 MCP 工具演示链
5. 论文与答辩中明确表述：
   - HTTP 工具链通过 `/api/proxy`、`/mcp/proxy` 代理
   - 本地 STDIO MCP 工具链通过 `bridge-stdio` 包装命令代理
   - 二者都属于“低入侵增强模式”

**低入侵边界必须写清**

在不改 Agent 源码、不要求用户重新打包闭源 Agent 的前提下，当前作品后续可扩展覆盖的链路应限定为：

1. **可改 endpoint 的链路**
   - OpenAI-compatible LLM
   - HTTP MCP
   - OpenAPI / REST / HTTP RPC
   - A2A
   - 数据库连接串（若未来做 TCP proxy）
2. **可改启动命令的链路**
   - MCP STDIO（通过 bridge 包装命令）
3. **天然不在低入侵范围内的链路**
   - 同进程 Function Calling
   - 框架内部 Tool dispatch
   - 宿主直接 `subprocess` / `os.system` 的 Shell / CodeAct

这三类天然盲区即使后续版本也不能轻易宣称“默认可防”。

---

## P0.5：必须同步调整的论文与产品表述

**目标**

避免继续把系统描述成“覆盖所有 Agent Runtime Attack”。

**必须改的表述**

把“全面运行时防护”收缩成：

> 本系统当前主要覆盖基于 LLM 推理链路的数据流攻击，以及显式经由 OpenAI-compatible 网关、HTTP MCP Proxy、OpenAPI/REST Proxy、可桥接的 STDIO MCP 等链路暴露的工具调用安全；对于未接入 AegisGuard 的 Agent 内部状态（如同进程函数调用、框架内部工具分发、本地 Shell/CodeAct、内存对象、Planner 内部状态）相关攻击，当前版本未完全覆盖。

**为什么必须加**

因为 `.tmp/taolun.md` 的核心判断是对的：你们最大的问题不是“检测规则不够”，而是“观测点是否存在”。这必须体现在最终表述里。

---

## P1：高收益补强

### P1-1. [x] 为 Nonce 增加容量控制和更稳妥的状态管理

**目标**

降低 nonce map 的内存风险和重启后重放窗口。

**需要改动**

1. 给 `usedNonces` 加容量上限
2. 增加淘汰策略
3. 若时间允许，再接 SQLite 持久化

**原因**

这项同时提升安全性和运行稳定性，但不如前三项关键。

---

### P1-2. [x] 扩展高危参数溯源

**目标**

把 provenance 从“概念演示”提升到“对高危参数有实际约束”。

**优先覆盖字段**

1. shell：`command`, `script`, `path`
2. 文件：`path`, `target`, `filename`
3. 数据库：`query`, `table`, `record_id`
4. 网络：`url`, `host`, `recipient`

**来源约束**

1. 只能来自用户原始输入
2. 只能来自指定工具返回
3. 只能来自系统可信配置

---

### P1-3. [x] 扩展 Return Gate 对真实载体的覆盖

**目标**

缩小 OPI、reverse injection、external result pollution 的盲区。

**需要改动**

1. 对 `gzip/deflate` 先解压再评估
2. 对 `text/html` 做基础文本抽取
3. 非 JSON 文本统一进入 sanitize + sandbox
4. 无法解析的二进制默认隔离并打标，不直接当安全结果透传

---

### P1-4. [x] 把 memory write 提升为显式受控动作

**目标**

把“记忆投毒防护”从规则匹配升级为流程控制。

**需要改动**

1. 定义显式 memory 工具
   - `memory.write`
   - `memory.update`
   - `memory.promote`
2. 这些动作统一走 Action Gate
3. 只有 `safe_summary` 能进入 trusted memory
4. 审计记录 memory source 和 promotion reason

**最小闭环拆分**

1. `P1-4a`：补 memory action 数据结构与审计字段
   - 在 sandbox transfer record 中补齐：
     - `tool_name`
     - `memory_source`
     - `promotion_reason`
   - 不重做整套 memory schema，只在现有 `TransferRecord` 上最小扩展
2. `P1-4b`：把 trusted memory 的写入收敛到显式 `memory.promote`
   - 保留现有 sandbox isolate 入口做兼容
   - 停止 Return Gate / sandbox 内部隐式自动晋升 trusted memory
   - trusted memory 只允许通过显式 promote 路径进入
3. `P1-4c`：在后端增加最小 memory action 执行入口
   - 显式支持：
     - `memory.write`
     - `memory.update`
     - `memory.promote`
   - 旧的 `/aegis/sandbox/isolate` 内部兼容映射到：
     - 首次写入 -> `memory.write`
     - 已有上下文更新 -> `memory.update`
     - `promote=true` -> 追加一次 `memory.promote`
4. `P1-4d`：memory action 统一走 Action Gate
   - 由后端内部签发最小 `RequireToken`
   - 保持与现有 `strict` token mode 兼容
   - `memory.promote` 额外要求：
     - `memory_source == safe_summary`
     - `promotion_reason` 非空
5. `P1-4e`：补最小测试闭环
   - 高风险 memory poisoning 不得进入 trusted memory
   - 低风险 `safe_summary` promote 成功
   - transfer/audit 记录可见 `tool_name`、`memory_source`、`promotion_reason`

**本轮执行范围**

1. 只做后端最小闭环与必要前端展示字段兼容
2. 不做完整 memory subsystem 重构
3. 不把通用本地内存对象、同进程 planner state 一并纳入
4. 不扩大到未经过 AegisGuard 的内部 runtime 盲区

**本轮已完成**

1. 新增显式 memory action 语义：
   - `memory.write`
   - `memory.update`
   - `memory.promote`
2. 新增最小 memory action 执行入口：
   - `POST /aegis/sandbox/memory`
3. 旧的 `POST /aegis/sandbox/isolate` 已兼容映射到显式动作：
   - 首次写入走 `memory.write`
   - 带 `context_id` 更新走 `memory.update`
   - `promote=true` 追加一次 `memory.promote`
4. memory action 现已统一进入 `Action Gate`
   - 后端内部签发最小 `RequireToken`
   - 与现有 `strict` token mode 兼容
5. `memory.promote` 增加显式约束：
   - 仅允许 `memory_source == safe_summary`
   - 强制要求 `promotion_reason`
6. trusted memory 已不再从 Return Gate / sandbox 内部隐式自动晋升
   - proxy 返回链路仅写入 sandbox candidate
   - 进入 trusted memory 只能通过显式 `memory.promote`
7. transfer 审计已补齐结构化字段：
   - `tool_name`
   - `memory_source`
   - `promotion_reason`
8. 前端 sandbox transfer 展示已兼容上述新字段
9. 对应单元测试已补齐并通过：
   - 高风险 memory poisoning 不进入 trusted memory
   - 低风险 `safe_summary` promote 成功
   - 非 `safe_summary` 的 direct promote 被拒绝

---

## P2：时间充裕再做

### P2-1. [x] 最小多 Agent / MCP 边界控制

**目标**

避免作品在“多 Agent / 外部 MCP Server”问题上完全裸露。

**最小实现**

1. 委派请求强制带调用方身份
2. 低权限 agent 不允许请求高风险工具
3. 明确未接入 AegisGuard 的 MCP / sub-agent 默认不受保护
4. 区分：
   - `HTTP MCP`：可低入侵代理
   - `STDIO MCP`：需 bridge
   - `同进程 dispatch`：默认不可代理

**本轮已完成**

1. `bridge-stdio` / `HTTP MCP proxy` 强制携带 `caller_agent_id`
2. `ActionGate` 新增最小边界校验：
   - 缺少调用方身份时拒绝边界请求
   - `caller_agent_id != agent_id` 的 delegated 请求默认按低权限 agent 处理
   - delegated agent 请求高风险工具时直接拒绝
3. 明确当前受保护边界：
   - `HTTP MCP`：`/mcp/proxy`
   - `STDIO MCP`：`bridge-stdio -> /aegis/bridge/evaluate/action`
4. 未接入以上代理链路的外部 MCP / sub-agent / 同进程 dispatch 仍默认不受保护

---

### P2-2. 增加安全验证用例

**建议新增 4 个最值钱的演示**

1. 间接注入诱导 `shell.exec`，被 Action Gate 拒绝
2. 工具 schema 被篡改，因 schema hash mismatch 被拒绝
3. memory poisoning 只能进入 sandbox，不能直接晋升 trusted memory
4. 旧 token / 过期 token / 跨 task token 被 verifier 拒绝

---

## 四、建议执行顺序

### 第 1 阶段：已完成

1. `SchemaHash` 链路闭合
2. `RequireToken` 绑定修复

### 第 2 阶段：剩余 2 天内必须做

1. 做一个最小 `Bridge / Proxy`【已完成（HTTP Proxy 路由版）】
2. 优先实现：
   - `/mcp/proxy`【已完成】
   - `/api/proxy`【已完成】
   - 若时间允许，再补 `stdio bridge`
3. 让它们执行前走 `ActionGate`
4. 补一个演示攻击链：
   - 间接注入
   - 诱导 HTTP MCP / REST 工具
   - 被 Proxy + ActionGate 拦截

### 第 3 阶段：若还有余力再做

1. Nonce 容量控制
2. provenance 扩展
3. Return Gate 载体扩展中的 `gzip/html` 最小支持

### 第 4 阶段：这次提交不再强求

1. memory write 受控化
2. 多 Agent / MCP 最小边界
3. 演示与验证样例补齐

---

## 五、最核心的一句话

如果时间只够做一条主线，就做：

**“SchemaHash 强制校验 + RequireToken 绑定修复 + 一个最小 Bridge/Proxy（HTTP MCP / OpenAPI/REST / 可选 STDIO MCP）接入 Action Gate”**

这是当前最能把 AegisGuard 从“LLM gateway 防护”推进到“Gateway + Bridge/Proxy 增强防护”的组合，也是剩余 2 天内最现实、最有答辩价值的路线。

---

## 六、两天版明确取舍

### 这两天建议继续做

1. 最小 `Bridge / Proxy`
2. 一个 HTTP 型 MCP 代理入口
3. 一个 OpenAPI / REST 工具代理入口
4. 可选一个 STDIO MCP bridge【已完成（最小 bridge-stdio 版本）】
5. 一条完整演示链和答辩话术

### 这两天不建议继续做

1. 完整 Memory Guard
2. 完整 Planner / Plan-of-Thought 防护
3. 数据库透明 TCP 代理
4. Function Calling / 框架内部 dispatch 通用拦截
5. Shell / CodeAct 宿主内通用拦截
6. 全面多 Agent 权限体系

### 理由

`.tmp/taolun.md` 已经把边界讲清楚了：

1. 纯 Gateway 永远看不到所有 runtime 内部状态
2. 真正值得补的不是“更多规则”，而是“增加新的可代理观测点”
3. 在 2 天时间里，最合理的新观测点不是完整 SDK，而是最小 Bridge / Proxy
4. 在低入侵原则下，可新增覆盖的重点是：
   - HTTP MCP
   - OpenAPI / REST / HTTP RPC
   - 可桥接的 STDIO MCP
5. 在低入侵原则下，以下仍是天然盲区：
   - 同进程 Function Calling
   - 框架内部 Tool dispatch
   - 宿主直接发起的 Shell / CodeAct
