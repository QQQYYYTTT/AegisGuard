from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[3]


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
        except urllib.error.HTTPError as exc:
            payload = exc.read()
            self.send_response(exc.code)
            self.send_header("Content-Type", exc.headers.get("Content-Type") or "application/json")
            self.end_headers()
            self.wfile.write(payload)
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


def main() -> None:
    parser = argparse.ArgumentParser(description="Local OpenAI-compatible proxy using Python urllib.")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=18080)
    parser.add_argument("--env-file", default=str(REPO_ROOT / ".env"))
    args = parser.parse_args()
    load_env_file(Path(args.env_file))
    server = ThreadingHTTPServer((args.host, args.port), ProxyHandler)
    print(f"OpenAI proxy listening on http://{args.host}:{args.port}/v1", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
