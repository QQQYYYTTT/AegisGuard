import argparse
import csv
import os
import subprocess
import sys
from datetime import datetime
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[3]
RUN_BATCH = REPO_ROOT / "experiments" / "asb" / "langgraph" / "run_batch.py"


ABLATIONS = {
    "no_defense": {
        "label": "No Defense",
        "defense": "",
        "message": "0",
        "action": "0",
        "return": "0",
        "sandbox": "0",
    },
    "full": {
        "label": "Full AegisGuard",
        "defense": "aegisguard_gate",
        "message": "1",
        "action": "1",
        "return": "1",
        "sandbox": "1",
    },
    "wo_memory_sandbox": {
        "label": "w/o Memory Sandbox",
        "defense": "aegisguard_gate",
        "message": "1",
        "action": "1",
        "return": "1",
        "sandbox": "0",
    },
    "memory_sandbox_only": {
        "label": "Memory Sandbox Only",
        "defense": "aegisguard_gate",
        "message": "0",
        "action": "0",
        "return": "0",
        "sandbox": "1",
    },
}


ATTACK_TYPES = ["naive", "fake_completion", "escape_characters"]


def output_dir() -> Path:
    return REPO_ROOT / "ASB" / "logs" / "langgraph_batch"


def run_command(cmd: list[str], env: dict[str, str]) -> int:
    print(" ".join(str(part) for part in cmd), flush=True)
    return subprocess.run(cmd, cwd=REPO_ROOT, env=env).returncode


def base_env(args: argparse.Namespace, config: dict[str, str] | None = None) -> dict[str, str]:
    env = os.environ.copy()
    env["LANGGRAPH_OPENAI_MODEL"] = args.model
    env["AEGISGUARD_ENABLE_MESSAGE_GATE"] = (config or {}).get("message", "0")
    env["AEGISGUARD_ENABLE_ACTION_GATE"] = (config or {}).get("action", "0")
    env["AEGISGUARD_ENABLE_RETURN_GATE"] = (config or {}).get("return", "0")
    env["AEGISGUARD_ENABLE_MEMORY_SANDBOX"] = (config or {}).get("sandbox", "0")
    return env


def run_batch_command(
    args: argparse.Namespace,
    attack_family: str,
    run_id: str,
    database: str,
    env: dict[str, str],
    defense_type: str = "",
    write_db: bool = False,
    read_db: bool = False,
) -> int:
    cmd = [
        str(args.python),
        str(RUN_BATCH),
        "--attack-family",
        attack_family,
        "--attack-types",
        *ATTACK_TYPES,
        "--run-id",
        run_id,
        "--llm-name",
        args.model,
        "--kernel-llm-name",
        args.kernel_llm_name,
        "--tasks-path",
        args.tasks_path,
        "--attacker-tools-path",
        args.attacker_tools_path,
        "--task-num",
        str(args.task_num),
        "--timeout",
        str(args.timeout),
        "--tool-chunk-size",
        str(args.tool_chunk_size),
        "--database",
        database,
    ]
    if defense_type:
        cmd.extend(["--defense-type", defense_type])
    if write_db:
        cmd.append("--write-db")
    if read_db:
        cmd.append("--read-db")
    return run_command(cmd, env)


def prepare_poisoned_memory(args: argparse.Namespace, database: str) -> int:
    run_id = f"{args.run_prefix}-{args.model}-prepare-poisoned-memory"
    env = base_env(args)
    print("[memory-ablation] preparing shared poisoned memory with DPI write_db", flush=True)
    return run_batch_command(
        args=args,
        attack_family="dpi",
        run_id=run_id,
        database=database,
        env=env,
        write_db=True,
    )


def run_read_ablation(args: argparse.Namespace, name: str, config: dict[str, str], database: str) -> int:
    run_id = f"{args.run_prefix}-{args.model}-{name}"
    env = base_env(args, config)
    print(f"[memory-ablation] running MP read: {config['label']}", flush=True)
    return run_batch_command(
        args=args,
        attack_family="mp",
        run_id=run_id,
        database=database,
        env=env,
        defense_type=config["defense"],
        read_db=True,
    )


