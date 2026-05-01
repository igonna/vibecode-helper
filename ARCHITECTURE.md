# Target Architecture

This project should become a single always-running Vibecode Helper with many
small per-app runners.

## Goal

The user should not edit config files for normal setup. They should open the
helper UI, create an app, copy the generated hook config, and run the generated
runner command for that app.

## Stack Decision

The target implementation should be light and local-first.

Use:

- Go for the helper backend and runner mode.
- SQLite with WAL mode for persistence.
- Server-rendered HTML with Go templates.
- htmx plus small vanilla JavaScript for UI interactions.
- Server-sent events for browser updates and runner updates.
- A single binary for normal installation.

Avoid for the normal product:

- Next.js. It adds Node, build tooling, memory usage, and framework complexity
  that this local dashboard does not need.
- Postgres. It adds a database daemon, ports, memory, startup time, backups,
  and more failure modes. SQLite is enough for local app configs, prompt stats,
  and event history.
- Docker as the normal runtime. Docker is useful for the prototype, but the
  final helper should not depend on Docker for basic use.
- Python as the always-running backend. It works for the prototype, but Go is a
  better fit for a low-idle local daemon and process runner.

The expected final binary modes:

```bash
vibecode-helper serve
vibecode-helper runner --app my-app --helper-url http://localhost:8766
```

The generated per-app `run.sh` should usually be only a tiny wrapper around
runner mode.

```bash
#!/usr/bin/env bash
set -euo pipefail
vibecode-helper runner --app my-app --helper-url http://localhost:8766
```

### Resource Goals

Idle behavior matters because this tool is expected to stay open on a laptop.

Targets:

- very low idle CPU
- small memory footprint
- no background polling when SSE or long polling is available
- no database daemon
- no Node process for the UI
- one browser tab for UI and sound playback

SQLite should use WAL mode:

```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
```

Suggested local paths:

```text
~/.local/share/vibecode-helper/helper.db
~/.cache/vibecode-helper/
```

### Central Helper

Runs once and stays up independently from individual apps.

Responsibilities:

- Serve the web UI.
- Store apps, settings, stats, and event history in SQLite.
- Generate hook config per app.
- Receive hook events:
  - `POST /apps/:app_id/prompt_started`
  - `POST /apps/:app_id/permission_asked`
  - `POST /apps/:app_id/prompt_finished`
- Track prompt timing:
  - prompt count
  - total prompt time
  - average prompt time
  - per-prompt durations
- Track app and runner state:
  - runner connected/disconnected
  - command running/stopped
  - restart requested/running/done
  - last health result
- Send browser sound notifications:
  - success
  - failure
  - permission asked
- Check app health after restart, preferably from the browser tab when the
  health URL is browser-facing.

The helper is the control plane and source of truth.

### Per-App Runner

Each app gets a small generated runner script. The runner owns exactly one app
command.

Responsibilities:

- Start the configured command.
- Keep command output visible in the terminal where the runner is running.
- Connect to the central helper.
- Wait for restart requests.
- Stop and restart the command when requested.
- Report lifecycle events back to helper:
  - runner connected
  - command started
  - command exited
  - restart started
  - restart finished

The runner is the worker. It should contain minimal logic and no persistent app
state.

## UX

1. User starts the helper once.

   ```bash
   vibecode-helper
   ```

2. User opens the UI.

   ```text
   http://localhost:8766
   ```

3. User clicks `New app` and fills:

   - app name
   - working directory
   - start command
   - health URL
   - startup timeout
   - sound repeat count

4. UI shows a generated runner command, for example:

   ```bash
   mkdir -p ~/vibecode-runners/my-app
   cd ~/vibecode-runners/my-app
   curl -fsSL http://localhost:8766/apps/my-app/runner.sh -o run.sh
   chmod +x run.sh
   ./run.sh
   ```

