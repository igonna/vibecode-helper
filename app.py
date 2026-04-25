import json
import os
import queue
import sys
import threading
import time
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


DEFAULT_TARGET_URLS = "http://host.docker.internal:3000,http://platform-proxy:3000"
TARGET_URLS = [
    url.strip()
    for url in os.getenv("TARGET_URLS", os.getenv("TARGET_URL", DEFAULT_TARGET_URLS)).split(",")
    if url.strip()
]
CHECK_INTERVAL_SECONDS = float(os.getenv("CHECK_INTERVAL_SECONDS", "3"))
REQUEST_TIMEOUT_SECONDS = float(os.getenv("REQUEST_TIMEOUT_SECONDS", "2"))
NOTIFY_URL = os.getenv("NOTIFY_URL", "http://sound-notifier:8765/notify")

HOST = os.getenv("HOST_NOTIFY_BIND", "0.0.0.0")
PORT = int(os.getenv("HOST_NOTIFY_PORT", "8765"))
PUBLIC_BROWSER_PORT = os.getenv("BROWSER_PORT", str(PORT))
SUCCESS_SOUND_URL = os.getenv(
    "SUCCESS_SOUND_URL",
    os.getenv(
        "SOUND_URL",
        "https://actions.google.com/sounds/v1/cartoon/wood_plank_flicks.ogg",
    ),
)
FAILURE_SOUND_URL = os.getenv(
    "FAILURE_SOUND_URL",
    "https://actions.google.com/sounds/v1/cartoon/wood_plank_flicks.ogg",
)
SOUND_CACHE_DIR = Path(os.getenv("SOUND_CACHE_DIR", "/cache"))
SOUND_REPEAT_COUNT = max(1, int(os.getenv("SOUND_REPEAT_COUNT", "3")))

SUBSCRIBERS: list[queue.Queue[str]] = []
SUBSCRIBERS_LOCK = threading.Lock()

