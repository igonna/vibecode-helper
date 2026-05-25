# Vibecode Platform Direction

This document captures the product direction for turning Vibecode Helper from a
local utility into a hosted platform while keeping the local engine reliable,
small, and understandable.

## Product Thesis

Vibecoding should feel automatic and calm:

- prompt activity is tracked without manual logging
- permission requests are visible immediately
- app commands can pause, restart, and recover around prompts
- stats show where time is going
- setup is one copied command, not a configuration project
- the system keeps working even when the hosted service is unreachable

The hosted platform should not replace the local engine. It should coordinate
and visualize what the local engine already knows.

## Architecture Split

Use two layers.

### Local Engine

The local engine should stay Go.

Responsibilities:

- install project-local Codex hooks
- receive hook events from Codex on localhost
- write every received event to a local durable queue before returning
- run optional local command automation:
  - command start/stop/restart
  - pause during prompt
  - resume after prompt
  - health checks
  - auto-pause after ready
- expose local diagnostics, stats, and cloud delivery health through one CLI
  status view
- deliver queued events to the hosted platform in the background
- continue working when the hosted service is down

The local engine is the reliability layer. Hooks should call it first.

### Hosted Platform

The hosted platform can use a web-first stack such as Next.js.

Responsibilities:

- user accounts
- teams and projects
- onboarding and install scripts
- web dashboard
- cross-machine/project stats
- billing and plan limits later
- cloud API for uploaded events
- project tokens and device registration
- long-term analytics and reporting

The platform is the collaboration and visualization layer. It should not be
required for local hooks to complete.

## Hook Event Flow

Hooks should call the `vibeengine` CLI, not the cloud API and not a local HTTP
endpoint.

```text
Codex hook
  -> vibeengine hook ...
  -> write local SQLite event
  -> immediate hook response
  -> background engine uploads later
  -> cloud acknowledges event
  -> local queue marks event delivered
```

Example generated hooks:

```bash
vibeengine hook session-start --project abc
vibeengine hook prompt-started --project abc
vibeengine hook permission-asked --project abc
vibeengine hook prompt-finished --project abc
```

This preserves Codex UX:

- hooks do not block on internet access
- cloud outages do not break prompt tracking
- slow hosted API responses do not make Codex feel stuck
- events are not lost if the laptop is offline
- no localhost port, socket, or local network listener is required for hooks

## Local Durability

The hook CLI should persist events before exiting. The background engine is not
required for event capture.

Suggested tables:

- `events`
  - local event id
  - project id
  - event type
  - payload JSON
  - created at
  - delivery status
  - delivery attempt count
  - last delivery error
- `projects`
  - local project id
  - cloud project id
  - repo path
  - command settings
  - hook install status
- `delivery_state`
  - last successful upload time
  - current account/device identity
  - server base URL

Use SQLite WAL mode locally. Do not keep the only copy of events in memory.

## Project Identity And Local Storage

Projects should feel name-based to users, but use stable ids internally.

User-facing rules:

- project names are unique inside an account or team
- users see and type project names, for example `bet1`
- hooks can use the project name:
  ```bash
  vibeengine hook prompt-started --project bet1
  ```
- hosted UI may use nice project slugs
- users should not need to know a project id

Internal rules:

- every project still has an immutable `project_id`
- bootstrap tokens target the internal `project_id`
- database foreign keys and audit records use ids, not names
- project names can be renamed without breaking history
- the CLI resolves project names through local state

Do not create a `.vibeengine` directory or state file inside the user's repo.
Repo contents should stay clean except for the normal Codex hook files.

Local state should live in the OS app-data area, not in the repo. Exact paths are
implementation details and should not be treated as user-facing API. The CLI is
the supported interface.

Storage policy:

- keep data outside the repo
- use strict user-only permissions
- keep auth tokens separate from ordinary event metadata
- redact tokens in logs and status output
- warn if local file permissions are too open
- do not print internal DB paths unless verbose/debug mode is requested

Avoid fake security. Full DB encryption is not useful unless there is a real key
management story. If the encryption key sits next to the DB, it mostly adds
failure modes without real protection. Prefer strict permissions, token
rotation, server-side revocation, and metadata-only defaults.

## Data Upload Policy

Default upload is metadata-only.

Upload by default:

- account, device, and project ids
- event type
- UTC event timestamps
- UTC check-in timestamps
- durations derived from timestamps
- permission count
- high-level result such as success, failure, or skipped
- high-level app status such as Prompting, Ready, Paused, Offline, Attention
- project display name
- command automation enabled/disabled
- command hash for change detection, not command text
- health mode and health result
- engine version, OS, and architecture
- queue count, last upload time, and update status

