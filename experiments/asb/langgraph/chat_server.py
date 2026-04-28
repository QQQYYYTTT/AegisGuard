from __future__ import annotations

import argparse
import json
import os
import sys
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from types import SimpleNamespace
from urllib.parse import urlparse


REPO_ROOT = Path(__file__).resolve().parents[3]
ASB_ROOT = REPO_ROOT / "ASB"
CHAT_PAGE = REPO_ROOT / "frontend" / "langgraph-financial-chat.html"


def ensure_import_path() -> None:
    if str(ASB_ROOT) not in sys.path:
        sys.path.insert(0, str(ASB_ROOT))
    os.chdir(ASB_ROOT)


def clean_args() -> SimpleNamespace:
    return SimpleNamespace(
        agent_backend="pyopenagi",
        llm_name=os.getenv("LANGGRAPH_OPENAI_MODEL") or os.getenv("CUSTOM_MODEL_ID") or os.getenv("OPENAI_MODEL") or "gpt-4o-mini",
        defense_type=None,
        attack_type="naive",
        direct_prompt_injection=False,
        observation_prompt_injection=False,
        memory_attack=False,
        pot_backdoor=False,
        pot_clean=False,
        clean=True,
        read_db=False,
        write_db=False,
        trigger="",
        tools_info_path="data/all_normal_tools.jsonl",
        database="memory_db/chroma_db",
        openclaw_bin="",
        openclaw_cwd="",
        openclaw_timeout=180,
        openclaw_gateway=False,
    )


def run_financial_agent(message: str) -> dict:
    ensure_import_path()

    from pyopenagi.agents.agent_factory import AgentFactory
    from pyopenagi.agents.agent_process import AgentProcessFactory

    args = clean_args()
    attacker_tool = {
        "Attacker Tool": "CleanBaselinePlaceholder",
        "Attacker Instruction": "",
        "Description": "Clean baseline placeholder. No attack instruction is injected.",
        "Attack goal": "",
        "Aggressive": "False",
    }

    factory = AgentFactory(
        agent_process_queue=None,
        agent_process_factory=AgentProcessFactory(),
        agent_log_mode="console",
    )
    started = time.time()
    result = factory.run_agent(
        "example/langgraph_financial_agent",
        message,
        args,
        attacker_tool,
        None,
        "False",
    )
    duration_ms = int((time.time() - started) * 1000)

    messages = result.get("messages", [])
    final_answer = ""
    workflow = ""
    actions = []
    for item in messages:
        content = item.get("content") or item.get("thinking") or ""
        if "[Action]:" in content:
            actions.append(content)
        if "[Thinking]: The workflow generated" in content:
            workflow = content
        elif item.get("role") == "assistant" and content and not content.startswith("[Action]:") and not content.startswith("[Thinking]:"):
            final_answer = content

    return {
        "agent": "langgraph_financial_agent",
        "mode": "clean-baseline",
        "defense": "none",
        "llm": args.llm_name,
        "duration_ms": duration_ms,
        "answer": final_answer,
        "actions": actions,
        "workflow": workflow,
        "raw_messages": messages,
    }


class Handler(BaseHTTPRequestHandler):
    server_version = "LangGraphFinancialChat/0.1"

    def _send_json(self, payload: dict, status: int = 200) -> None:
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _send_html(self) -> None:
        body = CHAT_PAGE.read_bytes()
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self) -> None:
        path = urlparse(self.path).path
        if path in {"/", "/chat"}:
            self._send_html()
            return
        if path == "/api/health":
            self._send_json({
                "ok": True,
                "agent": "langgraph_financial_agent",
                "mode": "clean-baseline",
                "defense": "none",
            })
            return
        self._send_json({"error": "not found"}, status=404)

    def do_POST(self) -> None:
        path = urlparse(self.path).path
        if path != "/api/chat":
            self._send_json({"error": "not found"}, status=404)
            return

        try:
            length = int(self.headers.get("Content-Length", "0"))
            payload = json.loads(self.rfile.read(length).decode("utf-8"))
            message = str(payload.get("message", "")).strip()
            if not message:
                self._send_json({"error": "message is required"}, status=400)
                return
            self._send_json(run_financial_agent(message))
        except Exception as exc:  # noqa: BLE001
            self._send_json({"error": f"{exc.__class__.__name__}: {exc}"}, status=500)


def main() -> int:
    parser = argparse.ArgumentParser(description="Run a clean/no-defense LangGraph financial-agent chat UI.")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=8765)
    args = parser.parse_args()

    ensure_import_path()
    server = ThreadingHTTPServer((args.host, args.port), Handler)
    print(f"LangGraph financial-agent chat running at http://{args.host}:{args.port}/chat", flush=True)
    server.serve_forever()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
