# vibecode-dev-notify

One command to run your dev app, wait until it is healthy, and play browser
sounds when it succeeds or fails.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the target design: one central helper
server with many per-app runners.

See [ROADMAP.md](ROADMAP.md) for the prioritized rewrite checklist.

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

Then `./run.sh` plays test sounds for `success`, `failure`, and `permission`,
starts your app command in the same terminal, and later plays notifications from
hooks.

If the page shows `Enable sound permission`, click it once.

## Config

Edit only these:

- `app_cwd`: directory where your app command should run.
- `app_start_command`: command that starts your app.
- `app_health_url`: URL your browser can open when the app is ready.
- `app_startup_timeout_seconds`: how long to wait before playing failure.
- `prompt_started_url`: called when a prompt starts.
- `prompt_finished_url`: called when a prompt finishes; this restarts the app.
- `permission_asked_url`: called when Codex asks for permission.
- `sound_repeat_count`: how many times each sound should play.

By default, the open browser tab checks `app_health_url`. That avoids Docker
`localhost` problems: if your browser can open the app, the health check can see
it too.

If you need a custom check, use `app_health_command` instead.

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

## Vibecode Hooks

`./run.sh` prints the full hook config to paste into Vibecode. It uses:

```bash
curl -X POST http://127.0.0.1:8767/prompt_started
curl -X POST http://127.0.0.1:8767/permission_asked
curl -X POST http://127.0.0.1:8767/prompt_finished
```

`prompt_started` records timing. `prompt_finished` records duration, prints
prompt stats, restarts the app command, waits for health, then plays `success`
or `failure`. `permission_asked` plays the permission sound when the agent is
waiting for you.

The same hook JSON is tracked in:

```text
example.hooks.json
```

Stats are also available while `./run.sh` is running:

```bash
curl http://127.0.0.1:8767/stats
```

The old endpoint still works as a compatibility alias:

```text
POST http://127.0.0.1:8767/restart
```

For Codex, enable hooks in `~/.codex/config.toml`:

```toml
[features]
codex_hooks = true
```

Use the hook JSON printed by `./run.sh`.

## Manual Sound

From this machine:

```bash
curl -X POST http://localhost:8766/notify \
  -H 'Content-Type: application/json' \
  -d '{"event":"permission"}'
```
