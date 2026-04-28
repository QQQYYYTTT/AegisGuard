# experiments

本目录只保留当前 ASB-first 实验路线所需内容。

```text
experiments/
|-- asb/         # ASB 运行入口、结果转换器、manifest、trace 和结果表
|-- eval/        # 统一结果 schema 与指标统计
`-- aegisguard/  # 后续接入 AegisGuard 防护后的 ASB 实验记录
```

## 当前原则

- 新实验统一基于原始 ASB benchmark。
- ASB 源码不复制进本仓库，保持为外部 checkout。
- AegisGuard 只负责调用 ASB、记录运行 manifest、转换结果并生成统一统计。
- 新结果统一写入 `experiments/asb/results/`。

## 已移除内容

旧的本地原生 Agent 实验和 guardrail 对照实验已经删除，包括：

- 本地 LangChain runner
- 本地攻击 fixtures
- 旧 CSV / JSON 结果
- 旧 trace
- 旧图表
- 旧实验说明文档

这样可以避免把本地 pilot 结果误认为原始 ASB benchmark 结果。
