from __future__ import annotations

import argparse
import csv
import json
import os
import signal
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


REPO_ROOT = Path(__file__).resolve().parents[3]
RESULTS_DIR = REPO_ROOT / "experiments" / "asb" / "results"
TRACE_DIR = RESULTS_DIR / "traces"
PINNED_OPENCLAW_VERSION = "2026.5.28"
DEFAULT_STATE_DIR = REPO_ROOT / ".tmp" / "openclaw-state"
DEFAULT_WORKSPACE_DIR = REPO_ROOT / ".tmp" / "openclaw-workspace"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run OpenClaw as a black-box agent and save ASB-compatible raw rows.")
    parser.add_argument(
        "--openclaw-bin",
        default="",
        help="Executable to launch. Defaults to this repo's pinned node_modules OpenClaw.",
    )
    parser.add_argument(
        "--openclaw-arg",
        action="append",
        default=[],
        help="Extra argument inserted before 'agent', e.g. --openclaw-arg openclaw for 'pnpm openclaw agent'.",
    )
    parser.add_argument(
        "--agent-arg",
        action="append",
        default=[],
        help="Extra argument inserted after 'agent' and before '--message', e.g. --agent-arg --local.",
    )
    parser.add_argument("--cwd", default="", help="Working directory for the OpenClaw command.")
    parser.add_argument("--message", default="", help="Single message to send to OpenClaw.")
    parser.add_argument("--input-jsonl", default="", help="JSONL file with fields case_id and message.")
    parser.add_argument("--start-index", type=int, default=1, help="1-based start index for JSONL tasks.")
    parser.add_argument("--max-cases", type=int, default=0, help="Maximum number of JSONL tasks to run.")
    parser.add_argument("--run-id", default="", help="Stable run id.")
    parser.add_argument("--timeout", type=int, default=180, help="Per-case timeout in seconds.")
    parser.add_argument(
        "--gateway",
        action="store_true",
        help="Use the OpenClaw Gateway instead of the embedded local agent. Default is local mode for test stability.",
    )
    parser.add_argument(
        "--env",
        action="append",
        default=[],
        help="Environment override in KEY=VALUE form. May be passed multiple times.",
    )
    parser.add_argument(
        "--env-file",
        default=str(REPO_ROOT / ".env"),
        help="Optional KEY=VALUE env file to load before --env overrides. Existing process env wins.",
    )
    parser.add_argument(
        "--state-dir",
        default=str(DEFAULT_STATE_DIR),
        help="OpenClaw state directory. Defaults to a repo-local .tmp directory.",
    )
    parser.add_argument(
        "--workspace-dir",
        default=str(DEFAULT_WORKSPACE_DIR),
        help="OpenClaw agent workspace used when bootstrapping repo-local config.",
    )
    parser.add_argument(
        "--no-bootstrap-config",
        action="store_true",
        help="Do not create a minimal OpenClaw config when the selected state dir has none.",
    )
    parser.add_argument("--agent-name", default="OpenClaw")
    parser.add_argument("--agent-version", default=f"npm-{PINNED_OPENCLAW_VERSION}")
    parser.add_argument("--attack", default="openclaw_probe", help="Attack or suite label to write into the CSV.")
    parser.add_argument(
        "--fail-on-error",
        action="store_true",
        help="Exit non-zero if any case times out, exits non-zero, or produces no assistant text/stdout.",
    )
    return parser.parse_args()


def default_openclaw_bin() -> str:
    if os.name == "nt":
        candidate = REPO_ROOT / "node_modules" / ".bin" / "openclaw.cmd"
    else:
        candidate = REPO_ROOT / "node_modules" / ".bin" / "openclaw"
    if candidate.exists():
        return str(candidate)
    return "openclaw"


def load_env_file(env: dict[str, str], env_file: str) -> None:
    if not env_file:
        return
    path = Path(env_file)
    if not path.exists():
        return
    with path.open("r", encoding="utf-8") as handle:
        for line in handle:
            stripped = line.strip()
            if not stripped or stripped.startswith("#") or "=" not in stripped:
                continue
            key, value = stripped.split("=", 1)
            key = key.strip()
            if not key or key in env:
                continue
            value = value.strip()
            if len(value) >= 2 and value[0] == value[-1] and value[0] in {"'", '"'}:
                value = value[1:-1]
            env[key] = value


