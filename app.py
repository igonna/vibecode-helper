import json
import os
import queue
import shlex
import signal
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


APP_VERSION = "2026-04-25-single-runner-v4"
CONFIG_PATH = Path(os.getenv("CONFIG", "config.json"))


def load_config() -> dict:
    if CONFIG_PATH.exists():
        with CONFIG_PATH.open() as file:
            return json.load(file)
    return {}


CONFIG = load_config()


ENV_ALIASES = {
    "app_network": "VIBECODE_APP_NETWORK",
    "browser_port": "VIBECODE_BROWSER_PORT",
    "browser_url": "VIBECODE_BROWSER_URL",
    "failure_sound_url": "VIBECODE_FAILURE_SOUND_URL",
    "notify_host": "VIBECODE_NOTIFY_HOST",
    "notify_url": "VIBECODE_NOTIFY_URL",
    "sound_repeat_count": "VIBECODE_SOUND_REPEAT_COUNT",
    "success_sound_url": "VIBECODE_SUCCESS_SOUND_URL",
}


def config_value(name: str, default):
    if name in CONFIG:
        return CONFIG[name]

    env_name = ENV_ALIASES.get(name, name.upper())
    env_value = os.getenv(env_name)
    if env_value is not None and env_value != "":
        return env_value

    return default


def config_urls(name: str, default: list[str]) -> list[str]:
    value = CONFIG.get(name, default)
    if isinstance(value, str):
        value = [value]
    return [str(url).strip() for url in value if str(url).strip()]


TARGET_URLS = config_urls("target_urls", [str(config_value("app_health_url", "http://localhost:3000"))])
CHECK_INTERVAL_SECONDS = float(config_value("check_interval_seconds", 3))
REQUEST_TIMEOUT_SECONDS = float(config_value("request_timeout_seconds", 2))
NOTIFY_HOST = str(config_value("notify_host", "vibecode-dev-notify"))
NOTIFY_URL = str(config_value("notify_url", f"http://{NOTIFY_HOST}:8765/notify"))

NOTIFIER_BIND_HOST = str(config_value("notifier_host", "0.0.0.0"))
PORT = int(config_value("container_port", 8765))
PUBLIC_BROWSER_URL = str(config_value("browser_url", "http://localhost:8766"))
APP_NETWORK = str(config_value("app_network", config_value("docker_network", "repo_default")))
SUCCESS_SOUND_URL = str(
    config_value(
        "success_sound_url",
        "https://actions.google.com/sounds/v1/cartoon/wood_plank_flicks.ogg",
    )
)
FAILURE_SOUND_URL = str(
    config_value(
        "failure_sound_url",
        "https://actions.google.com/sounds/v1/cartoon/cartoon_boing.ogg",
    )
)
SOUND_CACHE_DIR = Path(config_value("sound_cache_dir", "/cache"))
SOUND_REPEAT_COUNT = max(1, int(config_value("sound_repeat_count", 1)))
SUPERVISOR_PROCESSES: list[subprocess.Popen] = []
SUPERVISOR_LOCK = threading.Lock()

SUBSCRIBERS: list[queue.Queue[str]] = []
SUBSCRIBERS_LOCK = threading.Lock()
PENDING_EVENTS: list[str] = []
LAST_CLIENT_STATUS = "waiting for browser"

