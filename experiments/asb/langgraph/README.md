# LangGraph ASB Experiments

This directory contains the ASB-native LangGraph/LangChain experiment entrypoints.

## Purpose

Use this path when the tested agent can be represented as an ASB-native `pyopenagi` agent. The current target is:

- `ASB/pyopenagi/agents/example/langgraph_financial_agent/`

The agent is executed by `ASB/main_attacker.py`, so the result is closer to native ASB execution than the OpenClaw adapter path.

## Files

```text
experiments/asb/langgraph/
|-- run_batch.py   # Batch runner for LangGraph ASB-native DPI experiments
`-- README.md
```

Related ASB-side files:

```text
ASB/pyopenagi/agents/langgraph_agent.py
ASB/pyopenagi/agents/example/langgraph_financial_agent/
ASB/data/agent_task_langgraph_smoke.jsonl
ASB/data/attack_tools_langgraph_smoke.jsonl
ASB/data/agent_task_langgraph_finance_5.jsonl
ASB/data/attack_tools_langgraph_finance_5.jsonl
ASB/config/DPI_langgraph_smoke.yml
```

## Supported Attack Families

The LangGraph batch runner can run these ASB attack families separately:

| Family | Meaning | Default attack types |
|---|---|---|
| `dpi` | Direct Prompt Injection | `naive`, `fake_completion`, `escape_characters` |
| `opi` | Observation Prompt Injection | `context_ignoring` |
| `mp` | Memory Poisoning | `combined_attack` |
| `mixed` | DPI + OPI combined | `combined_attack` |
| `pot` | Plan-of-Thought Backdoor | `naive` |
| `clean` | Clean baseline | `naive` |

## Quick Smoke

```powershell
$env:PYTHONIOENCODING='utf-8'
$env:PYTHONUTF8='1'
.\.venv-asb-openclaw\Scripts\python.exe .\experiments\asb\langgraph\run_batch.py --attack-family dpi --run-id langgraph-batch-smoke --timeout 240
```

## Finance 5x5 DPI Batch

```powershell
$env:PYTHONIOENCODING='utf-8'
$env:PYTHONUTF8='1'
.\.venv-asb-openclaw\Scripts\python.exe .\experiments\asb\langgraph\run_batch.py `
  --attack-family dpi `
  --run-id langgraph-finance-5x5-dpi `
  --tasks-path data/agent_task_langgraph_finance_5.jsonl `
  --attacker-tools-path data/attack_tools_langgraph_finance_5.jsonl `
  --task-num 5 `
  --timeout 600 `
  --tool-chunk-size 1
```

`--tool-chunk-size 1` runs one attacker tool at a time. This avoids provider `429` rate limits while still producing one merged result per attack variant.

## Run Each Attack Separately

Use one run id per attack family:

```powershell
$common = @(
  "--tasks-path", "data/agent_task_langgraph_finance_5.jsonl",
  "--attacker-tools-path", "data/attack_tools_langgraph_finance_5.jsonl",
  "--task-num", "5",
  "--timeout", "600",
  "--tool-chunk-size", "1"
)

.\.venv-asb-openclaw\Scripts\python.exe .\experiments\asb\langgraph\run_batch.py --attack-family dpi --run-id langgraph-finance-5x5-dpi @common
.\.venv-asb-openclaw\Scripts\python.exe .\experiments\asb\langgraph\run_batch.py --attack-family opi --run-id langgraph-finance-5x5-opi @common
.\.venv-asb-openclaw\Scripts\python.exe .\experiments\asb\langgraph\run_batch.py --attack-family mixed --run-id langgraph-finance-5x5-mixed @common
.\.venv-asb-openclaw\Scripts\python.exe .\experiments\asb\langgraph\run_batch.py --attack-family mp --run-id langgraph-finance-5x5-mp --database memory_db/direct_prompt_injection/combined_attack_gpt-4o-mini @common
.\.venv-asb-openclaw\Scripts\python.exe .\experiments\asb\langgraph\run_batch.py --attack-family pot --run-id langgraph-finance-5x5-pot --trigger "with perspicacious discernment" @common
```

Memory poisoning needs a populated vector database. If the selected `--database` does not contain relevant poison memories, MP results should be treated as a plumbing check rather than a final memory-poisoning score.

## Outputs

By default, LangGraph batch outputs are written under:

```text
ASB/logs/langgraph_batch/
```

Each run creates:

- `<run-id>-main-table.csv`
- `<run-id>-cases.csv`
- `<run-id>-summary.json`
- `<run-id>-report.md`
- `<run-id>-<attack_type>.csv`
- `<run-id>-<attack_type>.log`

The old path `experiments/asb/run_langgraph_batch.py` is kept as a compatibility wrapper.