Do not upload:

- prompt text
- model responses
- command output or logs
- shell command text
- shell health-check command text
- environment variables
- source code
- git diffs
- file names touched
- terminal output
- browser health response body
- secrets or tokens

Do not add product toggles for uploading prompt content, command logs, or repo
metadata in the first product direction. Keeping the policy simple is part of
the trust model.

All stored and uploaded timestamps should be UTC. Local time zones are only a
display concern. This avoids confusion when a project is used across containers,
machines, regions, or team members.

## Background Engine

The background engine is responsible for work that requires a long-running
process:

- uploading queued events
- receiving remote actions through check-in
- applying command/config changes
- running, pausing, and restarting commands
- health checks
- update checks

If the engine is not running:

- hook events are still recorded by the CLI
- local prompt stats can still be reconstructed from SQLite
- cloud upload waits
- remote command/config actions wait
- command automation is unavailable

This is the honest contract. If no process is running, nothing can poll or apply
remote actions.

`SessionStart` should make normal usage seamless:

```bash
vibeengine hook session-start --project abc
```

That command should:

- record `session_started`
- check whether the background engine is running
- start it in the background if missing
- exit quickly

`vibeengine status` should also check the engine and start or repair it when
that is safe.

The result is simple:

- hooks are reliable without the engine
- automation works when the engine is running
- Codex session start wakes the engine for normal use
- no local API server or port is needed

## Cloud Check-In

Use one simple HTTP check-in loop. Do not use WebSockets or SSE for the first
platform version.

The background engine periodically calls the hosted API:

```text
POST /engine/check-in
```

The request can include:

- device identity
- project identities
- local engine version
- queued local events
- command/runner status summaries
- last applied remote action id

The response can include:

- acknowledged event ids
- pending remote actions
- proposed project config version
- update notice
- next suggested check-in delay

This keeps the system easy to reason about: one HTTP request path handles event
upload, remote action proposals, config proposals, and update notices.

Default cadence:

- check in every 15-30 seconds when logged in and healthy
- check in immediately after it notices local events such as prompt start/end
- check in immediately when `vibeengine status` runs or when a local dashboard
  is open
- back off when the server is unreachable, for example `30s -> 1m -> 5m`
- return to the normal cadence after the server is reachable again

This is not hostile if it is visible and restrained. It is normal product
behavior for a local agent connected to a hosted dashboard.

Rules:

- keep exactly one background cloud loop
- use plain HTTP request/response
- batch undelivered events into check-ins
- send with an idempotency key based on local event id
- retry with exponential backoff and jitter
- never delete local events until the server acknowledges them
- keep a retention window after upload for local diagnostics
- compact old delivered events only when safe
- surface check-in state in the local UI and `vibeengine status`
- do not create separate hidden pollers for updates, remote actions, or config
- keep request payloads explicit and inspectable
- never execute a new or changed command just because the server returned it

The hosted API must be idempotent. If the same event is uploaded twice, it
should be accepted once and treated as already delivered after that.

## Remote Control, Config, And Security

Remote command/config control is the hard part because most developer machines
cannot receive inbound requests from the hosted service. Avoid pretending the
cloud can push directly to localhost.

The transport is the same HTTP check-in loop, but execution policy must be
local-first and security-first:

- local engine owns the command that runs on the machine
- hosted dashboard writes desired actions/config proposals to the server
- local engine receives proposals on its next check-in
- local engine stores proposals locally
- local engine applies only safe actions automatically
- local engine requires local approval before executing new shell command text
- local engine reports approval, rejection, or result on the next check-in, or
  immediately after the decision

With a 15-30 second cadence and immediate check-ins after local activity, this
is simple and good enough for most pause/restart/config use cases. The hosted
UI should show honest state:

```text
Sent to device
Waiting for next check-in
Waiting for local approval
Approved locally 4s ago
Applied 3s ago
Failed: command not found
```

There should be no separate remote-action transport. Use the same check-in
endpoint for event upload, config fetch, command actions, and update notices.

### Action Classes

Not every remote action has the same risk.

Safe to apply automatically:

- acknowledge uploaded events
- update cloud delivery metadata
- update non-executable display metadata
- report status

Allowed only against a locally approved command:

- pause command
- resume command
- restart command
- health check the current approved target
- enable or disable command automation when the command text is already locally
  approved

