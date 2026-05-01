# Three-Gate Terminal Demo

Run the real AegisGuard three-gate demo:

```powershell
npm run gate:demo
```

or:

```powershell
python .\experiments\aegisguard\run_three_gate_demo.py
```

The demo is interactive. Example inputs:

```text
正常投资风险分析
提示注入
导出客户明细
调仓审批
工具污染返回
敏感信息泄露
中文攻击样例
```

Each run executes the real `RuleBasedGatePolicy` hard-rule gate first, then calls
the configured OpenAI-compatible LLM as an auxiliary semantic judge. The final
decision is produced by deterministic priority fusion:

```text
quarantine > deny > human_approval > degrade > allow
```

The latest JSON trace is written to:

```text
experiments/aegisguard/results/three_gate_demo_last.json
```

For local debugging without LLM calls:

```powershell
python .\experiments\aegisguard\run_three_gate_demo.py --case 正常投资风险分析 --no-llm
```
