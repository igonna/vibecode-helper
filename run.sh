#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_PATH="${CONFIG:-$ROOT_DIR/config.json}"
RUNTIME_DIR="$ROOT_DIR/.codex/runtime"
NETWORK_OVERRIDE="$RUNTIME_DIR/docker-compose.network.yml"
SUPERVISOR_CONFIG="$RUNTIME_DIR/supervisor.config.json"

if [[ ! -f "$CONFIG_PATH" ]]; then
  cp "$ROOT_DIR/example.config.json" "$CONFIG_PATH"
  echo "Created config.json from example.config.json."
  echo "Edit config.json, then run ./run.sh again."
  exit 1
fi

config_value() {
  local key="$1"
  local fallback="$2"
  python3 - "$CONFIG_PATH" "$key" "$fallback" <<'PY'
import json
import sys

with open(sys.argv[1]) as file:
    config = json.load(file)

value = config.get(sys.argv[2], sys.argv[3])
if isinstance(value, (list, dict)):
    print(json.dumps(value))
else:
    print(value)
PY
}

browser_port_from_url() {
  python3 - "$1" <<'PY'
import sys
from urllib.parse import urlparse

url = urlparse(sys.argv[1])
if url.port:
    print(url.port)
elif url.scheme == "https":
    print(443)
else:
    print(80)
PY
}

notifier_json_value() {
  docker compose "${COMPOSE_ARGS[@]}" exec -T sound-notifier python - "$1" <<'PY' 2>/dev/null || true
import json
import sys
import urllib.request

try:
    with urllib.request.urlopen("http://127.0.0.1:8765/status", timeout=1) as response:
        payload = json.load(response)
    print(payload.get(sys.argv[1], ""))
except Exception:
    print("")
PY
}

notifier_post_event() {
  docker compose "${COMPOSE_ARGS[@]}" exec -T sound-notifier python - "$1" <<'PY'
import json
import sys
import urllib.request

data = json.dumps({"event": sys.argv[1]}).encode()
request = urllib.request.Request(
    "http://127.0.0.1:8765/notify",
    data=data,
    headers={"Content-Type": "application/json"},
    method="POST",
)
with urllib.request.urlopen(request, timeout=3) as response:
    response.read()
PY
}

write_supervisor_config() {
  python3 - "$CONFIG_PATH" "$SUPERVISOR_CONFIG" "$ROOT_DIR/docker-compose.yml" "$NETWORK_OVERRIDE" <<'PY'
import json
import os
import sys

source_path, destination_path, compose_file, network_override = sys.argv[1:5]

with open(source_path) as file:
    config = json.load(file)

compose_files = [compose_file]
if os.path.exists(network_override):
    compose_files.append(network_override)

compose_command = ["docker", "compose"]
for path in compose_files:
    compose_command.extend(["-f", path])

health_url = str(config.get("app_health_url", "")).strip()
browser_health = config.get("app_health_from_browser", True)
if health_url and browser_health and "app_health_command" not in config:
    health_script = """
import json
import sys
import urllib.parse
import urllib.request

target_url = sys.argv[1]
timeout = float(sys.argv[2])
query = urllib.parse.urlencode({"url": target_url, "timeout": str(timeout)})
with urllib.request.urlopen(f"http://127.0.0.1:8765/browser-check?{query}", timeout=timeout + 2) as response:
    payload = json.load(response)
print(payload.get("detail", "browser health check finished"))
raise SystemExit(0 if payload.get("ok") else 1)
""".strip()
    timeout = str(config.get("app_health_browser_timeout_seconds", 3))
    config["app_health_command"] = [
        *compose_command,
        "exec",
        "-T",
        "sound-notifier",
        "python",
        "-c",
        health_script,
        health_url,
        timeout,
    ]
    config["app_health_label"] = f"browser checks {health_url}"

if "notify_command" not in config:
    notify_script = """
import json
import sys
import urllib.request

data = json.dumps({"event": sys.argv[1]}).encode()
request = urllib.request.Request(
    "http://127.0.0.1:8765/notify",
    data=data,
    headers={"Content-Type": "application/json"},
    method="POST",
)
with urllib.request.urlopen(request, timeout=3) as response:
    response.read()
""".strip()
    config["notify_command"] = [
        *compose_command,
        "exec",
        "-T",
        "sound-notifier",
        "python",
        "-c",
        notify_script,
        "{event}",
    ]

with open(destination_path, "w") as file:
    json.dump(config, file, indent=2)
    file.write("\n")
PY
}

BROWSER_URL="$(config_value browser_url "")"
if [[ -z "$BROWSER_URL" ]]; then
  BROWSER_URL="http://localhost:$(config_value browser_port "8766")"