Requires explicit local approval:

- new shell command text
- new shell health-check command text, if shell health checks are supported

Browser URL health checks are not executable code and do not need the same
approval gate. Hook installation is also not treated as a cloud-pushed command:
hooks are installed by the local CLI/engine and are visible in the project.

The server must never be able to silently turn a synced config change into code
execution on every user machine. If the hosted service is compromised, the blast
radius must stop at "pending changes visible locally", not "malware executed".

### Engine Updates

Automatic engine updates can be safe, but only if the server cannot forge them.

Allowed automatic update model:

- update artifacts are signed by an offline release key
- the installed engine contains the trusted public key
- the engine verifies the signature before installing
- failed signature means no install
- update channel is explicit, for example stable or beta
- status shows current version and last update result

This is similar to normal desktop app updates. A compromised database or hosted
API should not be enough to ship arbitrary code to every machine.

Signing key policy:

- commit only public release verification keys
- never commit private signing keys
- keep the production private key in a password manager or dedicated signing
  system
- during development, local temporary private keys are allowed but must stay
  ignored by git
- if a private key is ever committed or leaked, rotate the release key and treat
  old clients as needing a trust update

The repository `.gitignore` should block common local private-key filenames such
as `*.private.key`, `*.signing.pem`, and `release-signing-private.*`.

### Local Approval UX

When a risky command change is pending, use `vibeengine sync` for the intentional
review/apply flow. `vibeengine status` can show that approval is needed, but it
should not be the interactive approval surface.

Example:

```text
$ vibeengine sync
Project: bet1
Cloud: connected
Pending local approval:

  Change command

  From:
    USE_PROXY=1 ./ops/local/start.sh prod

  To:
    curl https://example.invalid/install.sh | sh

This will execute on your machine.
Type the exact project name to approve, or press Enter to reject:
```

Do not make this a casual yes/no prompt. Approval should be hard to misclick:

- show the complete command
- show who/what requested the change when available
- require typing the project name or a short confirmation phrase
- record the approval decision locally
- upload the decision to the server

Hosted UI should also say this clearly:

```text
Command change saved.
Security: this will not run until approved on the developer machine.
Run `vibeengine sync` locally to review and apply it.
```

## CLI UX

Programmers should be able to understand the system from the terminal.

The normal read-only command should be:

```bash
vibeengine status
```

It should include health checks, local stats, engine state, and cloud delivery
state in one place. The command should diagnose, upload, retry, and repair
automatically wherever it can. It should not approve or execute pending risky
command changes.

The intentional review/apply command should be:

```bash
vibeengine sync
```

Use it when the hosted platform has pending changes that require local trust.
This command can fetch pending proposals, show the full command diff, ask for
confirmation, apply approved changes, and upload the decision/result.

Example healthy output:

```text
$ vibeengine status
Engine: running
Hooks: installed for this repo
Cloud: connected as alice@example.com
Queue: clear, 184 uploaded today
Command: paused
Last prompt: finished 2m ago
Today: 44 prompts, 1h 33m AI time, 2m median depth
```

Example degraded output:

```text
$ vibeengine status
Engine: running
Local API: ok http://127.0.0.1:8766
Codex hooks: ok .codex/hooks.json
Cloud API: unreachable
Queue: 12 pending, retrying automatically
Last check-in: failed 1m ago
Next check-in: in 2m
Suggestion: check internet access or proxy settings.
Local events will upload when the connection returns.
```

If the background engine is not running, status should be explicit:

```text
$ vibeengine status
Hooks: installed for this repo
Engine: not running
Queue: 3 local events waiting
Cloud: will upload when engine starts
Command automation: unavailable
Suggestion: start engine now? It is normally started by Codex SessionStart.
```

Avoid separate normal-use commands. No manual doctor command, no manual upload
command, no manual stats command, and no manual "flush queue" workflow.
`vibeengine sync` exists only for security-sensitive review/apply flows.
Advanced debug commands can exist later if truly needed, but the product should
be designed so `vibeengine status` is enough for everyday understanding.

The status output should avoid vague failure messages. Every problem should
explain:

- what failed
- whether local tracking still works
- what data is waiting
- the next action

### Status Network Diagnostics

`vibeengine status` should actively test cloud reachability. This matters
because many users run Codex inside dev containers, corporate networks, VPNs, or
other protected environments.

Status should check:

