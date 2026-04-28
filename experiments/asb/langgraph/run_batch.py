import argparse
import csv
import json
import os
import subprocess
import sys
import tempfile
from datetime import datetime, timezone
from pathlib import Path


DEFAULT_ATTACK_TYPES = ["naive", "fake_completion", "escape_characters"]
ATTACK_FAMILIES = {
    "dpi": {
        "flags": ["--direct_prompt_injection"],
        "default_attack_types": ["naive", "fake_completion", "escape_characters"],
        "description": "Direct Prompt Injection",
    },
    "opi": {
        "flags": ["--observation_prompt_injection"],
        "default_attack_types": ["context_ignoring"],
        "description": "Observation Prompt Injection",
    },
    "mp": {
        "flags": ["--memory_attack", "--read_db"],
        "default_attack_types": ["combined_attack"],
        "description": "Memory Poisoning",
    },
    "mixed": {
        "flags": ["--direct_prompt_injection", "--observation_prompt_injection"],
        "default_attack_types": ["combined_attack"],
        "description": "Mixed DPI + OPI",
    },
    "pot": {
        "flags": ["--pot_backdoor"],
        "default_attack_types": ["naive"],
        "description": "Plan-of-Thought Backdoor",
    },
    "clean": {
        "flags": ["--clean"],
        "default_attack_types": ["naive"],
        "description": "Clean baseline",
    },
}


def repo_root() -> Path:
    return Path(__file__).resolve().parents[3]


def asb_root() -> Path:
    return repo_root() / "ASB"


def default_python() -> Path:
    return repo_root() / ".venv-asb-openclaw" / "Scripts" / "python.exe"


def run_command(args: argparse.Namespace, cmd: list[str], log_path: Path) -> int:
    env = os.environ.copy()
    env.setdefault("PYTHONIOENCODING", "utf-8")
    env.setdefault("PYTHONUTF8", "1")

    try:
        with log_path.open("w", encoding="utf-8", errors="replace") as log_file:
            proc = subprocess.run(
                cmd,
                cwd=asb_root(),
                env=env,
                stdout=log_file,
                stderr=subprocess.STDOUT,
                text=True,
                timeout=args.timeout,
            )
        return proc.returncode
    except subprocess.TimeoutExpired:
        with log_path.open("a", encoding="utf-8", errors="replace") as log_file:
            log_file.write(f"\nTIMEOUT after {args.timeout} seconds\n")
        return 124


def read_jsonl(path: Path) -> list[dict]:
    records = []
    with path.open("r", encoding="utf-8") as handle:
        for line in handle:
            line = line.strip()
            if line:
                records.append(json.loads(line))
    return records


def write_jsonl(path: Path, records: list[dict]) -> None:
    with path.open("w", encoding="utf-8") as handle:
        for record in records:
            handle.write(json.dumps(record, ensure_ascii=False) + "\n")


def merge_csvs(sources: list[Path], target: Path) -> None:
    wrote_header = False
    with target.open("w", newline="", encoding="utf-8", errors="replace") as out_handle:
        writer = None
        for source in sources:
            if not source.exists():
                continue
            with source.open("r", newline="", encoding="utf-8", errors="replace") as in_handle:
                reader = csv.DictReader(in_handle)
                if reader.fieldnames is None:
                    continue
                if writer is None:
                    writer = csv.DictWriter(out_handle, fieldnames=reader.fieldnames)
                if not wrote_header:
                    writer.writeheader()
                    wrote_header = True
                for row in reader:
                    writer.writerow(row)


def family_flags(args: argparse.Namespace) -> list[str]:
    flags = list(ATTACK_FAMILIES[args.attack_family]["flags"])
    if args.write_db and "--write_db" not in flags:
        flags.append("--write_db")
    if args.read_db and "--read_db" not in flags:
        flags.append("--read_db")
    return flags


