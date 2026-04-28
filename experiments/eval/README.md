# 统一评估格式

`experiments/eval/` 保存 AegisGuard 实验结果的统一 schema 和指标统计逻辑。当前主要服务于 ASB 输出转换。

## 当前流程

```text
原始 ASB 输出 -> experiments/asb/collect_results.py -> ExperimentRecord CSV
```

## 主要字段

- `run_id`：实验编号
- `repeat_index`：重复次数编号
- `timestamp_utc`：记录时间
- `agent_name`、`agent_version`：被测 agent 信息
- `defense`：ASB 配置或防御名称
- `asb_attack`：ASB 攻击类型，例如 `dpi`、`opi`、`mp`、`mixed`、`pot`
- `case_id`：样例编号
- `scenario`：场景或变体名称
- `benchmark_family`：benchmark 名称，当前为 `ASB`
- `benchmark_suite`：ASB suite / attack 类型
- `under_attack`：是否处于攻击条件
- `defense_enabled`：是否开启防御
- `attack_success`：攻击是否成功
- `refused`：是否拒绝危险任务
- `task_success`：任务是否成功
- `benign_success`：良性任务是否成功
- `poison_detected`：污染样本是否被检测到
- `clean_detected_as_poison`：干净样本是否被误判为污染
- `latency_ms`：延迟
- `asr`、`asr_d`、`rr`、`pna`、`pna_d`、`bp`、`fnr`、`fpr`：ASB 指标字段
- `trace_path`：对应 trace 文件
- `raw_source_path`：原始 ASB 输出文件路径
- `judge_method`：判定方式
- `evaluator_version`：转换器 / 评估版本
- `notes`：备注

## ASB 指标

当前结果输出按 ASB 原始指标组织：

- `ASR`：Attack Success Rate，攻击成功率
- `ASR-d`：Attack Success Rate under Defense，防御开启后的攻击成功率
- `RR`：Refuse Rate，拒绝率
- `PNA`：Performance under No Attack，无攻击条件下的任务表现
- `PNA-d`：PNA under Defense，防御开启后的无攻击任务表现
- `BP`：Benign Performance，PoT 等场景下无触发器时的良性表现
- `FNR`：False Negative Rate，漏检率
- `FPR`：False Positive Rate，误报率

`latency_ms` 会继续保留，方便工程分析，但它不是 ASB 核心指标之一。

## 注意事项

ASB-backed 结果必须来自原始 ASB 仓库和 `experiments/asb/` 适配器。不要把其他来源的 pilot 结果混入 ASB 表格。
