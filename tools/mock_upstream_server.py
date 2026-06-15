import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class MockUpstreamHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path not in ("/", "/health"):
            self.send_error(404)
            return

        self._write_json(
            200,
            {
                "status": "ok",
                "service": "aegisguard-mock-upstream",
            },
        )

    def do_POST(self):
        content_length = int(self.headers.get("Content-Length", "0"))
        raw_body = self.rfile.read(content_length)

        try:
            request_body = json.loads(raw_body or b"{}")
        except json.JSONDecodeError:
            self._write_json(400, {"error": "invalid JSON request body"})
            return

        print(
            f"UPSTREAM RECEIVED: {self.command} {self.path} "
            f"{json.dumps(request_body, ensure_ascii=False)}",
            flush=True,
        )

        self._write_json(
            200,
            {
                "id": "chatcmpl-demo",
                "object": "chat.completion",
                "choices": [
                    {
                        "index": 0,
                        "message": {
                            "role": "assistant",
                            "content": "This is a safe upstream model response.",
                        },
                        "finish_reason": "stop",
                    }
                ],
            },
        )

    def log_message(self, format, *args):
        return

    def _write_json(self, status_code, payload):
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status_code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)


if __name__ == "__main__":
    address = ("127.0.0.1", 18080)
    print(f"Mock upstream listening on http://{address[0]}:{address[1]}", flush=True)
    ThreadingHTTPServer(address, MockUpstreamHandler).serve_forever()
