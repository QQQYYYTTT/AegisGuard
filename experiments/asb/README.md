# ASB Experiment Layout

`experiments/asb/` contains AegisGuard's experiment wrappers around ASB. It should not be treated as the upstream ASB source tree. The upstream-style benchmark code and agent implementations live under `ASB/`.

## Directory Map

```text
experiments/asb/
|-- README.md
|-- run_asb.py                 # Generic launcher for original ASB scripts
|-- collect_results.py         # Convert ASB/OpenClaw raw outputs to AegisGuard schema
|-- run_langgraph_batch.py     # Compatibility wrapper; delegates to langgraph/run_batch.py
|-- langgraph/
|   |-- run_batch.py           # ASB-native LangGraph batch runner
|   `-- README.md
|-- openclaw/
|   |-- run_openclaw_cli.py    # OpenClaw CLI/Gateway adapter
|   |-- judge_openclaw_raw.py  # Post-label OpenClaw raw outputs
|   |-- build_asb_tasks.py     # Build OpenClaw task JSONL from ASB-like cases
|   |-- *.jsonl                # OpenClaw adapter task sets
|   `-- README.md
`-- results/
    |-- *.csv / *.json         # Adapter-based OpenClaw normalized outputs
    |-- manifests/
    `-- traces/
```

## Two Evaluation Paths

### 1. LangGraph, ASB-Native

Use this path for agents implemented inside ASB's `pyopenagi` lifecycle.

Main entrypoint:

```text
experiments/asb/langgraph/run_batch.py
```

ASB-side implementation:

```text
ASB/pyopenagi/agents/langgraph_agent.py
ASB/pyopenagi/agents/example/langgraph_financial_agent/
ASB/data/agent_task_langgraph_*.jsonl
ASB/data/attack_tools_langgraph_*.jsonl
```

Default outputs:

```text
ASB/logs/langgraph_batch/
```

Report this as an ASB-native LangGraph evaluation because `ASB/main_attacker.py` creates and runs the tested agent.

### 2. OpenClaw, Adapter-Based

Use this path for OpenClaw because it is primarily invoked through CLI/Gateway rather than ASB's native `AgentFactory`.

Main entrypoint:

```text
experiments/asb/openclaw/run_openclaw_cli.py
```

Default outputs:

```text
experiments/asb/results/
```

Report this as an ASB-derived OpenClaw adapter evaluation unless a future native ASB wrapper is added.

## Generic ASB Launcher

`run_asb.py` is a thin wrapper around original ASB scripts:

```powershell
python .\experiments\asb\run_asb.py --asb-root .\ASB --attack dpi --run-id asb-dpi-v1
```

Supported attack families:

- `dpi`
- `opi`
- `mp`
- `mixed`
- `pot`

## Result Conversion

`collect_results.py` converts raw ASB/OpenClaw outputs into the shared AegisGuard result schema:

```powershell
python .\experiments\asb\collect_results.py --input .\experiments\asb\results --attack dpi --run-id openclaw-dpi-v1
```

LangGraph batch already writes its own main table and case table under `ASB/logs/langgraph_batch/`, so it usually does not need `collect_results.py`.

## Current Naming Rule

- `ASB/data/agent_task_langgraph_*`: ASB-native LangGraph task sets
- `ASB/data/attack_tools_langgraph_*`: ASB-native LangGraph attacker sets
- `experiments/asb/openclaw/*.jsonl`: OpenClaw adapter task sets
- `ASB/logs/langgraph_batch/*`: LangGraph native batch outputs
- `experiments/asb/results/openclaw-*`: OpenClaw adapter outputs
