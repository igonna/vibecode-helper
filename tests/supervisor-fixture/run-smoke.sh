#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LOG_DIR="$ROOT/tests/supervisor-fixture/.logs"
mkdir -p "$LOG_DIR"
cd "$ROOT"

cleanup() {
  docker compose -f "$ROOT/tests/supervisor-fixture/docker-compose.yml" down --remove-orphans >/dev/null 2>&1 || true
  if [[ -n "${SUPERVISOR_PID:-}" ]]; then
    kill "$SUPERVISOR_PID" >/dev/null 2>&1 || true
    wait "$SUPERVISOR_PID" >/dev/null 2>&1 || true
  fi
}

wait_for_log() {
  local pattern="$1"
  local file="$2"
  local deadline=$((SECONDS + 45))

  while (( SECONDS < deadline )); do
    if grep -q "$pattern" "$file"; then
      return 0
    fi
    sleep 1
  done

  echo "Timed out waiting for '$pattern' in $file" >&2
  tail -n 80 "$file" >&2 || true
  return 1
}

trap cleanup EXIT

SUCCESS_LOG="$LOG_DIR/success.log"
CONFIG="$ROOT/tests/supervisor-fixture/supervisor-success.json" \
  python3 "$ROOT/app.py" supervisor >"$SUCCESS_LOG" 2>&1 &
SUPERVISOR_PID="$!"
wait_for_log "Initial start result: success" "$SUCCESS_LOG"
curl -fsS -X POST http://127.0.0.1:18767/restart >/dev/null
wait_for_log "Restart result: success" "$SUCCESS_LOG"
kill "$SUPERVISOR_PID"
wait "$SUPERVISOR_PID" >/dev/null 2>&1 || true
unset SUPERVISOR_PID
docker compose -f "$ROOT/tests/supervisor-fixture/docker-compose.yml" down --remove-orphans >/dev/null

FAILURE_LOG="$LOG_DIR/failure.log"
STARTUP_DELAY_SECONDS=120 \
CONFIG="$ROOT/tests/supervisor-fixture/supervisor-failure.json" \
  python3 "$ROOT/app.py" supervisor >"$FAILURE_LOG" 2>&1 &
SUPERVISOR_PID="$!"
wait_for_log "Initial start result: failure" "$FAILURE_LOG"

echo "Supervisor smoke tests passed."