INDEX_HTML = f"""<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Recovery Sound</title>
  <style>
    :root {{
      color-scheme: light dark;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }}
    body {{
      min-height: 100vh;
      margin: 0;
      display: grid;
      place-items: center;
      background: #101318;
      color: #f5f7fa;
    }}
    main {{
      width: min(520px, calc(100vw - 32px));
      display: grid;
      gap: 18px;
    }}
    h1 {{
      margin: 0;
      font-size: 28px;
      font-weight: 650;
    }}
    p {{
      margin: 0;
      color: #bcc5d1;
      line-height: 1.45;
    }}
    button {{
      width: fit-content;
      border: 0;
      border-radius: 6px;
      padding: 11px 16px;
      background: #2f80ed;
      color: white;
      font: inherit;
      font-weight: 600;
      cursor: pointer;
    }}
    .status {{
      min-height: 24px;
      font-size: 14px;
      color: #8fd19e;
    }}
  </style>
</head>
<body>
  <main>
    <h1>Recovery Sound</h1>
    <p>Keep this page open. Recovery notifications play here.</p>
    <button id="enable" hidden>Enable sound permission</button>
    <div id="status" class="status">Connecting.</div>
    <audio id="successSound" preload="auto" src="/sound/success"></audio>
    <audio id="failureSound" preload="auto" src="/sound/failure"></audio>
  </main>
  <script>
    const button = document.getElementById("enable");
    const status = document.getElementById("status");
    const sounds = {{
      success: document.getElementById("successSound"),
      failure: document.getElementById("failureSound"),
    }};
    let enabled = localStorage.getItem("soundEnabled") === "true";
    let playing = Promise.resolve();

    function showEnableButton() {{
      button.hidden = false;
      button.disabled = false;
      button.textContent = "Enable sound permission";
    }}

    function hideEnableButton() {{
      button.hidden = true;
      button.disabled = true;
      button.textContent = "Enable sound permission";
    }}

    function delay(ms) {{
      return new Promise((resolve) => setTimeout(resolve, ms));
    }}

    function playOnce(audio) {{
      return new Promise((resolve, reject) => {{
        audio.pause();
        audio.currentTime = 0;
        audio.onended = resolve;
        audio.onerror = reject;
        audio.play().catch(reject);
      }});
    }}

    async function playNotificationSound(eventName) {{
      if (!enabled) {{
        status.textContent = "Sound permission required.";
        showEnableButton();
        return;
      }}

      const audio = sounds[eventName] || sounds.success;
      status.textContent = `Playing ${{eventName}} sound.`;
      for (let index = 0; index < {SOUND_REPEAT_COUNT}; index += 1) {{
        await playOnce(audio);
        await delay(150);
      }}
      status.textContent = "Sound enabled.";
    }}

    button.addEventListener("click", async () => {{
      try {{
        sounds.success.volume = 1;
        sounds.failure.volume = 1;
        await playOnce(sounds.success);
        enabled = true;
        localStorage.setItem("soundEnabled", "true");
        hideEnableButton();
        status.textContent = "Sound enabled.";
      }} catch (error) {{
        status.textContent = "Sound permission required.";
        showEnableButton();
      }}
    }});

    async function restoreSound() {{
      if (!enabled) {{
        status.textContent = "Sound permission required.";
        showEnableButton();
        return;
      }}

      hideEnableButton();

      try {{
        await sounds.success.play();
        sounds.success.pause();
        sounds.success.currentTime = 0;
        status.textContent = "Sound enabled.";
      }} catch (error) {{
        enabled = false;
        showEnableButton();
        status.textContent = "Sound permission required.";
      }}
    }}

    function connectEvents() {{
      const events = new EventSource("/events");

      events.addEventListener("open", () => {{
        if (enabled) {{
          status.textContent = "Sound enabled.";
        }}
      }});

      for (const eventName of ["success", "failure"]) {{
        events.addEventListener(eventName, () => {{
          playing = playing.then(() => playNotificationSound(eventName)).catch(() => {{
            status.textContent = "Sound permission required.";
            showEnableButton();
          }});
        }});
      }}

      events.onerror = () => {{
        status.textContent = "Reconnecting to notifier.";
      }};
    }}

    restoreSound();
    connectEvents();
  </script>
</body>
</html>
"""


def is_up(url: str) -> tuple[bool, str]:
    request = urllib.request.Request(url, method="GET")

    try:
        with urllib.request.urlopen(request, timeout=REQUEST_TIMEOUT_SECONDS) as response:
            return response.status < 500, f"HTTP {response.status}"
    except urllib.error.HTTPError as exc:
        return exc.code < 500, f"HTTP {exc.code}"
    except (urllib.error.URLError, TimeoutError, OSError) as exc:
        return False, str(exc)


def any_target_up() -> tuple[bool, str]:
    failures = []

    for url in TARGET_URLS:
        up, detail = is_up(url)
        if up:
            return True, f"{url} ({detail})"
        failures.append(f"{url} ({detail})")

    return False, "; ".join(failures)


