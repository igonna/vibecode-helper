# Rewrite Roadmap

The current repository is a Python/Docker prototype. The final product should be
a lightweight Go-based Vibecode Helper with one central server and many per-app
runners.

Old prototype code can be rewritten or removed when the new implementation
replaces it. Do not preserve legacy structure just because it exists.

## Priority 0: Product Shape

- [x] Keep one central Vibecode Helper server.
- [x] Support many apps from one helper instance.
- [x] Keep per-app runners separate from the helper.
- [x] Store everything persistent in SQLite.
- [x] Make the browser UI the source of app setup.
- [x] Keep runner terminals focused on app command output.
- [x] Avoid requiring users to edit config files for normal setup.

## Priority 1: Core Backend

- [x] Create Go project structure.
- [x] Add `vibecode-helper serve`.
- [x] Add SQLite database initialization.
- [x] Enable SQLite WAL mode.
- [x] Add migrations or deterministic schema setup.
- [x] Add app CRUD:
  - app name
  - working directory
  - command
  - health mode
  - health URL or health command
  - startup timeout
  - sound repeat count
  - Codex environment
  - runner environment
- [x] Add event storage.
- [x] Add prompt run storage.
- [x] Add restart run storage.
- [x] Add notification storage.

## Priority 2: Hook API

- [x] Add per-app hook endpoints:
  - `POST /apps/:app_id/prompt_started`
  - `POST /apps/:app_id/permission_asked`
  - `POST /apps/:app_id/prompt_finished`
- [x] On `prompt_started`, create active prompt run.
- [x] On `permission_asked`, record event and trigger permission sound.
- [x] On `prompt_finished`, close prompt duration and request app restart.
- [x] Track main wait metric:
  - `prompt_started -> app healthy after restart`
- [x] Generate hook JSON per app in the UI.
- [x] Keep status messages:
  - `Prompt started`
  - `Permission asked`
  - `Prompt finished`

## Priority 3: Runner Protocol

- [x] Add `vibecode-helper runner --app <id> --helper-url <url>`.
- [x] Generate per-app `run.sh` from the helper.
- [x] Runner connects outward to helper.
- [x] Runner reports connected/disconnected.
- [x] Runner starts configured command.
- [x] Runner keeps command output visible in terminal.
- [x] Runner waits for restart requests.
- [x] Runner stops command gracefully.
- [x] Runner starts command again.
- [x] Runner reports:
  - command started
  - command exited
  - restart started
  - restart finished
  - exit code
- [x] Prefer SSE or long polling over frequent polling.
- [x] Support `HELPER_URL=http://host.docker.internal:8766 ./run.sh` for dev
  container runners.

## Priority 4: Health Checks

- [x] Support browser URL health checks.
- [x] Support runner command health checks.
- [x] Store health check mode per app.
- [x] After restart, wait until healthy or timeout.
- [x] Record:
  - restart requested at
  - restart started at
  - restart finished at
  - healthy at
  - failure reason
- [x] Trigger success sound on healthy.
- [x] Trigger failure sound on timeout or failed restart.

## Priority 5: Browser Sound Bridge

- [x] Serve browser UI from helper.
- [x] Use browser tab for sound playback.
- [x] Require one click if browser blocks autoplay.
- [x] Support sounds:
  - success
  - failure
  - permission asked
- [x] Cache sound files.
- [x] Send sound events over SSE.
- [x] Show browser sound status in UI.
- [x] Add test buttons:
  - test success
  - test failure
  - test permission

## Priority 6: Dashboard UI

- [x] Build server-rendered Go templates.
- [x] Use htmx plus small vanilla JS.
- [x] Add top-centered search input.
- [x] Add `+ Add app`.
- [x] Show app blocks under search.
- [x] Collapsed app block shows:
  - app name
  - current status
  - health URL
  - prompt count
  - total wait
  - average wait
  - last wait
  - runner connected/disconnected
- [x] Click app block to expand inline.
- [x] Expanded app block includes:
  - overview
  - setup
  - stats
  - events
  - config
- [x] Add copy buttons for runner command and hook config.
- [x] Add app-level notifications:
  - needs permission
  - restarted successfully
  - failed to become healthy

## Priority 7: Statistics

- [x] Store prompt count per app.
- [x] Store active prompt start time.
- [x] Store prompt duration:
  - `prompt_started -> prompt_finished`
- [x] Store main user wait duration:
  - `prompt_started -> app healthy after restart`
- [x] Store total user wait duration.
- [x] Store average user wait duration.
- [x] Store min/max user wait duration.
- [x] Store last user wait duration.
- [x] Store permission count per prompt.
- [x] Store restart result per prompt:
  - success
  - failure
  - skipped
- [x] Show headline stats on dashboard.
- [x] Show detailed prompt history in expanded app view.

## Priority 8: Packaging

- [x] Build single Go binary.
- [x] Provide Docker Compose for always-running helper.
- [x] Mount SQLite data volume.
- [x] Expose stable host port, default `8766`.
- [x] Use `restart: unless-stopped`.
- [x] Keep direct binary mode available for users who do not want Docker.
- [x] Document Docker Desktop always-on setup.

## Priority 9: Migration From Prototype

- [x] Keep current prototype available until Go helper can replace it.
- [x] Do not port Python code line-by-line.
- [x] Reuse only behavior and lessons learned:
  - browser sound bridge
  - prompt hooks
  - permission sound
  - browser health checks
  - graceful restart behavior
- [x] Remove obsolete files after replacement:
  - old Python supervisor paths
  - single-app config flow
  - prototype Dockerfile/Compose if no longer needed

## Not Planned

These are intentionally not part of this product direction:

- Cloud sync.
- Multi-user auth.
- Postgres.
- Next.js.
- Remote runners over the internet.
- Complex alert routing.
