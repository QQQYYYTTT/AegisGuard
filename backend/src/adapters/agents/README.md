# Agent Adapters

这里预留给后续主流 Agent 的实验接入层。

- `openhands/`: OpenHands 原生安全机制测试与调用封装
- `dbgpt/`: DB-GPT 查询链路与权限边界测试
- `openclaw/`: 工具调用型 Agent 的工具审批与 trust model 测试
- `langchain/`: 框架默认安全能力与自定义 guardrail 测试

建议每个适配器后续统一提供：

- `getCapabilities()`
- `getNativeSecurityProfile()`
- `runBaselineTask()`
- `runAttackCase()`
- `collectAuditEvidence()`