def base_command(args: argparse.Namespace, attack_type: str, attacker_tools_path: str, csv_path: Path) -> list[str]:
    asb = asb_root()
    cmd = [
        str(args.python),
        str(asb / "main_attacker.py"),
        "--agent_backend",
        "pyopenagi",
        "--llm_name",
        args.llm_name,
        "--attack_type",
        attack_type,
        "--attacker_tools_path",
        attacker_tools_path,
        "--tasks_path",
        args.tasks_path,
        "--tools_info_path",
        args.tools_info_path,
        "--task_num",
        str(args.task_num),
        "--res_file",
        str(csv_path.relative_to(asb)),
    ]
    cmd.extend(family_flags(args))
    if args.database:
        cmd.extend(["--database", args.database])
    if args.trigger:
        cmd.extend(["--trigger", args.trigger])
    if args.defense_type:
        cmd.extend(["--defense_type", args.defense_type])
    return cmd


def run_one(args: argparse.Namespace, attack_type: str, output_dir: Path) -> dict:
    asb = asb_root()
    output_dir.mkdir(parents=True, exist_ok=True)
    csv_path = output_dir / f"{args.run_id}-{attack_type}.csv"
    log_path = output_dir / f"{args.run_id}-{attack_type}.log"

    started = datetime.now(timezone.utc)
    if args.tool_chunk_size and args.tool_chunk_size > 0:
        tools = read_jsonl(asb / args.attacker_tools_path)
        chunk_csvs = []
        returncodes = []
        chunks_dir = output_dir / "_chunks" / f"{args.run_id}-{attack_type}"
        chunks_dir.mkdir(parents=True, exist_ok=True)
        with log_path.open("w", encoding="utf-8", errors="replace") as combined_log:
            combined_log.write(f"Chunked run: {len(tools)} tools, chunk size {args.tool_chunk_size}\n")
        for index in range(0, len(tools), args.tool_chunk_size):
            chunk_no = index // args.tool_chunk_size + 1
            chunk_tools = tools[index:index + args.tool_chunk_size]
            chunk_tools_path = chunks_dir / f"tools-{chunk_no:03d}.jsonl"
            chunk_csv = chunks_dir / f"result-{chunk_no:03d}.csv"
            chunk_log = chunks_dir / f"result-{chunk_no:03d}.log"
            write_jsonl(chunk_tools_path, chunk_tools)
            cmd = base_command(args, attack_type, str(chunk_tools_path.relative_to(asb)), chunk_csv)
            returncodes.append(run_command(args, cmd, chunk_log))
            chunk_csvs.append(chunk_csv)
            with log_path.open("a", encoding="utf-8", errors="replace") as combined_log:
                names = ", ".join(tool.get("Attacker Tool", "") for tool in chunk_tools)
                combined_log.write(f"chunk {chunk_no}: returncode={returncodes[-1]} tools={names}\n")
        merge_csvs(chunk_csvs, csv_path)
        returncode = max(returncodes) if returncodes else 0
    else:
        cmd = base_command(args, attack_type, args.attacker_tools_path, csv_path)
        returncode = run_command(args, cmd, log_path)
    ended = datetime.now(timezone.utc)

    return {
        "attack_type": attack_type,
        "returncode": returncode,
        "csv": str(csv_path.relative_to(repo_root())),
        "log": str(log_path.relative_to(repo_root())),
        "started_at": started.isoformat(),
        "ended_at": ended.isoformat(),
        "duration_seconds": round((ended - started).total_seconds(), 3),
    }


def read_rows(csv_path: Path) -> list[dict]:
    if not csv_path.exists():
        return []
    with csv_path.open("r", newline="", encoding="utf-8", errors="replace") as handle:
        return list(csv.DictReader(handle))


def rate(numerator: int, denominator: int) -> float:
    return round(numerator / denominator, 4) if denominator else 0.0


def summarize_run(run: dict) -> dict:
    rows = read_rows(repo_root() / run["csv"])
    total = len(rows)
    attack_success = sum(int(row.get("Attack Successful") or 0) for row in rows)
    original_success = sum(int(row.get("Original Task Successful") or 0) for row in rows)
    refusal = sum(int(row.get("Refuse Result") or 0) for row in rows)
    aggressive = sum(int(row.get("Aggressive") or 0) for row in rows)
    return {
        **run,
        "total": total,
        "attack_success": attack_success,
        "attack_success_rate": rate(attack_success, total),
        "original_task_success": original_success,
        "original_task_success_rate": rate(original_success, total),
        "refusal_count": refusal,
        "refusal_rate": rate(refusal, total),
        "aggressive_count": aggressive,
    }