def load_tasks(args: argparse.Namespace) -> list[dict[str, str]]:
    tasks: list[dict[str, Any]] = []
    if args.message:
        tasks.append({"case_id": "manual-001", "message": args.message})
    if args.input_jsonl:
        path = Path(args.input_jsonl)
        selected = 0
        with path.open("r", encoding="utf-8") as handle:
            for index, line in enumerate(handle, 1):
                if not line.strip():
                    continue
                if index < args.start_index:
                    continue
                if args.max_cases and selected >= args.max_cases:
                    break
                item = json.loads(line)
                task = dict(item)
                task["case_id"] = str(item.get("case_id") or f"case-{index:04d}")
                task["message"] = str(item["message"])
                tasks.append(task)
                selected += 1
    if not tasks:
        raise SystemExit("Provide --message or --input-jsonl.")
    return tasks


def build_env(args: argparse.Namespace) -> dict[str, str]:
    env = os.environ.copy()
    load_env_file(env, args.env_file)
    for item in args.env:
        if "=" not in item:
            raise SystemExit(f"Invalid --env value, expected KEY=VALUE: {item}")
        key, value = item.split("=", 1)
        if not key.strip():
            raise SystemExit(f"Invalid --env value, empty key: {item}")
        env[key.strip()] = value
    if args.state_dir:
        state_dir = Path(args.state_dir).resolve()
        env["OPENCLAW_STATE_DIR"] = str(state_dir)
        env.setdefault("OPENCLAW_CONFIG_PATH", str(state_dir / "openclaw.json"))
    return env


def model_provider_from_env(env: dict[str, str]) -> tuple[str, dict[str, Any], str] | None:
    if env.get("OFOXAI_API_KEY"):
        model_id = env.get("OPENAI_MODEL") or env.get("CUSTOM_MODEL_ID") or "openai/gpt-4o-mini"
        provider_id = "ofoxai-openai"
        return (
            provider_id,
            {
                "models": [
                    {
                        "id": model_id,
                        "name": model_id,
                        "reasoning": False,
                        "contextWindow": 128000,
                        "maxTokens": 768,
                    }
                ],
                "baseUrl": "https://api.ofox.ai/v1",
                "api": "openai-completions",
                "apiKey": {"provider": "default", "source": "env", "id": "OFOXAI_API_KEY"},
                "timeoutSeconds": int(env.get("OPENCLAW_MODEL_TIMEOUT_SECONDS") or env.get("OPENCLAW_TIMEOUT_SECONDS") or "240"),
            },
            model_id,
        )
    if env.get("CUSTOM_API_KEY") and env.get("CUSTOM_BASE_URL") and env.get("CUSTOM_MODEL_ID"):
        provider_id = "custom-openai"
        model_id = env["CUSTOM_MODEL_ID"]
        return (
            provider_id,
            {
                "models": [{"id": model_id, "name": model_id, "reasoning": False}],
                "baseUrl": env["CUSTOM_BASE_URL"],
                "api": "openai-completions",
                "apiKey": {"provider": "default", "source": "env", "id": "CUSTOM_API_KEY"},
                "timeoutSeconds": int(env.get("OPENCLAW_MODEL_TIMEOUT_SECONDS") or env.get("OPENCLAW_TIMEOUT_SECONDS") or "240"),
            },
            model_id,
        )
    if env.get("OPENAI_API_KEY"):
        provider_id = "openai"
        model_id = env.get("OPENAI_MODEL") or "gpt-4o-mini"
        provider: dict[str, Any] = {
            "models": [{"id": model_id, "name": model_id, "reasoning": False}],
            "api": "openai-completions",
            "apiKey": {"provider": "default", "source": "env", "id": "OPENAI_API_KEY"},
            "timeoutSeconds": int(env.get("OPENCLAW_MODEL_TIMEOUT_SECONDS") or env.get("OPENCLAW_TIMEOUT_SECONDS") or "240"),
        }
        if env.get("OPENAI_BASE_URL"):
            provider["baseUrl"] = env["OPENAI_BASE_URL"]
        return provider_id, provider, model_id
    return None


def bootstrap_openclaw_config(args: argparse.Namespace, env: dict[str, str]) -> None:
    if args.no_bootstrap_config:
        return
    config_path = Path(env.get("OPENCLAW_CONFIG_PATH") or "")
    if not config_path or config_path.exists():
        return
    provider = model_provider_from_env(env)
    if provider is None:
        return
    provider_id, provider_config, model_id = provider
    state_dir = Path(env["OPENCLAW_STATE_DIR"])
    workspace_dir = Path(args.workspace_dir).resolve()
    state_dir.mkdir(parents=True, exist_ok=True)
    workspace_dir.mkdir(parents=True, exist_ok=True)
    config = {
        "gateway": {
            "mode": "local",
            "port": 19089,
            "bind": "loopback",
            "auth": {"mode": "token", "token": "aegisguard-local"},
        },
        "models": {"mode": "merge", "providers": {provider_id: provider_config}},
        "agents": {
            "defaults": {
                "workspace": str(workspace_dir),
                "model": {"primary": f"{provider_id}/{model_id}"},
                "timeoutSeconds": int(args.timeout),
                "contextInjection": "continuation-skip",
            }
        },
    }
    config_path.write_text(json.dumps(config, indent=2, ensure_ascii=False), encoding="utf-8")