5. UI shows hook config for that app:

   ```json
   {
     "hooks": {
       "UserPromptSubmit": [
         {
           "hooks": [
             {
               "type": "command",
               "command": "curl -s -X POST http://127.0.0.1:8767/apps/my-app/prompt_started >/dev/null",
               "timeout": 5,
               "statusMessage": "Prompt started"
             }
           ]
         }
       ],
       "PermissionRequest": [
         {
           "hooks": [
             {
               "type": "command",
               "command": "curl -s -X POST http://127.0.0.1:8767/apps/my-app/permission_asked >/dev/null",
               "timeout": 5,
               "statusMessage": "Permission asked"
             }
           ]
         }
       ],
       "Stop": [
         {
           "matcher": "*",
           "hooks": [
             {
               "type": "command",
               "command": "curl -s -X POST http://127.0.0.1:8767/apps/my-app/prompt_finished >/dev/null",
               "timeout": 5,
               "statusMessage": "Prompt finished"
             }
           ]
         }
       ]
     }
   }
   ```

### Dashboard

The first screen should be a simple modern dashboard:

```text
Vibecode Helper

                         [ Search apps... ]

[ + Add app ]

my-platform                                      Healthy
http://localhost:3000
Prompts 24   Total wait 1h 12m   Avg wait 3m 01s   Last wait 2m 14s

billing-api                                      Permission asked
http://localhost:8080
Prompts 8    Total wait 18m      Avg wait 2m 15s   Last wait 1m 44s

frontend                                         Runner disconnected
http://localhost:5173
Prompts 0    Total wait 0s       Avg wait 0s       Last wait -
```

The search input should be centered near the top. Under it, show app blocks.
The dashboard should avoid complex navigation and avoid dense admin-style
tables for the default view.

Each app card should show:

```text
my-platform                                      Running
http://localhost:3000

Prompts 24       Total wait 1h 12m       Avg wait 3m 01s       Last wait 2m 14s
```

Collapsed app blocks should show only high-signal state:

- app name
- current status
- health URL
- prompt count
- total user wait time
- average user wait time
- last user wait time
- runner connected/disconnected indicator

Common status values:

- runner disconnected
- running
- prompt active
- permission asked
- restart requested
- restarting
- checking health
- healthy
- failed

Clicking an app expands it inline. Expansion should not navigate away from the
dashboard unless the user explicitly opens a detail page.

Expanded app block:

```text
Status
Runner: connected
Command: running
Health: healthy
Last restart: success, 12s
Last event: prompt_finished

Setup
[ Copy runner command ] [ Copy hooks ]

Controls
[ Restart now ] [ Test success ] [ Test failure ] [ Test permission ]

Config
Command: docker compose up --build
Working dir: /repo
Health: http://localhost:3000
Timeout: 60s

Recent prompts
#24  wait 2m14s  prompt 1m52s  restart 22s  success
#23  wait 3m10s  prompt 2m41s  restart 29s  success
#22  wait 4m02s  prompt 3m05s  restart 57s  failure

Events
12:04 prompt_finished
12:04 restart_requested
12:04 runner_restarted
12:04 health_ok
12:04 success_sound
```

Expanded content can use segmented tabs:

- overview
- setup
- stats
- events
- config

The default expanded tab should be overview.

App card actions:

- copy hooks
- runner setup
- edit
- test sounds
- restart now
- view events

Global actions:

- add app
- browser sound status
- settings

### Add App Flow

The create form should ask only for the values the user understands:

- app name
- where Codex runs:
  - on my machine
  - inside a dev container
- where the runner runs:
  - on my machine
  - inside a dev container
- working directory
- command
- health check mode:
  - browser URL
  - runner command
- health URL or health command
- startup timeout
- sound repeat count

The UI should generate correct URLs based on where Codex and the runner run:

- host machine: `http://localhost:8766`
- dev container: `http://host.docker.internal:8766`

The user should not need to know Docker networking details for normal setup.

### App Detail

Each app should have a detail page with these sections:

- status
- prompt stats
- runner setup
- hook config
- app config
- events
- sounds

Status:

```text
Runner: connected
Command: running
Health: healthy
Last event: prompt_finished
Last sound: success
```

Prompt stats:

```text
Prompts completed: 18
Total prompt time: 30m 42s
Average prompt time: 1m 41s
Current prompt: running for 42s
Last prompt: 2m 14s
```

Recent prompts:

```text
#18  2m 14s  success  12:04
#17  1m 02s  success  11:58
#16  4m 31s  failure  11:52
```

Events:

```text
12:04 prompt finished, 2m 14s
12:04 restart requested
12:04 runner restarted command
12:04 browser health check passed
12:04 success sound played
```

Sounds:

- test success
- test failure
- test permission asked

When permission is requested, the dashboard should clearly show:

```text
my-platform needs permission
```

When restart succeeds:

```text
my-platform restarted successfully
```

When restart fails:

```text
my-platform failed to become healthy after 60s
```

## Statistics

Statistics are per app and should be stored in SQLite.

The main statistic is user waiting time:

```text
prompt_started -> app healthy after prompt_finished restart
```

This measures the real wait from when the user submits a prompt until the app is
usable again after the generated changes are applied and the runner restarts the
app.

Dashboard headline stats:

- prompts completed
- total user wait time
- average user wait time
- last user wait time
- current prompt elapsed time, when active
- last restart result

Prompt stats:

- prompt count
- active prompt start time
- active prompt elapsed time
- last prompt duration
- last user wait duration
- total user wait duration
- average user wait duration
- min user wait duration
- max user wait duration
- recent prompt history

Prompt run fields:

- app id
- prompt index
- started at
- finished at
- prompt duration, from `prompt_started` to `prompt_finished`
- user wait duration, from `prompt_started` to app healthy after restart
- restart requested at
- restart started at
- restart finished at
- healthy at
- whether permission was asked during the prompt
- permission count during the prompt
- restart result after the prompt:
  - success
  - failure
  - skipped
- health result after restart

Restart stats:

- restart count
- successful restart count
- failed restart count
- restart duration, from restart started to app healthy or failed
- last restart duration
- average restart duration
- last failure reason

Notification stats:

- success sound count
- failure sound count
- permission sound count
- last notification type
- last notification time

Runner stats:

- connected/disconnected state
- last connected at
- last disconnected at
- command started at
- command exited at
- last command exit code

The dashboard should show the highest-signal subset. Detailed tables can live on
the app detail page.

## Event Flow

### Prompt Started

1. Codex hook calls helper.
2. Helper records prompt start time for the app.
3. UI updates prompt state.

### Permission Asked

1. Codex hook calls helper.
2. Helper records the event.
3. Helper sends permission sound to the browser.
4. UI shows the app needs attention.

### Prompt Finished

1. Codex hook calls helper.
2. Helper records prompt duration and updates stats.
3. Helper marks the app as restart requested.
4. Runner receives restart request.
5. Runner restarts the app command.
6. Runner reports restart lifecycle.
7. Helper checks health.
8. Helper sends success or failure sound.
9. UI updates the specific app card.

## Runner Communication

The runner should connect outward to the helper, not wait for the helper to call
it. This works better when the runner is inside a dev container.

Preferred options:

- Server-sent events from helper to runner.
- Long polling from runner to helper.

Polling every 1-2 seconds is acceptable for a first implementation, but SSE or
long polling is cleaner.

## Networking Rule

The helper backend should run where hooks can reach it. The browser is only the
UI and sound bridge.

If the runner is inside a dev container and cannot reach `localhost:8766`, the
generated runner command should include a helper URL, for example:

```bash
HELPER_URL=http://host.docker.internal:8766 ./run.sh
```

The UI should make this explicit when generating runner commands.

## Persistence

The target implementation should use SQLite.

Suggested tables:

- `apps`
- `app_settings`
- `prompt_runs`
- `restart_runs`
- `runner_events`
- `notifications`

The current implementation keeps prompt stats in memory only. That is temporary.

## Current State

The current code is still a single-app prototype:

- `run.sh` starts the sound service and supervisor.
- `config.json` defines one app.
- prompt stats are in memory.
- hooks call global endpoints.

Future work should move this toward the central helper plus per-app runner
model described above.
