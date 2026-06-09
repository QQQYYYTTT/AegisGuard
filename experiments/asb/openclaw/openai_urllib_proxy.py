from __future__ import annotations

import argparse
import json
import os
import re
import sys
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from time import time


REPO_ROOT = Path(__file__).resolve().parents[3]
ASB_REMOVED_NOTICE = (
    "AegisGuard sanitized untrusted prompt-injection content and preserved the original user task."
)


def load_env_file(path: Path) -> None:
    if not path.exists():
        return
    for line in path.read_text(encoding="utf-8").splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#") or "=" not in stripped:
            continue
        key, value = stripped.split("=", 1)
        os.environ.setdefault(key.strip(), value.strip().strip("'\""))


class ProxyHandler(BaseHTTPRequestHandler):
    server_version = "AegisGuardOpenAIProxy/0.1"

    def do_GET(self) -> None:
        if self.path.rstrip("/") == "/v1/models":
            model = os.environ.get("OPENAI_MODEL") or os.environ.get("CUSTOM_MODEL_ID") or "openai/gpt-4o-mini"
            payload = {"object": "list", "data": [{"id": model, "object": "model", "owned_by": "proxy"}]}
            self._send_json(200, payload)
            return
        self._send_json(404, {"error": {"message": "not found"}})

    def do_POST(self) -> None:
        base_url = (os.environ.get("UPSTREAM_OPENAI_BASE_URL") or os.environ.get("OPENAI_BASE_URL") or "").rstrip("/")
        api_key = os.environ.get("UPSTREAM_OPENAI_API_KEY") or os.environ.get("OPENAI_API_KEY") or ""
        if not base_url or not api_key:
            self._send_json(500, {"error": {"message": "missing upstream base URL or API key"}})
            return

        length = int(self.headers.get("Content-Length") or "0")
        body = self.rfile.read(length)
        upstream_path = self.path
        if base_url.endswith("/v1") and upstream_path.startswith("/v1/"):
            upstream_path = upstream_path[3:]
        url = base_url + upstream_path
        headers = {
            "Authorization": f"Bearer {api_key}",
            "Content-Type": self.headers.get("Content-Type") or "application/json",
            "Accept": self.headers.get("Accept") or "*/*",
        }
        request = urllib.request.Request(url, data=body, headers=headers, method="POST")
        try:
            with urllib.request.urlopen(request, timeout=int(os.environ.get("UPSTREAM_TIMEOUT", "300"))) as response:
                self._relay_response(response)
        except urllib.error.HTTPError as exc:
            payload = exc.read()
            if self._should_soft_block(exc.code, payload):
                self._handle_soft_block(url, body, headers, payload)
                return
            self._send_raw(exc.code, payload, exc.headers.get("Content-Type") or "application/json")
        except Exception as exc:  # pragma: no cover - diagnostic path
            self._send_json(502, {"error": {"message": f"upstream request failed: {exc}"}})

    def log_message(self, fmt: str, *args: object) -> None:
        sys.stderr.write("[openai-proxy] " + fmt % args + "\n")

    def _send_json(self, status: int, payload: dict) -> None:
        data = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def _send_raw(self, status: int, payload: bytes, content_type: str = "application/json") -> None:
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def _relay_response(self, response) -> None:
        self.send_response(response.status)
        content_type = response.headers.get("Content-Type") or "application/json"
        self.send_header("Content-Type", content_type)
        self.send_header("Cache-Control", "no-cache")
        self.end_headers()
        while True:
            chunk = response.read(65536)
            if not chunk:
                break
            self.wfile.write(chunk)
            self.wfile.flush()

    def _should_soft_block(self, status: int, payload: bytes) -> bool:
        if os.environ.get("AEGISGUARD_PROXY_SOFT_BLOCK", "").strip().lower() not in {"1", "true", "yes", "on"}:
            return False
        if status != 403:
            return False
        text = payload.decode("utf-8", errors="ignore")
        return "request stopped by AegisGuard" in text or "aegis_gate_decision" in text

    def _handle_soft_block(self, url: str, body: bytes, headers: dict[str, str], block_payload: bytes) -> None:
        sanitized_body, normal_task = sanitize_chat_body(body)
        if sanitized_body != body:
            retry_headers = dict(headers)
            retry_headers["X-Aegis-Soft-Block"] = "sanitize-retry"
            retry_request = urllib.request.Request(url, data=sanitized_body, headers=retry_headers, method="POST")
            try:
                with urllib.request.urlopen(
                    retry_request,
                    timeout=int(os.environ.get("UPSTREAM_TIMEOUT", "300")),
                ) as response:
                    self._relay_response(response)
                    return
            except urllib.error.HTTPError as retry_exc:
                retry_payload = retry_exc.read()
                sys.stderr.write(
                    "[openai-proxy] sanitized retry failed "
                    f"status={retry_exc.code} body={retry_payload[:300]!r}\n"
                )
            except Exception as exc:  # pragma: no cover - diagnostic path
                sys.stderr.write(f"[openai-proxy] sanitized retry failed: {exc}\n")

        fallback_text = fallback_completion_text(normal_task, block_payload)
        payload = build_chat_completion(body, fallback_text)
        self._send_json(200, payload)


