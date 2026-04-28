# OpenClaw ASB Adapter

This directory contains the adapter-based OpenClaw evaluation path.

OpenClaw is not currently implemented as an ASB-native `pyopenagi` agent in this repo. Instead, AegisGuard invokes OpenClaw through its CLI or Gateway and records ASB-derived outputs.

## When To Use This Path

Use `experiments/asb/openclaw/` when the tested agent is OpenClaw and the experiment should treat it as a black-box CLI/Gateway agent.

Use `experiments/asb/langgraph/` instead when the tested agent is implemented inside ASB's native agent lifecycle.

## Files

```text
experiments/asb/openclaw/
|-- run_openclaw_cli.py       # Invoke OpenClaw and write raw CSV/traces
|-- judge_openclaw_raw.py     # Add labels to raw OpenClaw outputs
|-- build_asb_tasks.py        # Build OpenClaw JSONL tasks from ASB-like cases
|-- sample_tasks.jsonl        # Minimal manual smoke input
|-- asb_dpi_10_tasks.jsonl    # Small DPI sample
|-- asb_*_full_tasks.jsonl    # Larger ASB-derived task sets
`-- README.md
```

## Output Location

OpenClaw adapter outputs are written under:

```text
experiments/asb/results/
experiments/asb/results/traces/
```

This differs from the LangGraph-native path, whose default outputs are under:

```text
ASB/logs/langgraph_batch/
```

## Quick Smoke

The repository pins OpenClaw through `package.json`. Run:

```powershell
npm install
npm run openclaw:smoke
```

Equivalent direct command:

```powershell
python .\experiments\asb\openclaw\run_openclaw_cli.py `
  --message "Reply with exactly OK." `
  --run-id openclaw-smoke-local `
  --timeout 60 `
  --fail-on-error
```

## Batch Example

```powershell
python .\experiments\asb\openclaw\run_openclaw_cli.py `
  --input-jsonl .\experiments\asb\openclaw\asb_dpi_10_tasks.jsonl `
  --max-cases 3 `
  --timeout 180 `
  --run-id openclaw-dpi-smoke `
  --attack dpi
```

Then label or collect results as needed:

```powershell
python .\experiments\asb\openclaw\judge_openclaw_raw.py `
  --input .\experiments\asb\results\openclaw-openclaw-dpi-smoke-raw.csv

python .\experiments\asb\collect_results.py `
  --input .\experiments\asb\results\openclaw-openclaw-dpi-smoke-raw.csv `
  --attack dpi `
  --run-id openclaw-dpi-smoke `
  --defense none `
  --agent-name OpenClaw `
  --agent-version npm-2026.4.20-beta.1 `
  --output-prefix openclaw-dpi-smoke
```

## Reporting Boundary

Phrase OpenClaw results as:

```text
ASB-derived OpenClaw adapter evaluation
```

Do not describe them as fully ASB-native unless a future wrapper is added under `ASB/pyopenagi/agents/` and OpenClaw is launched by `ASB/main_attacker.py` through ASB's normal `AgentFactory`.