def notify_success() -> None:
    request = urllib.request.Request(
        NOTIFY_URL,
        data=b'{"event":"success"}',
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    try:
        with urllib.request.urlopen(request, timeout=REQUEST_TIMEOUT_SECONDS) as response:
            response.read()
    except (urllib.error.URLError, TimeoutError, OSError) as exc:
        print(f"Could not notify browser sound bridge: {exc}", flush=True)


def run_monitor() -> None:
    was_up = None

    print(
        f"Monitoring {', '.join(TARGET_URLS)} every {CHECK_INTERVAL_SECONDS:g}s; "
        f"browser sound bridge is {NOTIFY_URL}",
        flush=True,
    )

    while True:
        currently_up, detail = any_target_up()

        if was_up is None:
            print(
                f"Initial state: {'UP' if currently_up else 'DOWN'}; {detail}",
                flush=True,
            )
        elif was_up and not currently_up:
            print(f"State changed: UP -> DOWN; {detail}", flush=True)
        elif not was_up and currently_up:
            print(
                f"State changed: DOWN -> UP via {detail}; triggering browser sound",
                flush=True,
            )
            notify_success()

        was_up = currently_up
        time.sleep(CHECK_INTERVAL_SECONDS)


def sound_url_for_event(event: str) -> str:
    if event == "failure":
        return FAILURE_SOUND_URL
    return SUCCESS_SOUND_URL


def cached_sound_file(event: str) -> Path:
    sound_url = sound_url_for_event(event)
    SOUND_CACHE_DIR.mkdir(parents=True, exist_ok=True)
    destination = SOUND_CACHE_DIR / event / Path(sound_url).name

    if not destination.exists() or destination.stat().st_size == 0:
        destination.parent.mkdir(parents=True, exist_ok=True)
        print(f"Downloading sound to {destination}", flush=True)
        urllib.request.urlretrieve(sound_url, destination)

    return destination


def broadcast(event: str) -> None:
    with SUBSCRIBERS_LOCK:
        subscribers = list(SUBSCRIBERS)

    for subscriber in subscribers:
        subscriber.put(event)


class NotifierHandler(BaseHTTPRequestHandler):
    def do_GET(self) -> None:
        if self.path == "/":
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.end_headers()
            self.wfile.write(INDEX_HTML.encode("utf-8"))
            return

        if self.path in ("/sound", "/sound/success", "/sound/failure"):
            event = "failure" if self.path == "/sound/failure" else "success"
            sound_file = cached_sound_file(event)
            self.send_response(200)
            self.send_header("Content-Type", "audio/ogg")
            self.send_header("Content-Length", str(sound_file.stat().st_size))
            self.end_headers()
            with sound_file.open("rb") as file:
                self.wfile.write(file.read())
            return

        if self.path == "/events":
            self.handle_events()
            return

        self.send_response(404)
        self.end_headers()

    def do_POST(self) -> None:
        if self.path != "/notify":
            self.send_response(404)
            self.end_headers()
            return

        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length) if length else b"{}"

        try:
            payload = json.loads(body)
        except json.JSONDecodeError:
            payload = {}

        event = payload.get("event")
        if event in ("success", "failure"):
            cached_sound_file(event)
            broadcast(event)
            self.send_response(204)
            self.end_headers()
            return

        self.send_response(400)
        self.end_headers()

    def handle_events(self) -> None:
        subscriber: queue.Queue[str] = queue.Queue()

        with SUBSCRIBERS_LOCK:
            SUBSCRIBERS.append(subscriber)

        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "keep-alive")
        self.end_headers()

        try:
            self.wfile.write(b": connected\n\n")
            self.wfile.flush()

            while True:
                event = subscriber.get()
                self.wfile.write(f"event: {event}\ndata: {{}}\n\n".encode("utf-8"))
                self.wfile.flush()
        except (BrokenPipeError, ConnectionResetError):
            pass
        finally:
            with SUBSCRIBERS_LOCK:
                if subscriber in SUBSCRIBERS:
                    SUBSCRIBERS.remove(subscriber)

    def log_message(self, format: str, *args: object) -> None:
        print(f"{self.address_string()} - {format % args}", flush=True)


def run_notifier() -> None:
    server = ThreadingHTTPServer((HOST, PORT), NotifierHandler)
    print(
        f"Browser sound bridge listening on http://localhost:{PUBLIC_BROWSER_PORT}; "
        f"sound repeats {SOUND_REPEAT_COUNT} time(s)",
        flush=True,
    )
    server.serve_forever()


def main() -> None:
    mode = sys.argv[1] if len(sys.argv) > 1 else "monitor"

    if mode == "monitor":
        run_monitor()
    elif mode == "notifier":
        run_notifier()
    else:
        raise SystemExit(f"Unknown mode: {mode}")


if __name__ == "__main__":
    main()
