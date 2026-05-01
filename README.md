# vibecode-dev-notify

Local Vibecode helper for app restarts, prompt timing, health checks, and browser
sound notifications.

## Start helper

```bash
docker compose up --build
```

To keep it running in Docker Desktop:

```bash
docker compose up -d --build
```

Open and keep this page open:

```text
http://localhost:8766
```

The browser tab is used for sounds and browser-side health checks.

Direct binary mode is also supported:

```bash
make build
./vibecode-helper serve
```

## Add an app

In the dashboard:

1. Click `+ Add app`.
2. Enter the app command. Leave browser URL check off for immediate success after restart, or enable it when the app must become reachable before success.
3. Copy the generated runner command and run it from your app folder. It starts the app and installs project-local Codex hooks automatically.

Use the generated commands exactly as shown. They use Docker as the bridge to
the helper container, so they work from the host or a dev container as long as
the same Docker daemon is accessible.

## Hooks

The runner installs project-local Codex hook files automatically:

```text
<repo>/.codex/config.toml
<repo>/.codex/hooks.json
```

It preserves existing hooks and replaces only this app's generated hook entries.

Each app gets its own hook URLs:

```text
POST /apps/:app_id/prompt_started
POST /apps/:app_id/permission_asked
POST /apps/:app_id/prompt_finished
```

`prompt_finished` requests a runner restart. When the app becomes healthy, the
browser plays success. If it times out or restart fails, the browser plays
failure. `permission_asked` plays the permission sound.

## Notes

The helper stores app config, prompt stats, restart records, events, and
notifications in SQLite. Docker Compose mounts the database under `/data`.

See [ARCHITECTURE.md](ARCHITECTURE.md) and [ROADMAP.md](ROADMAP.md).
