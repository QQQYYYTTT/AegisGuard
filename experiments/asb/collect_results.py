from __future__ import annotations

import argparse
import csv
import json
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


REPO_ROOT = Path(__file__).resolve().parents[2]
RESULTS_DIR = REPO_ROOT / "experiments" / "asb" / "results"
TRACE_DIR = RESULTS_DIR / "traces"
if str(REPO_ROOT) not in sys.path:
    sys.path.insert(0, str(REPO_ROOT))

from experiments.eval.metrics import summarize_records  # noqa: E402
from experiments.eval.schema import FIELDNAMES, ExperimentRecord  # noqa: E402


ATTACK_METADATA = {
    "dpi": {
        "suite": "direct_prompt_injection",
    },
    "opi": {
        "suite": "observation_prompt_injection",
    },
    "mp": {
        "suite": "memory_poisoning",
    },
    "mixed": {
        "suite": "mixed_attack",
    },
    "pot": {
        "suite": "plan_of_thought_backdoor",
    },
    "tool": {
        "suite": "tool_use_or_privilege_misuse",
    },
}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Convert ASB output files into AegisGuard result schema.")
    parser.add_argument(
        "--input",
        action="append",
        required=True,
        help="ASB result CSV/JSON file or directory. May be passed multiple times.",
    )
    parser.add_argument("--attack", choices=sorted(ATTACK_METADATA), required=True)
    parser.add_argument("--run-id", default="", help="Run id to write into AegisGuard records.")
    parser.add_argument("--defense", default="asb_config", help="Defense label recorded in the output CSV.")
    parser.add_argument("--agent-name", default="", help="Override detected ASB agent name.")
    parser.add_argument("--agent-version", default="asb-original", help="Agent version label.")
    parser.add_argument("--output-prefix", default="", help="Output file prefix. Defaults to --run-id.")
    return parser.parse_args()


def normalize_key(key: str) -> str:
    return "".join(ch for ch in key.lower() if ch.isalnum())


def first_value(row: dict[str, Any], candidates: tuple[str, ...], default: str = "") -> str:
    normalized = {normalize_key(key): key for key in row}
    for candidate in candidates:
        key = normalized.get(normalize_key(candidate))
        if key is not None:
            value = row.get(key)
            if value is not None and str(value).strip() != "":
                return str(value).strip()
    return default


def as_bool(value: Any) -> bool:
    if isinstance(value, bool):
        return value
    if value is None:
        return False
    return str(value).strip().lower() in {"1", "true", "yes", "y", "success", "successful", "succeeded"}


def as_float(value: Any) -> float:
    if value is None or str(value).strip() == "":
        return 0.0
    try:
        raw = float(str(value).strip())
    except ValueError:
        return 0.0
    return raw / 100 if raw > 1 else raw


def discover_files(inputs: list[str]) -> list[Path]:
    files: list[Path] = []
    for raw in inputs:
        path = Path(raw).resolve()
        if path.is_dir():
            files.extend(sorted(path.rglob("*.csv")))
            files.extend(sorted(path.rglob("*.json")))
        elif path.is_file() and path.suffix.lower() in {".csv", ".json"}:
            files.append(path)
        else:
            print(f"Skipping missing or unsupported input: {path}", file=sys.stderr)
    return files


def read_csv(path: Path) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    with path.open("r", encoding="utf-8-sig", newline="") as handle:
        reader = csv.DictReader(handle)
        for row in reader:
            row["_source_file"] = str(path)
            rows.append(dict(row))
    return rows


def flatten_json_payload(payload: Any) -> list[dict[str, Any]]:
    if isinstance(payload, list):
        return [item for item in payload if isinstance(item, dict)]
    if isinstance(payload, dict):
        for key in ("results", "records", "rows", "data"):
            value = payload.get(key)
            if isinstance(value, list):
                return [item for item in value if isinstance(item, dict)]
        return [payload]
    return []


def read_json(path: Path) -> list[dict[str, Any]]:
    with path.open("r", encoding="utf-8") as handle:
        payload = json.load(handle)
    rows = flatten_json_payload(payload)
    for row in rows:
        row["_source_file"] = str(path)
    return rows


def load_rows(files: list[Path]) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for path in files:
        try:
            if path.suffix.lower() == ".csv":
                rows.extend(read_csv(path))
            elif path.suffix.lower() == ".json":
                rows.extend(read_json(path))
        except Exception as exc:  # noqa: BLE001
            print(f"Could not parse {path}: {exc}", file=sys.stderr)
    return rows


