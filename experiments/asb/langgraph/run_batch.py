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
        "default_attack_types": ["context_ignoring", "fake_completion", "combined_attack"],
        "description": "Observation Prompt Injection",
    },
    "mp": {
        "flags": ["--memory_attack", "--read_db"],
        "default_attack_types": ["naive", "fake_completion", "escape_characters"],
        "description": "Memory Poisoning",
    },
    "mixed": {
        "flags": ["--direct_prompt_injection", "--observation_prompt_injection"],
        "default_attack_types": ["naive", "fake_completion", "combined_attack"],
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
    fieldnames: list[str] = []
    rows: list[dict[str, str]] = []
    seen_fields = set()
    for source in sources:
        if not source.exists():
            continue
        with source.open("r", newline="", encoding="utf-8", errors="replace") as in_handle:
            reader = csv.DictReader(in_handle)
            if reader.fieldnames is None:
                continue
            for field in reader.fieldnames:
                if field not in seen_fields:
                    fieldnames.append(field)
                    seen_fields.add(field)
            rows.extend(reader)

    with target.open("w", newline="", encoding="utf-8", errors="replace") as out_handle:
        if not fieldnames:
            return
        writer = csv.DictWriter(out_handle, fieldnames=fieldnames, extrasaction="ignore")
        writer.writeheader()
        writer.writerows(rows)


def family_flags(args: argparse.Namespace) -> list[str]:
    flags = list(ATTACK_FAMILIES[args.attack_family]["flags"])
    if args.attack_family == "mp" and args.mp_write_only:
        flags = [flag for flag in flags if flag != "--read_db"]
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
        args.kernel_llm_name,
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


def as_int(value: str | int | None) -> int:
    if value is None or str(value).strip() == "":
        return 0
    try:
        return int(float(str(value).strip()))
    except ValueError:
        return 0


def summarize_run(run: dict) -> dict:
    rows = read_rows(repo_root() / run["csv"])
    total = len(rows)
    attack_success = sum(int(row.get("Attack Successful") or 0) for row in rows)
    original_success = sum(int(row.get("Original Task Successful") or 0) for row in rows)
    refusal = sum(int(row.get("Refuse Result") or 0) for row in rows)
    aggressive = sum(int(row.get("Aggressive") or 0) for row in rows)
    memory_found = sum(as_int(row.get("Memory Found")) for row in rows)
    input_tokens = sum(as_int(row.get("Input Tokens")) for row in rows)
    output_tokens = sum(as_int(row.get("Output Tokens")) for row in rows)
    total_tokens = sum(as_int(row.get("Total Tokens")) for row in rows)
    keyword_blocked = sum(as_int(row.get("Keyword Blocked")) for row in rows)
    keyword_sanitized = sum(as_int(row.get("Keyword Sanitized")) for row in rows)
    detector_triggered = sum(as_int(row.get("LLM Detector Triggered")) for row in rows)
    detector_input_tokens = sum(as_int(row.get("LLM Detector Input Tokens")) for row in rows)
    detector_output_tokens = sum(as_int(row.get("LLM Detector Output Tokens")) for row in rows)
    detector_total_tokens = sum(as_int(row.get("LLM Detector Total Tokens")) for row in rows)
    keyword_stages: dict[str, int] = {}
    keyword_categories: dict[str, int] = {}
    keyword_actions: dict[str, int] = {}
    detector_actions: dict[str, int] = {}
    detector_labels: dict[str, int] = {}
    detector_risk_types: dict[str, int] = {}
    detector_stages: dict[str, int] = {}
    for row in rows:
        stage = (row.get("Keyword Stage") or "").strip()
        if stage:
            keyword_stages[stage] = keyword_stages.get(stage, 0) + 1
        action = (row.get("Keyword Action") or "").strip()
        if action:
            keyword_actions[action] = keyword_actions.get(action, 0) + 1
        detector_action = (row.get("LLM Detector Action") or "").strip()
        if detector_action:
            detector_actions[detector_action] = detector_actions.get(detector_action, 0) + 1
        detector_label = (row.get("LLM Detector Label") or "").strip()
        if detector_label:
            detector_labels[detector_label] = detector_labels.get(detector_label, 0) + 1
        detector_risk = (row.get("LLM Detector Risk Type") or "").strip()
        if detector_risk:
            detector_risk_types[detector_risk] = detector_risk_types.get(detector_risk, 0) + 1
        detector_stage = (row.get("LLM Detector Stage") or "").strip()
        if detector_stage:
            detector_stages[detector_stage] = detector_stages.get(detector_stage, 0) + 1
        try:
            categories = json.loads(row.get("Keyword Categories") or "[]")
        except json.JSONDecodeError:
            categories = []
        for category in categories:
            keyword_categories[category] = keyword_categories.get(category, 0) + 1
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
        "memory_found_count": memory_found,
        "memory_found_rate": rate(memory_found, total),
        "input_tokens": input_tokens,
        "output_tokens": output_tokens,
        "total_tokens": total_tokens,
        "avg_input_tokens": round(input_tokens / total, 2) if total else 0,
        "avg_output_tokens": round(output_tokens / total, 2) if total else 0,
        "avg_total_tokens": round(total_tokens / total, 2) if total else 0,
        "keyword_blocked_count": keyword_blocked,
        "keyword_blocked_rate": rate(keyword_blocked, total),
        "keyword_sanitized_count": keyword_sanitized,
        "keyword_sanitized_rate": rate(keyword_sanitized, total),
        "keyword_action_counts": keyword_actions,
        "keyword_stage_counts": keyword_stages,
        "keyword_category_counts": keyword_categories,
        "llm_detector_triggered_count": detector_triggered,
        "llm_detector_triggered_rate": rate(detector_triggered, total),
        "llm_detector_action_counts": detector_actions,
        "llm_detector_label_counts": detector_labels,
        "llm_detector_risk_type_counts": detector_risk_types,
        "llm_detector_stage_counts": detector_stages,
        "llm_detector_input_tokens": detector_input_tokens,
        "llm_detector_output_tokens": detector_output_tokens,
        "llm_detector_total_tokens": detector_total_tokens,
        "combined_input_tokens": input_tokens + detector_input_tokens,
        "combined_output_tokens": output_tokens + detector_output_tokens,
        "combined_total_tokens": total_tokens + detector_total_tokens,
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
        f"- Memory found rate: `{summary['aggregate']['memory_found_rate']}` ({summary['aggregate']['memory_found_count']}/{summary['aggregate']['total']})",
        f"- Input tokens: `{summary['aggregate']['input_tokens']}`",
        f"- Output tokens: `{summary['aggregate']['output_tokens']}`",
        f"- Total tokens: `{summary['aggregate']['total_tokens']}`",
        f"- Detector total tokens: `{summary['aggregate']['llm_detector_total_tokens']}`",
        f"- Combined total tokens: `{summary['aggregate']['combined_total_tokens']}`",
        f"- Total time seconds: `{summary['aggregate']['total_time_seconds']}`",
        f"- Keyword blocked rate: `{summary['aggregate']['keyword_blocked_rate']}` ({summary['aggregate']['keyword_blocked_count']}/{summary['aggregate']['total']})",
        f"- Keyword sanitized rate: `{summary['aggregate']['keyword_sanitized_rate']}` ({summary['aggregate']['keyword_sanitized_count']}/{summary['aggregate']['total']})",
        f"- LLM detector triggered rate: `{summary['aggregate']['llm_detector_triggered_rate']}` ({summary['aggregate']['llm_detector_triggered_count']}/{summary['aggregate']['total']})",
        "",
        "## Per Attack Type",
        "",
        "| Attack Type | Cases | ASR | Original Success Rate | Refusal Rate | Memory Found Rate | Keyword Block Rate | Keyword Sanitize Rate | LLM Detector Rate | Agent Tokens | Detector Tokens | Combined Tokens | Time Seconds | Return Code |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for item in summary["runs"]:
        lines.append(
            f"| `{item['attack_type']}` | {item['total']} | {item['attack_success_rate']} | "
            f"{item['original_task_success_rate']} | {item['refusal_rate']} | {item['memory_found_rate']} | "
            f"{item['keyword_blocked_rate']} | {item['keyword_sanitized_rate']} | "
            f"{item['llm_detector_triggered_rate']} | "
            f"{item['total_tokens']} | {item['llm_detector_total_tokens']} | {item['combined_total_tokens']} | "
            f"{item['duration_seconds']} | {item['returncode']} |"
        )
    lines.extend([
        "",
        "## Keyword Filter Distribution",
        "",
        f"- Action counts: `{json.dumps(summary['aggregate']['keyword_action_counts'], ensure_ascii=False)}`",
        f"- Stage counts: `{json.dumps(summary['aggregate']['keyword_stage_counts'], ensure_ascii=False)}`",
        f"- Category counts: `{json.dumps(summary['aggregate']['keyword_category_counts'], ensure_ascii=False)}`",
        "",
        "## LLM Detector Distribution",
        "",
        f"- Action counts: `{json.dumps(summary['aggregate']['llm_detector_action_counts'], ensure_ascii=False)}`",
        f"- Label counts: `{json.dumps(summary['aggregate']['llm_detector_label_counts'], ensure_ascii=False)}`",
        f"- Risk type counts: `{json.dumps(summary['aggregate']['llm_detector_risk_type_counts'], ensure_ascii=False)}`",
        f"- Stage counts: `{json.dumps(summary['aggregate']['llm_detector_stage_counts'], ensure_ascii=False)}`",
    ])
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
        "memory_found_count",
        "memory_found_rate",
        "input_tokens",
        "output_tokens",
        "total_tokens",
        "avg_input_tokens",
        "avg_output_tokens",
        "avg_total_tokens",
        "keyword_blocked_count",
        "keyword_blocked_rate",
        "keyword_sanitized_count",
        "keyword_sanitized_rate",
        "keyword_action_counts",
        "keyword_stage_counts",
        "keyword_category_counts",
        "llm_detector_triggered_count",
        "llm_detector_triggered_rate",
        "llm_detector_action_counts",
        "llm_detector_label_counts",
        "llm_detector_risk_type_counts",
        "llm_detector_stage_counts",
        "llm_detector_input_tokens",
        "llm_detector_output_tokens",
        "llm_detector_total_tokens",
        "combined_input_tokens",
        "combined_output_tokens",
        "combined_total_tokens",
        "duration_seconds",
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
                "memory_found_count": item["memory_found_count"],
                "memory_found_rate": item["memory_found_rate"],
                "input_tokens": item["input_tokens"],
                "output_tokens": item["output_tokens"],
                "total_tokens": item["total_tokens"],
                "avg_input_tokens": item["avg_input_tokens"],
                "avg_output_tokens": item["avg_output_tokens"],
                "avg_total_tokens": item["avg_total_tokens"],
                "keyword_blocked_count": item["keyword_blocked_count"],
                "keyword_blocked_rate": item["keyword_blocked_rate"],
                "keyword_sanitized_count": item["keyword_sanitized_count"],
                "keyword_sanitized_rate": item["keyword_sanitized_rate"],
                "keyword_action_counts": json.dumps(item["keyword_action_counts"], ensure_ascii=False),
                "keyword_stage_counts": json.dumps(item["keyword_stage_counts"], ensure_ascii=False),
                "keyword_category_counts": json.dumps(item["keyword_category_counts"], ensure_ascii=False),
                "llm_detector_triggered_count": item["llm_detector_triggered_count"],
                "llm_detector_triggered_rate": item["llm_detector_triggered_rate"],
                "llm_detector_action_counts": json.dumps(item["llm_detector_action_counts"], ensure_ascii=False),
                "llm_detector_label_counts": json.dumps(item["llm_detector_label_counts"], ensure_ascii=False),
                "llm_detector_risk_type_counts": json.dumps(item["llm_detector_risk_type_counts"], ensure_ascii=False),
                "llm_detector_stage_counts": json.dumps(item["llm_detector_stage_counts"], ensure_ascii=False),
                "llm_detector_input_tokens": item["llm_detector_input_tokens"],
                "llm_detector_output_tokens": item["llm_detector_output_tokens"],
                "llm_detector_total_tokens": item["llm_detector_total_tokens"],
                "combined_input_tokens": item["combined_input_tokens"],
                "combined_output_tokens": item["combined_output_tokens"],
                "combined_total_tokens": item["combined_total_tokens"],
                "duration_seconds": item["duration_seconds"],
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
            "memory_found_count": aggregate["memory_found_count"],
            "memory_found_rate": aggregate["memory_found_rate"],
            "input_tokens": aggregate["input_tokens"],
            "output_tokens": aggregate["output_tokens"],
            "total_tokens": aggregate["total_tokens"],
            "avg_input_tokens": aggregate["avg_input_tokens"],
            "avg_output_tokens": aggregate["avg_output_tokens"],
            "avg_total_tokens": aggregate["avg_total_tokens"],
            "keyword_blocked_count": aggregate["keyword_blocked_count"],
            "keyword_blocked_rate": aggregate["keyword_blocked_rate"],
            "keyword_sanitized_count": aggregate["keyword_sanitized_count"],
            "keyword_sanitized_rate": aggregate["keyword_sanitized_rate"],
            "keyword_action_counts": json.dumps(aggregate["keyword_action_counts"], ensure_ascii=False),
            "keyword_stage_counts": json.dumps(aggregate["keyword_stage_counts"], ensure_ascii=False),
            "keyword_category_counts": json.dumps(aggregate["keyword_category_counts"], ensure_ascii=False),
            "llm_detector_triggered_count": aggregate["llm_detector_triggered_count"],
            "llm_detector_triggered_rate": aggregate["llm_detector_triggered_rate"],
            "llm_detector_action_counts": json.dumps(aggregate["llm_detector_action_counts"], ensure_ascii=False),
            "llm_detector_label_counts": json.dumps(aggregate["llm_detector_label_counts"], ensure_ascii=False),
            "llm_detector_risk_type_counts": json.dumps(aggregate["llm_detector_risk_type_counts"], ensure_ascii=False),
            "llm_detector_stage_counts": json.dumps(aggregate["llm_detector_stage_counts"], ensure_ascii=False),
            "llm_detector_input_tokens": aggregate["llm_detector_input_tokens"],
            "llm_detector_output_tokens": aggregate["llm_detector_output_tokens"],
            "llm_detector_total_tokens": aggregate["llm_detector_total_tokens"],
            "combined_input_tokens": aggregate["combined_input_tokens"],
            "combined_output_tokens": aggregate["combined_output_tokens"],
            "combined_total_tokens": aggregate["combined_total_tokens"],
            "duration_seconds": aggregate["total_time_seconds"],
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
        "input_tokens",
        "output_tokens",
        "total_tokens",
        "keyword_blocked",
        "keyword_sanitized",
        "keyword_action",
        "keyword_stage",
        "keyword_categories",
        "keyword_matches",
        "keyword_original_length",
        "keyword_sanitized_length",
        "llm_detector_triggered",
        "llm_detector_action",
        "llm_detector_stage",
        "llm_detector_label",
        "llm_detector_risk_type",
        "llm_detector_confidence",
        "llm_detector_input_tokens",
        "llm_detector_output_tokens",
        "llm_detector_total_tokens",
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
                    "input_tokens": row.get("Input Tokens", ""),
                    "output_tokens": row.get("Output Tokens", ""),
                    "total_tokens": row.get("Total Tokens", ""),
                    "keyword_blocked": row.get("Keyword Blocked", ""),
                    "keyword_sanitized": row.get("Keyword Sanitized", ""),
                    "keyword_action": row.get("Keyword Action", ""),
                    "keyword_stage": row.get("Keyword Stage", ""),
                    "keyword_categories": row.get("Keyword Categories", ""),
                    "keyword_matches": row.get("Keyword Matches", ""),
                    "keyword_original_length": row.get("Keyword Original Length", ""),
                    "keyword_sanitized_length": row.get("Keyword Sanitized Length", ""),
                    "llm_detector_triggered": row.get("LLM Detector Triggered", ""),
                    "llm_detector_action": row.get("LLM Detector Action", ""),
                    "llm_detector_stage": row.get("LLM Detector Stage", ""),
                    "llm_detector_label": row.get("LLM Detector Label", ""),
                    "llm_detector_risk_type": row.get("LLM Detector Risk Type", ""),
                    "llm_detector_confidence": row.get("LLM Detector Confidence", ""),
                    "llm_detector_input_tokens": row.get("LLM Detector Input Tokens", ""),
                    "llm_detector_output_tokens": row.get("LLM Detector Output Tokens", ""),
                    "llm_detector_total_tokens": row.get("LLM Detector Total Tokens", ""),
                })