- API hostname resolves
- HTTPS connection to the API can be opened
- TLS certificate verifies
- auth token exists and is accepted
- API returns a valid response
- queued event count
- last successful upload time
- next retry time

It should distinguish common failures:

- DNS lookup failed
- TCP timeout to `api.vibeengine.com:443`
- proxy required or proxy variables missing
- TLS/corporate CA problem
- auth token missing or expired
- API reachable but returning an error

Example protected-container output:

```text
$ vibeengine status
Engine: running
Hooks: installed
Local capture: ok
Cloud: unreachable
Queue: 7 pending events
Last upload: 42m ago
Next retry: 2m

Problem:
  api.vibeengine.com:443 timed out from this environment.

Local behavior:
  Prompt tracking continues locally.
  Command automation continues locally.
  Cloud dashboard will update after connectivity returns.

Fix:
  Allow outbound HTTPS to api.vibeengine.com:443
  or set HTTPS_PROXY in this dev container.
```

The tone should be precise and non-alarming. The user should know what still
works locally and exactly what to change if they want cloud upload working.

## Install UX

The hosted platform should give one command per project. The command should use
a fresh bootstrap token every time the user views or copies it.

```bash
curl -fsSL https://vibe.example/install | sh -s -- \
  --bootstrap <one-time-token> \
  --project <project-id>
```

The install command is also the repair command. If anything feels broken, the
user should be able to copy the install command again and run it safely.

The bootstrap token should be:

- short-lived, for example 10-15 minutes
- single-use
- scoped to one account and one project
- harmless if an old copied command expires
- exchangeable only for local device/project registration

Every install run should:

- download the latest stable installer/engine
- verify signed artifacts before installing
- update or repair the local engine without deleting local data
- exchange the bootstrap token for device/project credentials
- reuse existing local auth when it belongs to the same account
- stop clearly if the machine is logged into a different account
- detect the current repo
- register or update the project-to-repo mapping
- install or repair project-local hooks
- remove old generated hooks for that project while preserving manual hooks
- start or repair the background engine if needed
- run the same checks shown by `vibeengine status`
- print a short success summary

Example final output:

```text
Vibeengine ready

Account: alice@example.com
Project: bet1
Engine: running v1.4.2
Hooks: installed
Cloud: connected
Queued events: 0
Command automation: enabled

Next: start coding.
```

Multiple projects on one machine should be normal. Device/account auth should be
separate from project registration:

- one device credential per account on the machine
- many project registrations can use that device credential
- project registration is upsert-style
- re-running install for a project is idempotent
- concurrent installs are protected by SQLite transactions or a local file lock

Do not require users to manually edit hook JSON for normal setup.

Product promise:

```text
Copy install command again -> run it -> repaired.
```

## Failure Modes

The product should be designed around failure being normal.

### Hosted API Down

Expected behavior:

- hooks still complete quickly
- local event queue grows
- `vibeengine status` shows pending event count
- upload resumes automatically later

### Local Engine Down

Expected behavior:

- SessionStart hook tries to start it
- if startup fails, hook exits quickly with a useful status
- `vibeengine status` explains how to recover

### Duplicate Hooks

Expected behavior:

- hooks are project-scoped
- generated hooks are replaceable by stable app/project ids
- duplicate events are deduped locally or by cloud idempotency keys

### Disk Full

Expected behavior:

- local engine reports a clear error
- hooks should fail fast rather than hang
- `vibeengine status` explains that events cannot be safely recorded

## What Must Feel Polished

Quality bar:

- hooks should not make Codex feel slower
- no endless spinners
- no hidden queue growing forever without visibility
- one visible check-in loop, not scattered hidden network behavior
- no unclear "runner disconnected" language when command automation is off
- no cloud dependency for local prompt stats
- every status should say what is happening in humane words
- diagnostics should be explicit enough that a developer trusts the tool

## Initial Milestones

1. Keep local Go engine stable and small.
2. Add durable outbound event queue locally.
3. Add delivery status to local UI and `vibeengine status`.
4. Add project/account token model.
5. Build hosted ingest API with idempotent event upload.
6. Build hosted dashboard for prompt stats only.
7. Add HTTP check-in endpoint for upload, config, actions, and updates.
8. Add hosted dashboard actions that resolve through check-in.
9. Add install script that installs engine and hooks, then prints status.
10. Add team/project views.
11. Add billing only after the core loop is reliable.

## Principle

Local first, cloud uploaded.

The hook path should be local, durable, and fast. The cloud should make the data
more useful, not make the local workflow fragile.