def record_from_row(row: dict[str, Any], index: int, args: argparse.Namespace, run_id: str) -> ExperimentRecord:
    metadata = ATTACK_METADATA[args.attack]
    case_id = first_value(
        row,
        (
            "case_id",
            "case id",
            "task_id",
            "task id",
            "index",
            "idx",
            "example_id",
            "id",
        ),
        default=f"{args.attack}-{index:04d}",
    )
    variant_name = first_value(row, ("attack", "attack_type", "attack method", "scenario", "task", "dataset"), args.attack)
    agent_name = args.agent_name or first_value(row, ("agent_name", "agent name", "agent", "model", "llm"), "ASB agent")
    defense = args.defense or first_value(row, ("defense", "defense_type", "defense method"), "asb_config")
    defense_enabled = as_bool(
        first_value(
            row,
            (
                "defense_enabled",
                "with_defense",
                "defended",
                "is_defense",
            ),
            "true" if defense and defense not in {"none", "no_defense", "asb_config"} else "false",
        )
    )
    under_attack = as_bool(
        first_value(
            row,
            (
                "under_attack",
                "is_attack",
                "attacked",
                "attack_enabled",
            ),
            "true",
        )
    )

    attack_success = as_bool(
        first_value(
            row,
            (
                "attack_success",
                "attack successful",
                "attack success",
                "is_attack_success",
                "asr",
                "success",
            ),
        )
    )
    task_completed = as_bool(
        first_value(
            row,
            (
                "task_completed",
                "original task successful",
                "task_success",
                "task successful",
                "utility",
                "benign_success",
            ),
        )
    )
    blocked = as_bool(
        first_value(
            row,
            (
                "blocked",
                "refuse result",
                "refused",
                "refusal",
                "is_refused",
                "safe_redirect",
            ),
        )
    )
    false_positive = as_bool(first_value(row, ("false_positive", "fpr", "benign blocked"), "false"))
    benign_success = as_bool(
        first_value(
            row,
            (
                "benign_success",
                "benign successful",
                "benign performance",
                "bp_success",
                "clean_success",
            ),
        )
    )
    poison_detected = as_bool(
        first_value(
            row,
            (
                "poison_detected",
                "detected_poison",
                "poison detection",
                "detected",
                "is_detected",
            ),
        )
    )
    clean_detected_as_poison = as_bool(
        first_value(
            row,
            (
                "clean_detected_as_poison",
                "false_positive",
                "fpr",
                "clean_flagged",
                "benign blocked",
            ),
            "false",
        )
    )
    latency_raw = first_value(row, ("latency_ms", "latency", "time_ms", "duration_ms", "elapsed_ms"), "0")
    try:
        latency_ms = int(float(latency_raw))
    except ValueError:
        latency_ms = 0

    TRACE_DIR.mkdir(parents=True, exist_ok=True)
    safe_case_id = "".join(ch if ch.isalnum() or ch in {"-", "_"} else "-" for ch in case_id)
    trace_path = TRACE_DIR / f"{run_id}-{args.attack}-{safe_case_id}-{index:04d}.json"
    trace_payload = {
        "run_id": run_id,
        "attack": args.attack,
        "case_id": case_id,
        "source_file": row.get("_source_file", ""),
        "raw_asb_row": row,
    }
    trace_path.write_text(json.dumps(trace_payload, indent=2, ensure_ascii=False), encoding="utf-8")

    notes = []
    if row.get("_source_file"):
        notes.append(f"source={row['_source_file']}")
    if not any(normalize_key(key) in {normalize_key("attack_success"), normalize_key("attack successful")} for key in row):
        notes.append("review_success_mapping")

    return ExperimentRecord(
        run_id=run_id,
        repeat_index=1,
        benchmark_family="ASB",
        benchmark_suite=metadata["suite"],
        asb_attack=args.attack,
        case_id=case_id,
        scenario=variant_name,
        agent_name=agent_name,
        agent_version=args.agent_version,
        defense=defense,
        under_attack=under_attack,
        defense_enabled=defense_enabled,
        attack_success=attack_success,
        refused=blocked,
        task_success=task_completed,
        benign_success=benign_success,
        poison_detected=poison_detected,
        clean_detected_as_poison=clean_detected_as_poison,
        latency_ms=latency_ms,
        asr=as_float(first_value(row, ("asr", "attack_success_rate"), "")),
        asr_d=as_float(first_value(row, ("asr_d", "asr-d", "attack_success_rate_under_defense"), "")),
        rr=as_float(first_value(row, ("rr", "refuse_rate", "refusal_rate"), "")),
        pna=as_float(first_value(row, ("pna", "performance_under_no_attack"), "")),
        pna_d=as_float(first_value(row, ("pna_d", "pna-d", "performance_under_no_attack_under_defense"), "")),
        bp=as_float(first_value(row, ("bp", "benign_performance"), "")),
        fnr=as_float(first_value(row, ("fnr", "false_negative_rate"), "")),
        fpr=as_float(first_value(row, ("fpr", "false_positive_rate"), "")) if false_positive else 0.0,
        trace_path=str(trace_path.relative_to(REPO_ROOT)),
        raw_source_path=str(row.get("_source_file", "")),
        judge_method="asb_original_output+mapping_review",
        evaluator_version="2026-04-21.asb-adapter-v1",
        timestamp_utc=datetime.now(timezone.utc).isoformat(),
        notes="; ".join(notes),
    )


def main() -> None:
    args = parse_args()
    run_id = args.run_id or f"asb-{args.attack}-{datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')}"
    output_prefix = args.output_prefix or run_id

    files = discover_files(args.input)
    if not files:
        raise SystemExit("No ASB CSV/JSON files found.")

    raw_rows = load_rows(files)
    if not raw_rows:
        raise SystemExit("No rows could be parsed from ASB outputs.")

    RESULTS_DIR.mkdir(parents=True, exist_ok=True)
    records = [record_from_row(row, index, args, run_id) for index, row in enumerate(raw_rows, 1)]
    rows = [record.to_row() for record in records]

    csv_path = RESULTS_DIR / f"{output_prefix}-results.csv"
    with csv_path.open("w", encoding="utf-8", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=FIELDNAMES)
        writer.writeheader()
        writer.writerows(rows)

    summary = {
        "run_id": run_id,
        "benchmark": "ASB",
        "attack": args.attack,
        "input_files": [str(path) for path in files],
        "metrics": summarize_records(rows, group_keys=("benchmark_suite", "asb_attack", "defense")),
    }
    summary_path = RESULTS_DIR / f"{output_prefix}-summary.json"
    summary_path.write_text(json.dumps(summary, indent=2, ensure_ascii=False), encoding="utf-8")

    print(f"Converted {len(rows)} ASB rows.")
    print(f"CSV: {csv_path}")
    print(f"Summary: {summary_path}")


if __name__ == "__main__":
    main()