fi
BROWSER_PORT="$(browser_port_from_url "$BROWSER_URL")"
RESTART_HOOK_URL="$(config_value restart_hook_url "")"
if [[ -z "$RESTART_HOOK_URL" ]]; then
  RESTART_HOOK_URL="http://127.0.0.1:$(config_value restart_hook_port "8767")/restart"
fi
DOCKER_NETWORK="$(config_value docker_network "$(config_value app_network "")")"
NOTIFY_HOST="$(config_value notify_host "vibecode-dev-notify")"
SOUND_REPEAT_COUNT="$(config_value sound_repeat_count "1")"
SUCCESS_SOUND_URL="$(config_value success_sound_url "https://actions.google.com/sounds/v1/cartoon/wood_plank_flicks.ogg")"
FAILURE_SOUND_URL="$(config_value failure_sound_url "https://actions.google.com/sounds/v1/cartoon/cartoon_boing.ogg")"

export VIBECODE_APP_NETWORK="$DOCKER_NETWORK"
export VIBECODE_BROWSER_PORT="$BROWSER_PORT"
export VIBECODE_BROWSER_URL="$BROWSER_URL"
export VIBECODE_FAILURE_SOUND_URL="$FAILURE_SOUND_URL"
export VIBECODE_NOTIFY_HOST="$NOTIFY_HOST"
export VIBECODE_SOUND_REPEAT_COUNT="$SOUND_REPEAT_COUNT"
export VIBECODE_SUCCESS_SOUND_URL="$SUCCESS_SOUND_URL"

mkdir -p "$RUNTIME_DIR"
COMPOSE_ARGS=(-f "$ROOT_DIR/docker-compose.yml")
NOTIFIER_STARTED=0

cleanup() {
  if [[ "$NOTIFIER_STARTED" == "1" ]]; then
    echo "Stopping browser sound service..."
    docker compose "${COMPOSE_ARGS[@]}" stop sound-notifier >/dev/null 2>&1 || true
  fi
}

trap cleanup EXIT

if [[ -n "$DOCKER_NETWORK" ]] && docker network inspect "$DOCKER_NETWORK" >/dev/null 2>&1; then
  cat > "$NETWORK_OVERRIDE" <<YAML
services:
  sound-notifier:
    networks:
      app-network:
        aliases:
          - $NOTIFY_HOST
  web-monitor:
    networks:
      app-network:

networks:
  app-network:
    name: $DOCKER_NETWORK
    external: true
YAML
  COMPOSE_ARGS+=(-f "$NETWORK_OVERRIDE")
else
  rm -f "$NETWORK_OVERRIDE"
  if [[ -n "$DOCKER_NETWORK" ]]; then
    echo "Docker network '$DOCKER_NETWORK' was not found; browser sounds still work from this machine."
  fi
fi

echo "Starting browser sound service..."
docker compose "${COMPOSE_ARGS[@]}" up -d --build sound-notifier
NOTIFIER_STARTED=1

echo "Add this to Vibecode hook:"
echo "POST $RESTART_HOOK_URL"
echo
echo "Open this in browser and keep it open:"
echo "$BROWSER_URL"

echo "Waiting for browser sound service..."
service_ready=0
for _ in $(seq 1 60); do
  if notifier_json_value ok | grep -q True; then
    service_ready=1
    break
  fi
  sleep 1
done

if [[ "$service_ready" != "1" ]]; then
  echo "Browser sound service did not become ready."
  echo "Check logs with: docker compose logs sound-notifier"
  exit 1
fi

browser_tabs="$(notifier_json_value browser_tabs)"
if [[ "$browser_tabs" != "" ]] && (( browser_tabs > 0 )); then
  echo "Browser already connected."
else
  while true; do
    browser_tabs="$(notifier_json_value browser_tabs)"
    if [[ "$browser_tabs" != "" ]] && (( browser_tabs > 0 )); then
      echo "Browser connected."
      break
    fi
    sleep 1
  done
fi

echo "Playing test sound 1: success."
notifier_post_event success
sleep 2
last_status="$(notifier_json_value last_client_status)"
if [[ "$last_status" == *"blocked"* ]]; then
  echo "Click Enable sound permission in the browser page."
  while true; do
    last_status="$(notifier_json_value last_client_status)"
    if [[ "$last_status" == *"sound enabled"* ]]; then
      break
    fi
    sleep 1
  done
  echo "Sound enabled. Playing test sound 1 again."
  notifier_post_event success
  sleep 2
fi
echo "Playing test sound 2: failure."
notifier_post_event failure
sleep 1

write_supervisor_config
echo "Starting app supervisor. Your app command output appears below."
CONFIG="$SUPERVISOR_CONFIG" python3 "$ROOT_DIR/app.py" supervisor