def session_files(state_dir: str) -> set[Path]:
    if not state_dir:
        return set()
    sessions_dir = Path(state_dir).expanduser().resolve() / "agents" / "main" / "sessions"
    if not sessions_dir.exists():
        return set()
    return {path for path in sessions_dir.glob("*.jsonl") if path.is_file()}


def latest_session_file(state_dir: str, before: set[Path]) -> Path | None:
    candidates = [path for path in session_files(state_dir) if path not in before]
    if not candidates:
        candidates = list(session_files(state_dir))
    if not candidates:
        return None
    return max(candidates, key=lambda path: path.stat().st_mtime)


def extract_text_blocks(content: Any) -> list[str]:
    if isinstance(content, str):
        return [content]
    if not isinstance(content, list):
        return []
    texts: list[str] = []
    for block in content:
        if isinstance(block, dict) and block.get("type") == "text":
            text = block.get("text")
            if isinstance(text, str) and text.strip():
                texts.append(text)
    return texts


def extract_assistant_text(session_file: Path | None) -> str:
    if session_file is None or not session_file.exists():
        return ""
    assistant_text = ""
    try:
        with session_file.open("r", encoding="utf-8") as handle:
            for line in handle:
                if not line.strip():
                    continue
                try:
                    record = json.loads(line)
                except json.JSONDecodeError:
                    continue
                message = record.get("message") if isinstance(record, dict) else None
                if not isinstance(message, dict) or message.get("role") != "assistant":
                    continue
                texts = extract_text_blocks(message.get("content"))
                if texts:
                    assistant_text = "\n".join(texts).strip()
    except OSError:
        return ""
    return assistant_text


def extract_assistant_text_from_stdout(stdout: str) -> str:
    if not stdout.strip():
        return ""
    try:
        payload = json.loads(stdout)
    except json.JSONDecodeError:
        return ""
    if not isinstance(payload, dict):
        return ""
    payloads = payload.get("payloads")
    if isinstance(payloads, list):
        texts = [
            item.get("text", "")
            for item in payloads
            if isinstance(item, dict) and isinstance(item.get("text"), str) and item.get("text", "").strip()
        ]
        if texts:
            return "\n".join(texts).strip()
    meta = payload.get("meta")
    if isinstance(meta, dict):
        for key in ("finalAssistantVisibleText", "finalAssistantRawText"):
            value = meta.get(key)
            if isinstance(value, str) and value.strip():
                return value.strip()
    return ""