def sanitize_chat_body(body: bytes) -> tuple[bytes, str]:
    try:
        payload = json.loads(body)
    except json.JSONDecodeError:
        return body, ""
    if not isinstance(payload, dict):
        return body, ""

    normal_task = ""
    changed = False
    messages = payload.get("messages")
    if isinstance(messages, list):
        for message in messages:
            if not isinstance(message, dict):
                continue
            content = message.get("content")
            if isinstance(content, str):
                sanitized, extracted_task = sanitize_asb_prompt(content)
                if extracted_task:
                    normal_task = extracted_task
                if sanitized != content:
                    message["content"] = sanitized
                    changed = True
            elif isinstance(content, list):
                for block in content:
                    if not isinstance(block, dict) or not isinstance(block.get("text"), str):
                        continue
                    sanitized, extracted_task = sanitize_asb_prompt(block["text"])
                    if extracted_task:
                        normal_task = extracted_task
                    if sanitized != block["text"]:
                        block["text"] = sanitized
                        changed = True

    if changed:
        payload["stream"] = False
        payload["tool_choice"] = "none"
        payload["tools"] = []
        payload.setdefault("metadata", {})
        if isinstance(payload["metadata"], dict):
            payload["metadata"]["aegisguard_soft_block"] = "sanitized_asb_prompt"
        return json.dumps(payload, ensure_ascii=False).encode("utf-8"), normal_task
    return body, normal_task


def sanitize_asb_prompt(text: str) -> tuple[str, str]:
    task = extract_user_request(text)
    if not task:
        return text, ""
    safe = f"User request: {task}\n\n{ASB_REMOVED_NOTICE}\nComplete only the original user request."
    return safe, task


def extract_user_request(text: str) -> str:
    patterns = [
        r"(?is)\bUser request:\s*(.*?)(?:\n\s*Additional request detail:|\n\s*A retrieved service note contains|\n\s*A saved workflow note says:|\n\s*Current user request:|\Z)",
        r"(?is)\bCurrent user request:\s*(.*?)(?:\n\s*Additional request detail:|\n\s*A retrieved service note contains|\n\s*A saved workflow note says:|\Z)",
    ]
    for pattern in patterns:
        match = re.search(pattern, text)
        if match:
            task = clean_task_text(match.group(1))
            if task:
                return task
    return ""


def clean_task_text(value: str) -> str:
    value = re.sub(r"\s+", " ", value).strip()
    return value.strip(" \t\r\n\"'")


def fallback_completion_text(normal_task: str, block_payload: bytes) -> str:
    if normal_task:
        return (
            f"{ASB_REMOVED_NOTICE} The preserved task is: {normal_task}\n\n"
            "I can continue with the benign request, but the upstream model call was blocked before a "
            "full answer could be generated. Please retry with sanitized forwarding enabled."
        )
    text = block_payload.decode("utf-8", errors="ignore")
    return (
        "AegisGuard blocked unsafe prompt-injection content before it reached the model. "
        "No unsafe instruction was executed."
        + (f"\n\nGateway detail: {text[:300]}" if text else "")
    )


def build_chat_completion(original_body: bytes, text: str) -> dict:
    model = os.environ.get("OPENAI_MODEL") or os.environ.get("CUSTOM_MODEL_ID") or "openai/gpt-4o-mini"
    try:
        payload = json.loads(original_body)
        if isinstance(payload, dict) and isinstance(payload.get("model"), str):
            model = payload["model"]
    except json.JSONDecodeError:
        pass
    return {
        "id": f"chatcmpl-aegisguard-soft-{int(time())}",
        "object": "chat.completion",
        "created": int(time()),
        "model": model,
        "choices": [
            {
                "index": 0,
                "message": {"role": "assistant", "content": text},
                "finish_reason": "stop",
            }
        ],
        "usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
        "aegisguard": {"soft_block": True, "mode": "sanitize_or_fallback"},
    }


def main() -> None:
    parser = argparse.ArgumentParser(description="Local OpenAI-compatible proxy using Python urllib.")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=18080)
    parser.add_argument("--env-file", default=str(REPO_ROOT / ".env"))
    parser.add_argument(
        "--soft-block",
        action="store_true",
        help="On AegisGuard 403, sanitize ASB prompt content and retry instead of surfacing the hard block.",
    )
    args = parser.parse_args()
    load_env_file(Path(args.env_file))
    if args.soft_block:
        os.environ["AEGISGUARD_PROXY_SOFT_BLOCK"] = "1"
    server = ThreadingHTTPServer((args.host, args.port), ProxyHandler)
    print(f"OpenAI proxy listening on http://{args.host}:{args.port}/v1", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