def all_row(path: Path) -> dict[str, str] | None:
    if not path.exists():
        return None
    with path.open("r", newline="", encoding="utf-8", errors="replace") as handle:
        for row in csv.DictReader(handle):
            if row.get("attack_type") == "ALL":
                return row
    return None


def write_summary_table(args: argparse.Namespace, selected: list[str], returncodes: dict[str, int]) -> Path:
    table_path = output_dir() / f"{args.run_prefix}-{args.model}-memory-sandbox-ablation-table.csv"
    fieldnames = [
        "model",
        "ablation",
        "message_gate",
        "action_gate",
        "return_gate",
        "memory_sandbox",
        "cases",
        "attack_success_rate",
        "original_task_success_rate",
        "memory_found_rate",
        "refusal_rate",
        "input_tokens",
        "output_tokens",
        "total_tokens",
        "avg_total_tokens",
        "total_time_seconds",
        "returncode",
        "run_id",
    ]
    with table_path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=fieldnames)
        writer.writeheader()
        for name in selected:
            config = ABLATIONS[name]
            run_id = f"{args.run_prefix}-{args.model}-{name}"
            row = all_row(output_dir() / f"{run_id}-main-table.csv") or {}
            writer.writerow({
                "model": args.model,
                "ablation": config["label"],
                "message_gate": config["message"],
                "action_gate": config["action"],
                "return_gate": config["return"],
                "memory_sandbox": config["sandbox"],
                "cases": row.get("cases", ""),
                "attack_success_rate": row.get("attack_success_rate", ""),
                "original_task_success_rate": row.get("original_task_success_rate", ""),
                "memory_found_rate": row.get("memory_found_rate", ""),
                "refusal_rate": row.get("refusal_rate", ""),
                "input_tokens": row.get("input_tokens", ""),
                "output_tokens": row.get("output_tokens", ""),
                "total_tokens": row.get("total_tokens", ""),
                "avg_total_tokens": row.get("avg_total_tokens", ""),
                "total_time_seconds": row.get("duration_seconds", ""),
                "returncode": returncodes.get(name, ""),
                "run_id": run_id,
            })
    return table_path


def main() -> int:
    parser = argparse.ArgumentParser(description="Run LangGraph MP Memory Sandbox ablation experiments.")
    parser.add_argument("--python", type=Path, default=REPO_ROOT / ".venv-asb-openclaw" / "Scripts" / "python.exe")
    parser.add_argument("--model", default="gpt-4o-mini")
    parser.add_argument("--kernel-llm-name", default="gpt-4o-mini")
    parser.add_argument("--run-prefix", default=f"langgraph-mp-75c-memory-sandbox-ablation-{datetime.now().strftime('%Y%m%d-%H%M%S')}")
    parser.add_argument("--tasks-path", default="data/agent_task_langgraph_finance_5.jsonl")
    parser.add_argument("--attacker-tools-path", default="data/attack_tools_langgraph_finance_5.jsonl")
    parser.add_argument("--task-num", type=int, default=5)
    parser.add_argument("--timeout", type=int, default=1200)
    parser.add_argument("--tool-chunk-size", type=int, default=1)
    parser.add_argument("--database", default="")
    parser.add_argument("--skip-prepare", action="store_true")
    parser.add_argument("--ablation", action="append", choices=sorted(ABLATIONS))
    args = parser.parse_args()

    selected = args.ablation or list(ABLATIONS)
    database = args.database or f"memory_db/{args.run_prefix}-{args.model}-poisoned"

    returncodes: dict[str, int] = {}
    prepare_code = 0 if args.skip_prepare else prepare_poisoned_memory(args, database)
    for name in selected:
        returncodes[name] = run_read_ablation(args, name, ABLATIONS[name], database)

    table_path = write_summary_table(args, selected, returncodes)
    print(f"[memory-ablation] table: {table_path}", flush=True)
    return 0 if prepare_code == 0 and all(code == 0 for code in returncodes.values()) else 1


if __name__ == "__main__":
    sys.exit(main())
