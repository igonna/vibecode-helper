# vibecode-dev-notify

Restart your dev command, wait for your app to come back, and play a browser sound.

## Setup

```bash
cp example.config.json config.json
```

Edit `config.json`:

- `command` or `commands`: what should be restarted.
- `health_url`: the single URL that must become reachable after restart.
- `health_command`: optional command-based health check for isolated container environments.
- `target_urls`: what the background monitor watches.

Start the browser sound service:

```bash
docker compose up --build --force-recreate
```

Open once:

```text
http://localhost:8766
```

If the page asks for sound permission, click once and leave the tab open.

## Restart Hook

Run the supervisor in a normal terminal:

```bash
python3 app.py supervisor
```

The supervised command inherits this terminal, so its output appears directly,
without being hidden behind Docker Compose service logs.

On startup, the supervisor starts your command, waits for `health_url` or
`health_command`, then plays
`success` or `failure`.

Trigger restart:

```bash
curl -X POST http://127.0.0.1:8767/restart
```

On startup and restart, it starts your configured command, waits up to
`startup_timeout_seconds`, then plays:

- `success` if `health_url` becomes reachable, or `health_command` exits `0`.
- `failure` if it does not.

Use a URL that is reachable from where `python3 app.py supervisor` runs. If the
supervisor runs in an isolated container, `http://localhost:3000` means that
container, not your app container. In that case use a Docker network name such
as `http://platform-proxy:3000`, or attach the supervisor environment to the
network where the app runs. If that is not practical, use `health_command`; the
fixture test uses `docker compose exec` so the health check runs inside the
service container.

## Manual Sound Test

From a container on `repo_default`:

```bash
docker run --rm --network repo_default curlimages/curl \
  -X POST http://vibecode-dev-notify:8765/notify \
  -H 'Content-Type: application/json' \
  -d '{"event":"failure"}'
```

From the host:

```bash
curl -X POST http://localhost:8766/notify \
  -H 'Content-Type: application/json' \
  -d '{"event":"success"}'
```

## Codex Hook

Enable hooks in `~/.codex/config.toml`:

```toml
[features]
codex_hooks = true
```

Point the hook at:

```text
POST http://127.0.0.1:8767/restart
```

## Optional Background Monitor

The restart hook is the main workflow. If you also want continuous background
monitoring, start the optional monitor profile:

```bash
docker compose --profile monitor up --build --force-recreate
```

## Local Smoke Test

The fixture starts a tiny Python web service with Docker Compose and verifies:

- success: service becomes healthy after startup and after `/restart`.
- failure: service does not become healthy before the timeout.

Run:

```bash
bash tests/supervisor-fixture/run-smoke.sh
```

The test uses only the fixture under `tests/supervisor-fixture`; it does not
touch the target app stack.
