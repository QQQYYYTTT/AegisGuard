from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
RESULTS_DIR = REPO_ROOT / "experiments" / "asb" / "results"
MANIFEST_DIR = RESULTS_DIR / "manifests"

ASB_ATTACKS = {
    "dpi": {
        "config": "config/DPI.yml",
        "script": "scripts/agent_attack.py",
        "description": "Direct Prompt Injection",
    },
    "opi": {
        "config": "config/OPI.yml",
        "script": "scripts/agent_attack.py",
        "description": "Observation Prompt Injection",
    },
    "mp": {
        "config": "config/MP.yml",
        "script": "scripts/agent_attack.py",
        "description": "Memory Poisoning",
    },
    "mixed": {
        "config": "config/mixed.yml",
        "script": "scripts/agent_attack.py",
        "description": "Mixed Attack",
    },
    "pot": {
        "config": "config/POT.yml",
        "script": "scripts/agent_attack_pot.py",
        "description": "Plan-of-Thought Backdoor",
    },
}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Launch the original ASB benchmark scripts from the AegisGuard workspace."
    )
    parser.add_argument(
        "--asb-root",
        default=os.environ.get("ASB_ROOT", ""),
        help="Path to a local checkout of https://github.com/agiresearch/ASB. "
        "Can also be set with ASB_ROOT.",
    )
    parser.add_argument("--attack", choices=sorted(ASB_ATTACKS), required=True)
    parser.add_argument(
        "--config",
        default="",
        help="Optional ASB config path relative to --asb-root. Overrides the default config for the attack.",
    )
    parser.add_argument("--run-id", default="", help="Stable id for this ASB run.")
    parser.add_argument(
        "--python",
        default=sys.executable,
        help="Python executable to use inside the ASB checkout.",
    )
    parser.add_argument("--dry-run", action="store_true", help="Print and record the ASB command without running it.")
    return parser.parse_args()


def require_file(path: Path, label: str) -> None:
    if not path.exists():
        raise SystemExit(f"{label} not found: {path}")
    if not path.is_file():
        raise SystemExit(f"{label} is not a file: {path}")


def main() -> None:
    args = parse_args()
    # 如果 ASB 调 OpenAI-compatible endpoint 经过 AegisGuard，
    # 入口鉴权现在推荐使用 X-Gateway-Key；真实上游 Authorization 是否需要同时携带
    # 取决于网关侧 AEGIS_AUTH_MODE（gateway_managed / passthrough）。
    if not args.asb_root:
        raise SystemExit("Missing --asb-root or ASB_ROOT.")

    asb_root = Path(args.asb_root).resolve()
    if not asb_root.exists():
        raise SystemExit(f"ASB root not found: {asb_root}")

    attack = ASB_ATTACKS[args.attack]
    script_path = asb_root / attack["script"]
    require_file(script_path, "ASB script")

    command = [args.python, attack["script"]]
    config_rel = args.config or attack["config"]
    if config_rel:
        config_path = asb_root / config_rel
        require_file(config_path, "ASB config")
        command.extend(["--cfg_path", config_rel])

    timestamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    run_id = args.run_id or f"asb-{args.attack}-{timestamp}"
    MANIFEST_DIR.mkdir(parents=True, exist_ok=True)
    manifest_path = MANIFEST_DIR / f"{run_id}.json"

    manifest = {
        "run_id": run_id,
        "timestamp_utc": datetime.now(timezone.utc).isoformat(),
        "asb_root": str(asb_root),
        "attack": args.attack,
        "attack_description": attack["description"],
        "config": config_rel,
        "script": attack["script"],
        "command": command,
        "dry_run": args.dry_run,
    }

    print("ASB command:")
    print(" ".join(command))
    print(f"Working directory: {asb_root}")

    if args.dry_run:
        manifest["returncode"] = None
    else:
        completed = subprocess.run(command, cwd=asb_root, check=False)
        manifest["returncode"] = completed.returncode
        if completed.returncode != 0:
            manifest_path.write_text(json.dumps(manifest, indent=2, ensure_ascii=False), encoding="utf-8")
            raise SystemExit(completed.returncode)

    manifest_path.write_text(json.dumps(manifest, indent=2, ensure_ascii=False), encoding="utf-8")
    print(f"Manifest written: {manifest_path}")


if __name__ == "__main__":
    main()
