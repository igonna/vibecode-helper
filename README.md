# vibecode-dev-notify

Restart your dev command, wait for your app to come back, and play a browser sound.

## Setup

```bash
cp example.config.json config.json
```

Edit `config.json`:

- `command` or `commands`: what should be restarted.
- `urls`: what should be reachable after restart.
- `target_urls`: what the background monitor watches.

Start the browser sound service:

```bash
docker compose up --build --force-recreate
```

`config.json` is copied into the image during build, so rerun with `--build`
after editing it.

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

On startup, the supervisor starts your command, waits for `urls`, then plays
`success` or `failure`.

Trigger restart:

```bash
curl -X POST http://127.0.0.1:8767/restart
```

On startup and restart, it starts your configured command, waits up to
`startup_timeout_seconds`, then plays:

- `success` if a configured URL becomes reachable.
- `failure` if it does not.

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