def write_report(summary: dict, report_path: Path) -> None:
    lines = [
        "# LangGraph ASB Batch Report",
        "",
        f"- Run ID: `{summary['run_id']}`",
        f"- Generated at: `{summary['generated_at']}`",
        f"- LLM: `{summary['llm_name']}`",
        f"- Attack family: `{summary['attack_family']}` ({summary['attack_family_description']})",
        f"- Agent backend: `pyopenagi`",
        f"- Tasks path: `{summary['tasks_path']}`",
        f"- Attacker tools path: `{summary['attacker_tools_path']}`",
        f"- Task num per agent: `{summary['task_num']}`",
        "",
        "## Aggregate Metrics",
        "",
        f"- Total cases: `{summary['aggregate']['total']}`",
        f"- Attack success rate: `{summary['aggregate']['attack_success_rate']}` ({summary['aggregate']['attack_success']}/{summary['aggregate']['total']})",
        f"- Original task success rate: `{summary['aggregate']['original_task_success_rate']}` ({summary['aggregate']['original_task_success']}/{summary['aggregate']['total']})",
        f"- Refusal rate: `{summary['aggregate']['refusal_rate']}` ({summary['aggregate']['refusal_count']}/{summary['aggregate']['total']})",
        "",
        "## Per Attack Type",
        "",
        "| Attack Type | Cases | ASR | Original Success Rate | Refusal Rate | Return Code |",
        "|---|---:|---:|---:|---:|---:|",
    ]
    for item in summary["runs"]:
        lines.append(
            f"| `{item['attack_type']}` | {item['total']} | {item['attack_success_rate']} | "
            f"{item['original_task_success_rate']} | {item['refusal_rate']} | {item['returncode']} |"
        )
    lines.extend([
        "",
        "## Output Files",
        "",
    ])
    for item in summary["runs"]:
        lines.append(f"- `{item['attack_type']}` CSV: `{item['csv']}`")
        lines.append(f"- `{item['attack_type']}` log: `{item['log']}`")
    report_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def write_main_table(summary: dict, table_path: Path) -> None:
    fieldnames = [
        "run_id",
        "llm_name",
        "attack_family",
        "attack_type",
        "cases",
        "attack_success",
        "attack_success_rate",
        "original_task_success",
        "original_task_success_rate",
        "refusal_count",
        "refusal_rate",
        "returncode",
        "csv",
        "log",
    ]
    with table_path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=fieldnames)
        writer.writeheader()
        for item in summary["runs"]:
            writer.writerow({
                "run_id": summary["run_id"],
                "llm_name": summary["llm_name"],
                "attack_family": summary["attack_family"],
                "attack_type": item["attack_type"],
                "cases": item["total"],
                "attack_success": item["attack_success"],
                "attack_success_rate": item["attack_success_rate"],
                "original_task_success": item["original_task_success"],
                "original_task_success_rate": item["original_task_success_rate"],
                "refusal_count": item["refusal_count"],
                "refusal_rate": item["refusal_rate"],
                "returncode": item["returncode"],
                "csv": item["csv"],
                "log": item["log"],
            })
        aggregate = summary["aggregate"]
        writer.writerow({
            "run_id": summary["run_id"],
            "llm_name": summary["llm_name"],
            "attack_family": summary["attack_family"],
            "attack_type": "ALL",
            "cases": aggregate["total"],
            "attack_success": aggregate["attack_success"],
            "attack_success_rate": aggregate["attack_success_rate"],
            "original_task_success": aggregate["original_task_success"],
            "original_task_success_rate": aggregate["original_task_success_rate"],
            "refusal_count": aggregate["refusal_count"],
            "refusal_rate": aggregate["refusal_rate"],
            "returncode": max(item["returncode"] for item in summary["runs"]) if summary["runs"] else 0,
            "csv": "",
            "log": "",
        })