INDEX_HTML = f"""<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Vibecode Dev Notify</title>
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
    <h1>Vibecode Dev Notify</h1>
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

    function reportClientStatus(eventName, state, detail = "") {{
      fetch("/client-log", {{
        method: "POST",
        headers: {{"Content-Type": "application/json"}},
        body: JSON.stringify({{event: eventName, state, detail}}),
      }}).catch(() => {{}});
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
      reportClientStatus(eventName, "received");

      if (!enabled) {{
        status.textContent = "Sound permission required.";
        showEnableButton();
        reportClientStatus(eventName, "blocked", "sound permission required");
        return;
      }}

      const audio = sounds[eventName] || sounds.success;
      status.textContent = `Playing ${{eventName}} sound.`;
      for (let index = 0; index < {SOUND_REPEAT_COUNT}; index += 1) {{
        await playOnce(audio);
        await delay(150);
      }}
      status.textContent = "Sound enabled.";
      reportClientStatus(eventName, "played");
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
        reportClientStatus("sound", "enabled");
      }} catch (error) {{
        status.textContent = "Sound permission required.";
        showEnableButton();
        reportClientStatus("sound", "blocked", "enable button click failed");
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
        reportClientStatus("sound", "enabled");
      }} catch (error) {{
        enabled = false;
        showEnableButton();
        status.textContent = "Sound permission required.";
        reportClientStatus("sound", "blocked", "browser requires a click");
      }}
    }}

    function connectEvents() {{
      const events = new EventSource("/events");

      events.addEventListener("open", () => {{
        if (enabled) {{
          status.textContent = "Sound enabled.";
        }}
        reportClientStatus("events", "connected");
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
    notify_sound("success")


def notify_sound(event: str, url: str = NOTIFY_URL) -> None:
    data = json.dumps({"event": event}).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    try:
        with urllib.request.urlopen(request, timeout=REQUEST_TIMEOUT_SECONDS) as response:
            response.read()
    except (urllib.error.URLError, TimeoutError, OSError) as exc:
        print(f"Could not send {event} sound notification to {url}: {exc}", flush=True)


def run_monitor() -> None:
    was_up = None

    print(f"vibecode-dev-notify version {APP_VERSION}", flush=True)
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

        if not subscribers:
            PENDING_EVENTS.append(event)
            print(
                f"Queued {event} sound; no browser tab is connected to /events yet",
                flush=True,
            )
            return

    print(f"Broadcasting {event} sound to {len(subscribers)} browser tab(s)", flush=True)

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

        if self.path == "/health":
            body = json.dumps({"ok": True, "version": APP_VERSION}).encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return

        if self.path == "/status":
            with SUBSCRIBERS_LOCK:
                browser_tabs = len(SUBSCRIBERS)
                pending_events = len(PENDING_EVENTS)
            body = json.dumps(
                {
                    "ok": True,
                    "version": APP_VERSION,
                    "browser_tabs": browser_tabs,
                    "pending_events": pending_events,
                    "last_client_status": LAST_CLIENT_STATUS,
                }
            ).encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
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
        if self.path == "/client-log":
            self.handle_client_log()
            return

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

    def handle_client_log(self) -> None:
        global LAST_CLIENT_STATUS

        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length) if length else b"{}"

        try:
            payload = json.loads(body)
        except json.JSONDecodeError:
            payload = {}

        event = payload.get("event", "unknown")
        state = payload.get("state", "unknown")
        detail = payload.get("detail", "")
        suffix = f": {detail}" if detail else ""
        LAST_CLIENT_STATUS = f"{event} {state}{suffix}"
        print(f"Browser status: {LAST_CLIENT_STATUS}", flush=True)

        self.send_response(204)
        self.end_headers()

    def handle_events(self) -> None:
        subscriber: queue.Queue[str] = queue.Queue()

        with SUBSCRIBERS_LOCK:
            SUBSCRIBERS.append(subscriber)
            pending_events = list(PENDING_EVENTS)
            PENDING_EVENTS.clear()

        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "keep-alive")
        self.end_headers()

        try:
            self.wfile.write(b": connected\n\n")
            self.wfile.flush()
            print(
                f"Browser event stream connected; {len(SUBSCRIBERS)} tab(s) connected",
                flush=True,
            )

            for event in pending_events:
                subscriber.put(event)

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
                print(
                    f"Browser event stream disconnected; {len(SUBSCRIBERS)} tab(s) connected",
                    flush=True,
                )

    def log_message(self, format: str, *args: object) -> None:
        print(f"{self.address_string()} - {format % args}", flush=True)


def run_notifier() -> None:
    server = ThreadingHTTPServer((NOTIFIER_BIND_HOST, PORT), NotifierHandler)
    print(f"vibecode-dev-notify version {APP_VERSION}", flush=True)
    print(
        "Start command: ./run.sh",
        flush=True,
    )
    print(
        f"Browser sound bridge listening on {PUBLIC_BROWSER_URL}; "
        f"sound repeats {SOUND_REPEAT_COUNT} time(s)",
        flush=True,
    )
    print(
        "Trigger from the host with: "
        f"curl -X POST {PUBLIC_BROWSER_URL}/notify "
        "-H 'Content-Type: application/json' -d '{\"event\":\"success\"}'",
        flush=True,
    )
    print(
        f"Trigger from containers attached to Docker network '{APP_NETWORK}' with: "
        f"curl -X POST http://{NOTIFY_HOST}:8765/notify "
        "-H 'Content-Type: application/json' -d '{\"event\":\"failure\"}'",
        flush=True,
    )
    print(
        "If that hostname does not resolve, attach the caller container to that "
        f"network or test with: docker run --rm --network {APP_NETWORK} "
        f"curlimages/curl -X POST http://{NOTIFY_HOST}:8765/notify "
        "-H 'Content-Type: application/json' -d '{\"event\":\"failure\"}'",
        flush=True,
    )
    server.serve_forever()


def load_supervisor_config() -> dict:
    if not CONFIG:
        raise SystemExit(f"Missing config file: {CONFIG_PATH}")
    return CONFIG


def normalize_command(command: str | list[str]) -> list[str]:
    if isinstance(command, str):
        return shlex.split(command)
    return [str(part) for part in command]


def supervisor_config_value(config: dict, names: tuple[str, ...], default=None):
    for name in names:
        if name in config:
            return config[name]
    return default


def configured_commands(config: dict) -> list[list[str]]:
    if "app_start_commands" in config:
        commands = config["app_start_commands"]
    elif "app_start_command" in config:
        commands = [config["app_start_command"]]
    elif "commands" in config:
        commands = config["commands"]
    elif "command" in config:
        commands = [config["command"]]
    else:
        raise SystemExit("config.json requires `app_start_command`")

    return [normalize_command(command) for command in commands]


def configured_health_url(config: dict) -> str:
    health_url = str(
        supervisor_config_value(config, ("app_health_url", "health_url"), "")
    ).strip()
    if not health_url:
        raise SystemExit("config.json requires `app_health_url` or `app_health_command`")
    return health_url


def configured_health_command(config: dict) -> list[str] | None:
    command = supervisor_config_value(config, ("app_health_command", "health_command"))
    if command is None:
        return None
    return normalize_command(command)


def configured_notify_url(config: dict) -> str:
    notify_url = supervisor_config_value(config, ("notify_url",))
    if notify_url:
        return str(notify_url)

    browser_url = str(supervisor_config_value(config, ("browser_url",), PUBLIC_BROWSER_URL)).rstrip("/")
    return f"{browser_url}/notify"


def supervisor_notify(config: dict, event: str) -> None:
    notify_url = configured_notify_url(config)
    if not notify_url:
        return
    notify_sound(event, notify_url)


def start_supervised_commands(config: dict) -> None:
    global SUPERVISOR_PROCESSES

    cwd = Path(supervisor_config_value(config, ("app_cwd", "cwd"), ".")).resolve()
    SUPERVISOR_PROCESSES = []

    for command in configured_commands(config):
        print("\n=== Starting command ===", flush=True)
        print(" ".join(command), flush=True)
        SUPERVISOR_PROCESSES.append(
            subprocess.Popen(
                command,
                stdin=None,
                stdout=None,
                stderr=None,
                cwd=cwd,
                start_new_session=True,
            )
        )


def stop_supervised_commands(config: dict) -> None:
    global SUPERVISOR_PROCESSES

    stop_timeout = float(
        supervisor_config_value(config, ("app_stop_timeout_seconds", "stop_timeout_seconds"), 15)
    )
    kill_timeout = float(
        supervisor_config_value(config, ("app_kill_timeout_seconds", "kill_timeout_seconds"), 5)
    )

    for process in SUPERVISOR_PROCESSES:
        if process.poll() is not None:
            continue

        print("\n=== Stopping command ===", flush=True)
        try:
            os.killpg(os.getpgid(process.pid), signal.SIGINT)
            process.wait(timeout=stop_timeout)
        except subprocess.TimeoutExpired:
            print("=== SIGINT timeout, terminating ===", flush=True)
            os.killpg(os.getpgid(process.pid), signal.SIGTERM)
            try:
                process.wait(timeout=kill_timeout)
            except subprocess.TimeoutExpired:
                print("=== SIGTERM timeout, killing ===", flush=True)
                os.killpg(os.getpgid(process.pid), signal.SIGKILL)
                process.wait()

    SUPERVISOR_PROCESSES = []


def health_command_ready(config: dict, command: list[str]) -> tuple[bool, str]:
    cwd = Path(supervisor_config_value(config, ("app_cwd", "cwd"), ".")).resolve()
    timeout = float(
        supervisor_config_value(
            config,
            ("app_health_command_timeout_seconds", "health_command_timeout_seconds"),
            REQUEST_TIMEOUT_SECONDS,
        )
    )

    try:
        result = subprocess.run(
            command,
            cwd=cwd,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            timeout=timeout,
            check=False,
        )
    except subprocess.TimeoutExpired:
        return False, f"{' '.join(command)} (timed out after {timeout:g}s)"

    output = " ".join(result.stdout.split())
    if output:
        output = f"; {output[:240]}"

    return result.returncode == 0, f"{' '.join(command)} (exit {result.returncode}{output})"


def health_ready(config: dict) -> tuple[bool, str]:
    health_command = configured_health_command(config)
    if health_command:
        return health_command_ready(config, health_command)

    health_url = configured_health_url(config)
    up, detail = is_up(health_url)
    return up, f"{health_url} ({detail})"


def wait_for_health(config: dict) -> tuple[bool, str]:
    timeout = float(
        supervisor_config_value(
            config,
            ("app_startup_timeout_seconds", "startup_timeout_seconds"),
            60,
        )
    )
    interval = float(
        supervisor_config_value(
            config,
            ("app_health_interval_seconds", "check_interval_seconds"),
            2,
        )
    )
    deadline = time.monotonic() + timeout
    last_detail = "not checked"

    while time.monotonic() <= deadline:
        ready, detail = health_ready(config)
        last_detail = detail
        print(f"Health check: {'UP' if ready else 'DOWN'}; {detail}", flush=True)
        if ready:
            return True, detail
        time.sleep(interval)

    return False, last_detail


def restart_and_check(config: dict) -> None:
    with SUPERVISOR_LOCK:
        stop_supervised_commands(config)
        time.sleep(
            float(supervisor_config_value(config, ("app_restart_delay_seconds", "restart_delay_seconds"), 1))
        )
        start_supervised_commands(config)
        check_supervised_start(config, "Restart")


def check_supervised_start(config: dict, label: str) -> None:
    ready, detail = wait_for_health(config)

    if ready:
        print(f"{label} result: success; {detail}", flush=True)
        supervisor_notify(config, "success")
    else:
        print(f"{label} result: failure; {detail}", flush=True)
        supervisor_notify(config, "failure")


def make_supervisor_handler(config: dict):
    class SupervisorHandler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:
            if self.path != "/restart":
                self.send_response(404)
                self.end_headers()
                return

            threading.Thread(target=restart_and_check, args=(config,), daemon=True).start()
            self.send_response(202)
            self.end_headers()
            self.wfile.write(b"Restarting\n")

        def log_message(self, fmt: str, *args: object) -> None:
            return

    return SupervisorHandler


def run_supervisor() -> None:
    config = load_supervisor_config()
    host = supervisor_config_value(config, ("restart_hook_host", "host"), "127.0.0.1")
    port = int(supervisor_config_value(config, ("restart_hook_port", "port"), 8767))
    cwd = Path(supervisor_config_value(config, ("app_cwd", "cwd"), ".")).resolve()
    notify_url = configured_notify_url(config)

    print(f"vibecode-dev-notify version {APP_VERSION}", flush=True)
    print(f"Supervisor listening on http://{host}:{port}", flush=True)
    print("Starting command(s), waiting for health check, then playing success/failure", flush=True)
    print("POST /restart to repeat the same restart/check flow", flush=True)
    print(f"CWD: {cwd}", flush=True)
    print(f"Commands: {len(configured_commands(config))}", flush=True)
    health_command = configured_health_command(config)
    if health_command:
        print(f"Health command: {' '.join(health_command)}", flush=True)
    else:
        print(f"Health URL: {configured_health_url(config)}", flush=True)
    print(
        "Startup timeout: "
        f"{supervisor_config_value(config, ('app_startup_timeout_seconds', 'startup_timeout_seconds'), 60)}s",
        flush=True,
    )
    print(f"Sound notifications: {notify_url}", flush=True)

    start_supervised_commands(config)
    check_supervised_start(config, "Initial start")
    server = ThreadingHTTPServer((host, port), make_supervisor_handler(config))

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n=== Supervisor stopping ===", flush=True)
    finally:
        stop_supervised_commands(config)
        server.server_close()


def main() -> None:
    mode = sys.argv[1] if len(sys.argv) > 1 else "monitor"

    if mode == "monitor":
        run_monitor()
    elif mode == "notifier":
        run_notifier()
    elif mode == "supervisor":
        run_supervisor()
    else:
        raise SystemExit(f"Unknown mode: {mode}")


if __name__ == "__main__":
    main()
