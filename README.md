# vibecode-dev-notify

One command to run your dev app, wait until it is healthy, and play browser
sounds when it succeeds or fails.

## Start

```bash
cp example.config.json config.json
code config.json
./run.sh
```

When it prints this URL, open it in your browser and keep it open:

```text
http://localhost:8766
```

Then `./run.sh` plays test sound 1 and test sound 2, starts your app command in
the same terminal, and later plays `success` or `failure` after restarts.

If the page shows `Enable sound permission`, click it once.

## Config

Edit only these:

- `app_cwd`: directory where your app command should run.
- `app_start_command`: command that starts your app.
- `app_health_url`: URL that means the app is ready.
- `app_startup_timeout_seconds`: how long to wait before playing failure.
- `sound_repeat_count`: how many times each sound should play.

If `app_health_url` cannot be reached from this terminal, use
`app_health_command` instead.

Example:

```json
"app_health_command": [
  "docker",
  "compose",
  "exec",
  "-T",
  "platform-proxy",
  "wget",
  "-q",
  "-O",
  "-",
  "http://localhost:3000"
]
```

## Restart Hook

While `./run.sh` is running, trigger a restart with:

```bash
curl -X POST http://127.0.0.1:8767/restart
```

That stops the current app command, starts it again, waits for the health check,
then plays `success` or `failure`.

For Codex, enable hooks in `~/.codex/config.toml`:

```toml
[features]
codex_hooks = true
```

Point the hook at:

```text
POST http://127.0.0.1:8767/restart
```

## Manual Sound

From this machine:

```bash
curl -X POST http://localhost:8766/notify \
  -H 'Content-Type: application/json' \
  -d '{"event":"success"}'
```

From a container on `repo_default`:

```bash
docker run --rm --network repo_default curlimages/curl \
  -X POST http://vibecode-dev-notify:8765/notify \
  -H 'Content-Type: application/json' \
  -d '{"event":"failure"}'
```
