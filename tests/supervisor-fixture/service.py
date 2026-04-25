import os
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


HOST = "0.0.0.0"
PORT = int(os.getenv("PORT", "3000"))
STARTUP_DELAY_SECONDS = float(os.getenv("STARTUP_DELAY_SECONDS", "0"))


class Handler(BaseHTTPRequestHandler):
    def do_GET(self) -> None:
        if self.path == "/health":
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"OK\n")
            return

        self.send_response(404)
        self.end_headers()

    def log_message(self, format: str, *args: object) -> None:
        print(f"{self.address_string()} - {format % args}", flush=True)


if STARTUP_DELAY_SECONDS:
    print(f"Delaying startup for {STARTUP_DELAY_SECONDS:g}s", flush=True)
    time.sleep(STARTUP_DELAY_SECONDS)

print(f"Fixture service listening on http://{HOST}:{PORT}", flush=True)
ThreadingHTTPServer((HOST, PORT), Handler).serve_forever()