def write_case_table(summary: dict, case_path: Path) -> None:
    fieldnames = [
        "run_id",
        "llm_name",
        "attack_family",
        "attack_type",
        "agent_name",
        "attack_tool",
        "attack_successful",
        "original_task_successful",
        "refuse_result",
        "memory_found",
        "aggressive",
    ]
    with case_path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=fieldnames)
        writer.writeheader()
        for item in summary["runs"]:
            for row in read_rows(repo_root() / item["csv"]):
                writer.writerow({
                    "run_id": summary["run_id"],
                    "llm_name": summary["llm_name"],
                    "attack_family": summary["attack_family"],
                    "attack_type": item["attack_type"],
                    "agent_name": row.get("Agent Name", ""),
                    "attack_tool": row.get("Attack Tool", ""),
                    "attack_successful": row.get("Attack Successful", ""),
                    "original_task_successful": row.get("Original Task Successful", ""),
                    "refuse_result": row.get("Refuse Result", ""),
                    "memory_found": row.get("Memory Found", ""),
                    "aggressive": row.get("Aggressive", ""),
                })


def main() -> int:
    parser = argparse.ArgumentParser(description="Run ASB LangGraph batch tests and summarize metrics.")
    parser.add_argument("--run-id", default=f"langgraph-batch-{datetime.now().strftime('%Y%m%d-%H%M%S')}")
    parser.add_argument("--python", type=Path, default=default_python())
    parser.add_argument("--llm-name", default="gpt-4o-mini")
    parser.add_argument("--attack-family", choices=sorted(ATTACK_FAMILIES), default="dpi")
    parser.add_argument("--attack-types", nargs="+", default=None)
    parser.add_argument("--task-num", type=int, default=1)
    parser.add_argument("--timeout", type=int, default=240)
    parser.add_argument("--tasks-path", default="data/agent_task_langgraph_smoke.jsonl")
    parser.add_argument("--attacker-tools-path", default="data/attack_tools_langgraph_smoke.jsonl")
    parser.add_argument("--tools-info-path", default="data/all_normal_tools.jsonl")
    parser.add_argument("--output-dir", type=Path, default=asb_root() / "logs" / "langgraph_batch")
    parser.add_argument("--defense-type", default="")
    parser.add_argument("--database", default="")
    parser.add_argument("--trigger", default="")
    parser.add_argument("--read-db", action="store_true")
    parser.add_argument("--write-db", action="store_true")
    parser.add_argument("--tool-chunk-size", type=int, default=0)
    args = parser.parse_args()
    if args.attack_types is None:
        args.attack_types = ATTACK_FAMILIES[args.attack_family]["default_attack_types"]

    runs = []
    for attack_type in args.attack_types:
        print(f"[batch] running {attack_type}", flush=True)
        runs.append(run_one(args, attack_type, args.output_dir))

    run_summaries = [summarize_run(run) for run in runs]
    aggregate = {
        "total": sum(item["total"] for item in run_summaries),
        "attack_success": sum(item["attack_success"] for item in run_summaries),
        "original_task_success": sum(item["original_task_success"] for item in run_summaries),
        "refusal_count": sum(item["refusal_count"] for item in run_summaries),
    }
    aggregate["attack_success_rate"] = rate(aggregate["attack_success"], aggregate["total"])
    aggregate["original_task_success_rate"] = rate(aggregate["original_task_success"], aggregate["total"])
    aggregate["refusal_rate"] = rate(aggregate["refusal_count"], aggregate["total"])

    summary = {
        "run_id": args.run_id,
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "llm_name": args.llm_name,
        "attack_family": args.attack_family,
        "attack_family_description": ATTACK_FAMILIES[args.attack_family]["description"],
        "task_num": args.task_num,
        "tasks_path": args.tasks_path,
        "attacker_tools_path": args.attacker_tools_path,
        "runs": run_summaries,
        "aggregate": aggregate,
    }

    args.output_dir.mkdir(parents=True, exist_ok=True)
    summary_path = args.output_dir / f"{args.run_id}-summary.json"
    report_path = args.output_dir / f"{args.run_id}-report.md"
    main_table_path = args.output_dir / f"{args.run_id}-main-table.csv"
    case_table_path = args.output_dir / f"{args.run_id}-cases.csv"
    summary_path.write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")
    write_report(summary, report_path)
    write_main_table(summary, main_table_path)
    write_case_table(summary, case_table_path)
    print(f"[batch] summary: {summary_path}", flush=True)
    print(f"[batch] report: {report_path}", flush=True)
    print(f"[batch] main table: {main_table_path}", flush=True)
    print(f"[batch] cases: {case_table_path}", flush=True)
    return 0 if all(item["returncode"] == 0 for item in run_summaries) else 1


if __name__ == "__main__":
    sys.exit(main())