def stop_process_tree(process: subprocess.Popen[str]) -> None:
    if process.poll() is not None:
        return
    if os.name == "nt":
        subprocess.run(
            ["taskkill", "/PID", str(process.pid), "/T", "/F"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        )
        return
    try:
        os.killpg(process.pid, signal.SIGTERM)
    except OSError:
        process.kill()


def has_agent_selector(agent_args: list[str]) -> bool:
    selector_flags = {"--agent", "--session-id", "--to", "-t"}
    return any(item in selector_flags or item.startswith("--agent=") or item.startswith("--session-id=") or item.startswith("--to=") for item in agent_args)


def run_openclaw(args: argparse.Namespace, message: str, run_id: str, case_id: str) -> tuple[int, str, str, int, list[str], str]:
    agent_args = list(args.agent_arg)
    if not args.gateway and "--local" not in agent_args:
        agent_args.insert(0, "--local")
    if "--json" not in agent_args:
        agent_args.append("--json")
    if not has_agent_selector(agent_args):
        agent_args.extend(["--session-id", f"{run_id}-{case_id}"])
    has_agent_timeout = any(item == "--timeout" or item.startswith("--timeout=") for item in agent_args)
    if not has_agent_timeout:
        agent_args.extend(["--timeout", str(args.timeout)])
    command = [args.openclaw_bin or default_openclaw_bin(), *args.openclaw_arg, "agent", *agent_args, "--message", message]
    env = build_env(args)
    bootstrap_openclaw_config(args, env)
    state_dir = args.state_dir or env.get("OPENCLAW_STATE_DIR", "")
    before_sessions = session_files(state_dir)
    started = time.perf_counter()
    creationflags = subprocess.CREATE_NEW_PROCESS_GROUP if os.name == "nt" else 0
    start_new_session = os.name != "nt"
    process: subprocess.Popen[str] | None = None
    try:
        process = subprocess.Popen(
            command,
            cwd=args.cwd or None,
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            encoding="utf-8",
            errors="replace",
            creationflags=creationflags,
            start_new_session=start_new_session,
        )
        stdout, stderr = process.communicate(timeout=args.timeout)
        latency_ms = int((time.perf_counter() - started) * 1000)
        session_file = latest_session_file(state_dir, before_sessions)
        return int(process.returncode or 0), stdout, stderr, latency_ms, command, str(session_file or "")
    except subprocess.TimeoutExpired as exc:
        if process is not None:
            stop_process_tree(process)
            try:
                stdout, stderr = process.communicate(timeout=5)
            except subprocess.TimeoutExpired:
                stdout, stderr = "", ""
        else:
            stdout = exc.stdout if isinstance(exc.stdout, str) else ""
            stderr = exc.stderr if isinstance(exc.stderr, str) else ""
        latency_ms = int((time.perf_counter() - started) * 1000)
        session_file = latest_session_file(state_dir, before_sessions)
        return 124, stdout, stderr + "\nTIMEOUT", latency_ms, command, str(session_file or "")


def write_outputs(args: argparse.Namespace, rows: list[dict[str, Any]], run_id: str) -> None:
    RESULTS_DIR.mkdir(parents=True, exist_ok=True)
    TRACE_DIR.mkdir(parents=True, exist_ok=True)
    csv_path = RESULTS_DIR / f"openclaw-{run_id}-raw.csv"
    fieldnames = [
        "run_id",
        "timestamp_utc",
        "agent_name",
        "agent_version",
        "attack",
        "asb_attack_type",
        "asb_agent_name",
        "asb_normal_task",
        "asb_attack_tool",
        "asb_attack_goal",
        "asb_aggressive",
        "case_id",
        "message",
        "stdout",
        "stderr",
        "exit_code",
        "latency_ms",
        "attack_success",
        "task_success",
        "refused",
        "trace_path",
    ]
    with csv_path.open("w", encoding="utf-8", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(rows)
    print(f"Raw OpenClaw CSV: {csv_path}")


def main() -> None:
    args = parse_args()
    run_id = args.run_id or f"openclaw-{datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')}"
    rows: list[dict[str, Any]] = []
    for task in load_tasks(args):
        exit_code, stdout, stderr, latency_ms, command, session_file = run_openclaw(args, task["message"], run_id, task["case_id"])
        assistant_text = extract_assistant_text(Path(session_file)) if session_file else ""
        if not assistant_text:
            assistant_text = extract_assistant_text_from_stdout(stdout)
        effective_stdout = assistant_text if assistant_text else stdout
        trace_path = TRACE_DIR / f"openclaw-{run_id}-{task['case_id']}.json"
        trace_path.parent.mkdir(parents=True, exist_ok=True)
        trace_payload = {
            "run_id": run_id,
            "case_id": task["case_id"],
            "command": command,
            "message": task["message"],
            "exit_code": exit_code,
            "stdout": stdout,
            "assistant_text": assistant_text,
            "stderr": stderr,
            "latency_ms": latency_ms,
            "openclaw_session_file": session_file,
        }
        trace_path.write_text(json.dumps(trace_payload, indent=2, ensure_ascii=False), encoding="utf-8")
        rows.append(
            {
                "run_id": run_id,
                "timestamp_utc": datetime.now(timezone.utc).isoformat(),
                "agent_name": args.agent_name,
                "agent_version": args.agent_version,
                "attack": args.attack,
                "asb_attack_type": task.get("attack_type", ""),
                "asb_agent_name": task.get("agent_name", ""),
                "asb_normal_task": task.get("normal_task", ""),
                "asb_attack_tool": task.get("attack_tool", ""),
                "asb_attack_goal": task.get("attack_goal", ""),
                "asb_aggressive": task.get("aggressive", ""),
                "case_id": task["case_id"],
                "message": task["message"],
                "stdout": effective_stdout,
                "stderr": stderr,
                "exit_code": exit_code,
                "latency_ms": latency_ms,
                "attack_success": "",
                "task_success": "1" if exit_code == 0 and effective_stdout.strip() else "0",
                "refused": "",
                "trace_path": str(trace_path.relative_to(REPO_ROOT)),
            }
        )
    write_outputs(args, rows, run_id)
    if args.fail_on_error:
        failed = [
            row
            for row in rows
            if str(row["exit_code"]) != "0" or not str(row["stdout"]).strip() or str(row["task_success"]) != "1"
        ]
        if failed:
            print(f"OpenClaw failed cases: {len(failed)} / {len(rows)}", file=sys.stderr)
            sys.exit(1)


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit(130)
