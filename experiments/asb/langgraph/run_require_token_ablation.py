import argparse
import csv
import os
import subprocess
import sys
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
        "require_token": "0",
    },
    "full": {
        "label": "Full AegisGuard",
        "defense": "aegisguard_gate",
        "message": "1",
        "action": "1",
        "return": "1",
        "require_token": "1",
    },
    "wo_require_token": {
        "label": "w/o RequireToken",
        "defense": "aegisguard_gate",
        "message": "1",
        "action": "1",
        "return": "1",
        "require_token": "0",
    },
    "require_token_only": {
        "label": "RequireToken Only",
        "defense": "aegisguard_gate",
        "message": "0",
        "action": "0",
        "return": "0",
        "require_token": "1",
    },
    "wo_action_with_require_token": {
        "label": "w/o Action Gate + RequireToken",
        "defense": "aegisguard_gate",
        "message": "1",
        "action": "0",
        "return": "1",
        "require_token": "1",
    },
    "action_only": {
        "label": "Action Gate Only",
        "defense": "aegisguard_gate",
        "message": "0",
        "action": "1",
        "return": "0",
        "require_token": "0",
    },
}


def run_one(args: argparse.Namespace, name: str, config: dict[str, str]) -> int:
    env = os.environ.copy()
    env["LANGGRAPH_OPENAI_MODEL"] = args.model
    env["AEGISGUARD_ENABLE_MESSAGE_GATE"] = config["message"]
    env["AEGISGUARD_ENABLE_ACTION_GATE"] = config["action"]
    env["AEGISGUARD_ENABLE_RETURN_GATE"] = config["return"]
    env["AEGISGUARD_ENABLE_REQUIRE_TOKEN"] = config["require_token"]
    env["AEGISGUARD_REQUIRE_TOKEN_MODE"] = args.require_token_mode

    run_id = f"{args.run_prefix}-{args.model}-{name}"
    cmd = [
        str(args.python),
        str(RUN_BATCH),
        "--attack-family",
        "dpi",
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
    ]
    if config["defense"]:
        cmd.extend(["--defense-type", config["defense"]])

    print(f"[require-token-ablation] running {name}: {config['label']}", flush=True)
    return subprocess.run(cmd, cwd=REPO_ROOT, env=env).returncode


def read_all_row(path: Path) -> dict[str, str] | None:
    if not path.exists():
        return None
    with path.open("r", newline="", encoding="utf-8", errors="replace") as handle:
        for row in csv.DictReader(handle):
            if row.get("attack_type") == "ALL":
                return row
    return None


def write_markdown_table(csv_path: Path) -> Path:
    md_path = csv_path.with_suffix(".md")
    with csv_path.open("r", newline="", encoding="utf-8") as handle:
        rows = list(csv.DictReader(handle))
    headers = [
        "Ablation",
        "Msg",
        "Act",
        "Ret",
        "Token",
        "Cases",
        "ASR",
        "Task Completion",
        "Refusal",
        "Input Tokens",
        "Output Tokens",
        "Total Tokens",
        "Avg Tokens",
        "Time(s)",
    ]
    lines = [
        "| " + " | ".join(headers) + " |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for row in rows:
        values = [
            row["ablation"],
            row["message_gate"],
            row["action_gate"],
            row["return_gate"],
            row["require_token"],
            row["cases"],
            row["attack_success_rate"],
            row["original_task_success_rate"],
            row["refusal_rate"],
            row["input_tokens"],
            row["output_tokens"],
            row["total_tokens"],
            row["avg_total_tokens"],
            row["total_time_seconds"],
        ]
        lines.append("| " + " | ".join(values) + " |")
    md_path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return md_path


def write_ablation_table(args: argparse.Namespace, selected: list[str], returncodes: dict[str, int]) -> Path:
    out_dir = REPO_ROOT / "ASB" / "logs" / "langgraph_batch"
    table_path = out_dir / f"{args.run_prefix}-{args.model}-require-token-ablation-table.csv"
    fieldnames = [
        "model",
        "ablation",
        "defense",
        "message_gate",
        "action_gate",
        "return_gate",
        "require_token",
        "cases",
        "attack_success_rate",
        "original_task_success_rate",
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
            row = read_all_row(out_dir / f"{run_id}-main-table.csv")
            if row is None:
                continue
            writer.writerow(
                {
                    "model": args.model,
                    "ablation": config["label"],
                    "defense": config["defense"] or "none",
                    "message_gate": config["message"],
                    "action_gate": config["action"],
                    "return_gate": config["return"],
                    "require_token": config["require_token"],
                    "cases": row.get("cases", ""),
                    "attack_success_rate": row.get("attack_success_rate", ""),
                    "original_task_success_rate": row.get("original_task_success_rate", ""),
                    "refusal_rate": row.get("refusal_rate", ""),
                    "input_tokens": row.get("input_tokens", ""),
                    "output_tokens": row.get("output_tokens", ""),
                    "total_tokens": row.get("total_tokens", ""),
                    "avg_total_tokens": row.get("avg_total_tokens", ""),
                    "total_time_seconds": row.get("duration_seconds", ""),
                    "returncode": returncodes.get(name, ""),
                    "run_id": run_id,
                }
            )
    write_markdown_table(table_path)
    return table_path


def main() -> int:
    parser = argparse.ArgumentParser(description="Run LangGraph RequireToken ablation experiments.")
    parser.add_argument("--python", type=Path, default=REPO_ROOT / ".venv-asb-openclaw" / "Scripts" / "python.exe")
    parser.add_argument("--model", default="gpt-4o-mini")
    parser.add_argument("--kernel-llm-name", default="gpt-4o-mini")
    parser.add_argument("--run-prefix", default="langgraph-dpi-75c-require-token-ablation")
    parser.add_argument("--tasks-path", default="data/agent_task_langgraph_finance_5.jsonl")
    parser.add_argument("--attacker-tools-path", default="data/attack_tools_langgraph_finance_5.jsonl")
    parser.add_argument("--task-num", type=int, default=5)
    parser.add_argument("--timeout", type=int, default=1200)
    parser.add_argument("--tool-chunk-size", type=int, default=1)
    parser.add_argument("--require-token-mode", choices=["strict", "observe"], default="strict")
    parser.add_argument("--ablation", action="append", choices=sorted(ABLATIONS))
    args = parser.parse_args()

    selected = args.ablation or list(ABLATIONS)
    returncodes = {name: run_one(args, name, ABLATIONS[name]) for name in selected}
    table_path = write_ablation_table(args, selected, returncodes)
    print(f"[require-token-ablation] table: {table_path}", flush=True)
    return 0 if all(code == 0 for code in returncodes.values()) else 1


if __name__ == "__main__":
    sys.exit(main())