def main() -> int:
    parser = argparse.ArgumentParser(description="Run ASB LangGraph batch tests and summarize metrics.")
    parser.add_argument("--run-id", default=f"langgraph-batch-{datetime.now().strftime('%Y%m%d-%H%M%S')}")
    parser.add_argument("--python", type=Path, default=default_python())
    parser.add_argument("--llm-name", default="gpt-4o-mini")
    parser.add_argument(
        "--kernel-llm-name",
        default="gpt-4o-mini",
        help="Registered ASB LLMKernel placeholder. LangGraph uses --llm-name via LANGGRAPH_OPENAI_MODEL.",
    )
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
    parser.add_argument(
        "--mp-write-only",
        action="store_true",
        help="For MP preparation runs, use --memory_attack --write_db without the family default --read_db.",
    )
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
        "memory_found_count": sum(item["memory_found_count"] for item in run_summaries),
        "input_tokens": sum(item["input_tokens"] for item in run_summaries),
        "output_tokens": sum(item["output_tokens"] for item in run_summaries),
        "total_tokens": sum(item["total_tokens"] for item in run_summaries),
        "total_time_seconds": round(sum(item["duration_seconds"] for item in run_summaries), 3),
        "keyword_blocked_count": sum(item["keyword_blocked_count"] for item in run_summaries),
        "keyword_sanitized_count": sum(item["keyword_sanitized_count"] for item in run_summaries),
        "llm_detector_triggered_count": sum(item["llm_detector_triggered_count"] for item in run_summaries),
        "llm_detector_input_tokens": sum(item["llm_detector_input_tokens"] for item in run_summaries),
        "llm_detector_output_tokens": sum(item["llm_detector_output_tokens"] for item in run_summaries),
        "llm_detector_total_tokens": sum(item["llm_detector_total_tokens"] for item in run_summaries),
        "combined_input_tokens": sum(item["combined_input_tokens"] for item in run_summaries),
        "combined_output_tokens": sum(item["combined_output_tokens"] for item in run_summaries),
        "combined_total_tokens": sum(item["combined_total_tokens"] for item in run_summaries),
    }
    aggregate["attack_success_rate"] = rate(aggregate["attack_success"], aggregate["total"])
    aggregate["original_task_success_rate"] = rate(aggregate["original_task_success"], aggregate["total"])
    aggregate["refusal_rate"] = rate(aggregate["refusal_count"], aggregate["total"])
    aggregate["memory_found_rate"] = rate(aggregate["memory_found_count"], aggregate["total"])
    aggregate["avg_input_tokens"] = round(aggregate["input_tokens"] / aggregate["total"], 2) if aggregate["total"] else 0
    aggregate["avg_output_tokens"] = round(aggregate["output_tokens"] / aggregate["total"], 2) if aggregate["total"] else 0
    aggregate["avg_total_tokens"] = round(aggregate["total_tokens"] / aggregate["total"], 2) if aggregate["total"] else 0
    aggregate["keyword_blocked_rate"] = rate(aggregate["keyword_blocked_count"], aggregate["total"])
    aggregate["keyword_sanitized_rate"] = rate(aggregate["keyword_sanitized_count"], aggregate["total"])
    aggregate["llm_detector_triggered_rate"] = rate(aggregate["llm_detector_triggered_count"], aggregate["total"])
    aggregate["keyword_action_counts"] = {}
    aggregate["keyword_stage_counts"] = {}
    aggregate["keyword_category_counts"] = {}
    aggregate["llm_detector_action_counts"] = {}
    aggregate["llm_detector_label_counts"] = {}
    aggregate["llm_detector_risk_type_counts"] = {}
    aggregate["llm_detector_stage_counts"] = {}
    for item in run_summaries:
        for action, count in item["keyword_action_counts"].items():
            aggregate["keyword_action_counts"][action] = aggregate["keyword_action_counts"].get(action, 0) + count
        for stage, count in item["keyword_stage_counts"].items():
            aggregate["keyword_stage_counts"][stage] = aggregate["keyword_stage_counts"].get(stage, 0) + count
        for category, count in item["keyword_category_counts"].items():
            aggregate["keyword_category_counts"][category] = aggregate["keyword_category_counts"].get(category, 0) + count
        for action, count in item["llm_detector_action_counts"].items():
            aggregate["llm_detector_action_counts"][action] = aggregate["llm_detector_action_counts"].get(action, 0) + count
        for label, count in item["llm_detector_label_counts"].items():
            aggregate["llm_detector_label_counts"][label] = aggregate["llm_detector_label_counts"].get(label, 0) + count
        for risk, count in item["llm_detector_risk_type_counts"].items():
            aggregate["llm_detector_risk_type_counts"][risk] = aggregate["llm_detector_risk_type_counts"].get(risk, 0) + count
        for stage, count in item["llm_detector_stage_counts"].items():
            aggregate["llm_detector_stage_counts"][stage] = aggregate["llm_detector_stage_counts"].get(stage, 0) + count

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
