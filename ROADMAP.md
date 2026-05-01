# Rewrite Roadmap

The current repository is a Python/Docker prototype. The final product should be
a lightweight Go-based Vibecode Helper with one central server and many per-app
runners.

Old prototype code can be rewritten or removed when the new implementation
replaces it. Do not preserve legacy structure just because it exists.

## Priority 0: Product Shape

- [ ] Keep one central Vibecode Helper server.
- [ ] Support many apps from one helper instance.
- [ ] Keep per-app runners separate from the helper.
- [ ] Store everything persistent in SQLite.
- [ ] Make the browser UI the source of app setup.
- [ ] Keep runner terminals focused on app command output.
- [ ] Avoid requiring users to edit config files for normal setup.

## Priority 1: Core Backend

- [ ] Create Go project structure.
- [ ] Add `vibecode-helper serve`.
- [ ] Add SQLite database initialization.
- [ ] Enable SQLite WAL mode.
- [ ] Add migrations or deterministic schema setup.
- [ ] Add app CRUD:
  - app name
  - working directory
  - command
  - health mode
  - health URL or health command
  - startup timeout
  - sound repeat count
  - Codex environment
  - runner environment
- [ ] Add event storage.
- [ ] Add prompt run storage.
- [ ] Add restart run storage.
- [ ] Add notification storage.

## Priority 2: Hook API

- [ ] Add per-app hook endpoints:
  - `POST /apps/:app_id/prompt_started`
  - `POST /apps/:app_id/permission_asked`
  - `POST /apps/:app_id/prompt_finished`
- [ ] On `prompt_started`, create active prompt run.
- [ ] On `permission_asked`, record event and trigger permission sound.
- [ ] On `prompt_finished`, close prompt duration and request app restart.
- [ ] Track main wait metric:
  - `prompt_started -> app healthy after restart`
- [ ] Generate hook JSON per app in the UI.
- [ ] Keep status messages:
  - `Prompt started`
  - `Permission asked`
  - `Prompt finished`

## Priority 3: Runner Protocol

- [ ] Add `vibecode-helper runner --app <id> --helper-url <url>`.
- [ ] Generate per-app `run.sh` from the helper.
- [ ] Runner connects outward to helper.
- [ ] Runner reports connected/disconnected.
- [ ] Runner starts configured command.
- [ ] Runner keeps command output visible in terminal.
- [ ] Runner waits for restart requests.
- [ ] Runner stops command gracefully.
- [ ] Runner starts command again.
- [ ] Runner reports:
  - command started
  - command exited
  - restart started
  - restart finished
  - exit code
- [ ] Prefer SSE or long polling over frequent polling.
- [ ] Support `HELPER_URL=http://host.docker.internal:8766 ./run.sh` for dev
  container runners.

## Priority 4: Health Checks

- [ ] Support browser URL health checks.
- [ ] Support runner command health checks.
- [ ] Store health check mode per app.
- [ ] After restart, wait until healthy or timeout.
- [ ] Record:
  - restart requested at
  - restart started at
  - restart finished at
  - healthy at
  - failure reason
- [ ] Trigger success sound on healthy.
- [ ] Trigger failure sound on timeout or failed restart.

## Priority 5: Browser Sound Bridge

- [ ] Serve browser UI from helper.
- [ ] Use browser tab for sound playback.
- [ ] Require one click if browser blocks autoplay.
- [ ] Support sounds:
  - success
  - failure
  - permission asked
- [ ] Cache sound files.
- [ ] Send sound events over SSE.
- [ ] Show browser sound status in UI.
- [ ] Add test buttons:
  - test success
  - test failure
  - test permission

## Priority 6: Dashboard UI

- [ ] Build server-rendered Go templates.
- [ ] Use htmx plus small vanilla JS.
- [ ] Add top-centered search input.
- [ ] Add `+ Add app`.
- [ ] Show app blocks under search.
- [ ] Collapsed app block shows:
  - app name
  - current status
  - health URL
  - prompt count
  - total wait
  - average wait
  - last wait
  - runner connected/disconnected
- [ ] Click app block to expand inline.
- [ ] Expanded app block includes:
  - overview
  - setup
  - stats
  - events
  - config
- [ ] Add copy buttons for runner command and hook config.
- [ ] Add app-level notifications:
  - needs permission
  - restarted successfully
  - failed to become healthy

## Priority 7: Statistics

- [ ] Store prompt count per app.
- [ ] Store active prompt start time.
- [ ] Store prompt duration:
  - `prompt_started -> prompt_finished`
- [ ] Store main user wait duration:
  - `prompt_started -> app healthy after restart`
- [ ] Store total user wait duration.
- [ ] Store average user wait duration.
- [ ] Store min/max user wait duration.
- [ ] Store last user wait duration.
- [ ] Store permission count per prompt.
- [ ] Store restart result per prompt:
  - success
  - failure
  - skipped
- [ ] Show headline stats on dashboard.
- [ ] Show detailed prompt history in expanded app view.

## Priority 8: Packaging

- [ ] Build single Go binary.
- [ ] Provide Docker Compose for always-running helper.
- [ ] Mount SQLite data volume.
- [ ] Expose stable host port, default `8766`.
- [ ] Use `restart: unless-stopped`.
- [ ] Keep direct binary mode available for users who do not want Docker.
- [ ] Document Docker Desktop always-on setup.

## Priority 9: Migration From Prototype

- [ ] Keep current prototype available until Go helper can replace it.
- [ ] Do not port Python code line-by-line.
- [ ] Reuse only behavior and lessons learned:
  - browser sound bridge
  - prompt hooks
  - permission sound
  - browser health checks
  - graceful restart behavior
- [ ] Remove obsolete files after replacement:
  - old Python supervisor paths
  - single-app config flow
  - prototype Dockerfile/Compose if no longer needed

## Out of Scope For First Rewrite

- [ ] Cloud sync.
- [ ] Multi-user auth.
- [ ] Postgres.
- [ ] Next.js.
- [ ] Remote runners over the internet.
- [ ] Complex alert routing.
