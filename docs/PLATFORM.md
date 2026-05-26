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
- personal projects
- onboarding and install scripts
- web dashboard
- cross-machine/project stats
- billing and plan limits later
- cloud API for uploaded events
- project tokens and device registration
- long-term analytics and reporting

The platform is the collaboration and visualization layer. It should not be
required for local hooks to complete.

V1 should not include teams, workspaces, seats, invites, or role management.
The first product should be a personal developer account:

- one user account
- one personal subscription
- unlimited projects within reasonable abuse limits
- unlimited local installs within reasonable abuse limits
- project names unique inside that user's account

Future workspace support can be added later for companies, agencies, or shared
dashboards. That future model can use per-seat billing, where the workspace pays
for active members. The local engine protocol should keep using neutral
`account_id`/`project_id` concepts so this future layer does not require a
rewrite, but the V1 UI and product model should stay single-user.

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

Local SQLite should stay small. It is a local queue/cache for the engine, not a
mirror of the hosted Postgres database.

Suggested local tables:

- `local_kv`
  - device id
  - account id/email
  - engine version
  - last check-in
  - next check-in
  - last cloud error
- `projects`
  - project id
  - project name
  - account id
  - repo path
  - hook install state
  - command automation state
  - approved command hash
  - current local status
- `events`
  - local event id
  - project id
  - event type
  - payload JSON
  - UTC created time
  - delivery attempts
  - delivered time
  - last delivery error
- `pending_actions`
  - action id
  - project id
  - action type
  - payload JSON
  - pending/applied/failed/reported status
  - result message
- `pending_approvals`
  - proposal id
  - project id
  - proposal type
  - summary
  - payload JSON
  - pending/approved/rejected/applied/failed/reported status
- `approved_commands`
  - project id
  - approved command hash
  - local command preview
  - approved time
  - source proposal id

Auth tokens should live separately from ordinary metadata, for example a
permission-restricted auth file. They should not be mixed into the normal event
queue.

The hosted database is different. In V1, server Postgres owns users, accounts,
personal subscriptions, billing, uploaded events, hosted project records, audit
logs, remote action records, and proposal history. Teams, workspaces,
memberships, roles, and per-seat billing are future concepts, not V1
requirements.

Use SQLite WAL mode locally. Do not keep the only copy of events in memory.

## Project Identity And Local Storage

Projects should feel name-based to users, but use stable ids internally.

User-facing rules:

- project names are unique inside the user's account
- users see and type project names, for example `bet1`
- hooks can show the project name:
  ```bash
  vibeengine hook prompt-started --project bet1
  ```
- hosted UI may use nice project slugs
- users should not need to know a project id

Internal rules:

- every project still has an immutable `project_id`
- bootstrap tokens target the internal `project_id`
- generated hooks or local hook mappings keep the stable `project_id` internally
- database foreign keys and audit records use ids, not names
- project names can be renamed without breaking history
- the CLI resolves project names through local state
- a deleted project name can be reused without old events merging into the new
  project

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

## Project Lifecycle

Do not add archive as a normal product concept. Archive adds another state,
another filter, and another decision without giving the user much value. If
history is never lost and deleted names are immediately reusable, the user can
delete a project and create a new project with the same name in seconds.

Keep the lifecycle simple:

```text
active
deleted
```

Delete should not mean "destroy history". Delete means:

- remove the project from the active project list
- free the project name so it can be used again
- stop normal active status and command/control behavior for that project
- keep historical stats
- keep accepting old queued events for that `project_id` as historical backfill
- never reuse the old `project_id`

This protects the core stats promise. A local engine may have captured events
while offline and uploaded them later. Those events should not be lost just
because the user deleted the project before the queue finished uploading.

Name reuse example:

```text
Old bet1 = prj_123, deleted
New bet1 = prj_999, active
```

Old queued events for `prj_123` still attach to the deleted project's history.
New events for `prj_999` attach to the new active project. The same display name
must not merge histories.

Server behavior after deletion:

- hide the project from default active views
- show it only in historical stats or deleted-project history views if needed
- disable remote actions for the deleted project
- disable command/config proposals for the deleted project
- accept idempotent uploads for old local events tied to the old `project_id`
- return project state `deleted` in check-in responses

Local engine behavior after learning a project was deleted:

- stop command automation for that project
- stop live status reporting for that project
- keep already queued local events and attempt upload as history
- make generated hooks harmless and fast
- show a clear status message

Example:

```text
$ vibeengine status
Project: bet1
State: deleted in web app
Local capture: stopped
Command automation: off
Cloud: historical upload only
Queued local events: 3

Fix:
  Create a new project in the web app and run its install command in this repo.
```

Permanent deletion is a separate account/privacy operation, not normal project
delete. If the user deletes their account or makes a legal data deletion
request, that should follow the account deletion policy and purge hosted data as
required.

## Data Upload Policy

Default upload is metadata-only.

Upload by default:

- account, device, and project ids
- event type
- UTC event timestamps
- UTC check-in timestamps
- durations derived from timestamps
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
machines, or regions.

## Hosted Dashboard V1

The V1 dashboard should be live-first and simple. It should not copy the old
local UI structure or expose dense per-project statistics.

Visual direction:

- black background
- modern, polished, quiet interface
- content grouped into clean islands/panels
- simple project cards
- no decorative clutter
- no dense table-like project list
- no weird wrappers around basic actions
- modals/popovers use dimmed or blurred background treatment
- controls should feel intentional and direct, not nested or overexplained

Top-level layout:

- global statistics at the top
- user account button in the top-right corner
- search bar under the statistics
- add project button near the search/create flow
- browser sound status near the search/create area
- compact project cards under the search bar
- project management in an expanded side panel after clicking a project

Global statistics should summarize the account, not each project card:

- today's prompt time
- prompt count
- active/running project count
- cloud delivery health if there is a problem

Do not show permission count as a V1 metric. It is not useful enough to deserve
dashboard space.

Project cards should stay compact and calm:

- project name
- short status under the name
- enough spacing to feel intentional, not squeezed into a single dense row
- no per-project charts
- no command settings directly on the card

Clicking a project should open a side panel for management:

- current status
- install/repair command
- command automation settings
- health check settings
- pending local approval state
- delete project

Account button:

- lives in the top-right corner
- opens a compact account popover
- uses a blurred/dimmed background behind the popover
- contains account/subscription basics and sign-out/security actions
- does not navigate away from the dashboard for ordinary account actions

Add project flow:

- clicking `+ Project` opens a small modal
- the modal asks only for project name
- primary action is `Create`
- no install command inside the creation modal
- after create, the project appears in the project list
- install/repair command is available from the project side panel

Sound readiness:

- show browser sound status near search/create controls
- if the browser needs a user gesture before sounds can play, show a clear
  action such as `Enable sound`
- once enabled, the status should become quiet and not take dashboard attention
- sound status is browser/UI state, not project state

Avoid V1 dashboard bloat:

- no teams
- no seats
- no prompt content
- no command logs
- no repo/file metadata
- no device management screen
- no per-project statistics panels
- no complex analytics charts before the data model is proven

The dashboard should answer two questions quickly:

```text
What is happening overall?
Which project needs attention?
```

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
- if subscription is inactive, slow down to a tiny entitlement check, for
  example once per day
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

### Subscription Gating

The server should not keep accepting normal check-ins from inactive accounts.
Old installed engines should not become background load on the service.

Subscription state should not be only active/inactive. Billing can be processing
or temporarily uncertain during renewals.

Entitlement states:

- `active`: full service
- `grace`: renewal or billing state is temporarily processing; full service
  continues
- `past_due`: payment failed but is still inside the grace window; service
  continues with a warning
- `inactive`: grace expired or subscription ended; cloud features pause
- `canceled`: user canceled, but service remains active until paid period end

Billing uncertainty should never immediately stop check-ins. Only final
`inactive` state pauses cloud upload, hosted history, and remote actions.

When subscription is inactive, the server should return a small entitlement-only
response:

```json
{
  "entitlement": {
    "status": "inactive",
    "features": {
      "cloud_upload": false,
      "remote_actions": false,
      "hosted_history": false
    }
  },
  "next_check_in_seconds": 86400,
  "message": "Subscription inactive"
}
```

Local behavior when inactive:

- local hooks still work
- local command automation still works
- cloud upload pauses
- remote actions are disabled
- hosted history/dashboard updates pause
- local events are kept only up to the local retention limit
- automatic check-in slows to a rare entitlement check, for example once per day
- `vibeengine status` can force an immediate entitlement check

Example:

```text
$ vibeengine status
Account: alice@example.com
Subscription: inactive
Cloud: paused
Next automatic subscription check: tomorrow
Local capture: ok
Command automation: ok
Queued local events: 18

Fix:
  Renew subscription to resume cloud dashboard and upload.
```

If the user renews, the engine returns to normal check-in either at the next
rare entitlement check or immediately when `vibeengine status` runs.

Grace entitlement example:

```json
{
  "entitlement": {
    "status": "grace",
    "features": {
      "cloud_upload": true,
      "remote_actions": true,
      "hosted_history": true
    },
    "message": "Payment renewal is processing. Service continues.",
    "grace_until": "2026-06-02T00:00:00Z"
  }
}
```

Past-due entitlement example:

```json
{
  "entitlement": {
    "status": "past_due",
    "features": {
      "cloud_upload": true,
      "remote_actions": true,
      "hosted_history": true
    },
    "message": "Payment failed. Service continues until grace period ends.",
    "grace_until": "2026-06-02T00:00:00Z"
  }
}
```

### Check-In API Contract

Initial endpoint:

```text
POST /engine/check-in
Authorization: Bearer <access_token>
```

Request example:

```json
{
  "protocol_version": 1,
  "device": {
    "id": "dev_123",
    "engine_version": "1.4.2",
    "os": "linux",
    "arch": "arm64"
  },
  "account": {
    "id": "acct_123"
  },
  "projects": [
    {
      "id": "prj_123",
      "name": "bet1",
      "hooks_installed": true,
      "command_automation": true,
      "approved_command_hash": "sha256:...",
      "status": "paused",
      "queue": {
        "undelivered_events": 0,
        "pending_approvals": 0
      }
    }
  ],
  "events": [
    {
      "id": "evt_local_001",
      "project_id": "prj_123",
      "type": "prompt_finished",
      "created_at": "2026-05-25T20:01:04Z",
      "payload": {
        "duration_ms": 42000,
        "result": "success"
      }
    }
  ],
  "action_results": [
    {
      "action_id": "act_123",
      "project_id": "prj_123",
      "status": "applied",
      "finished_at": "2026-05-25T20:01:05Z",
      "message": "command paused"
    }
  ],
  "approval_results": [
    {
      "proposal_id": "prop_123",
      "project_id": "prj_123",
      "status": "approved",
      "decided_at": "2026-05-25T20:01:05Z"
    }
  ]
}
```

Response example:

```json
{
  "server_time": "2026-05-25T20:01:06Z",
  "next_check_in_seconds": 20,
  "entitlement": {
    "status": "active",
    "plan": "pro",
    "features": {
      "cloud_upload": true,
      "remote_actions": true,
      "hosted_history": true
    }
  },
  "acknowledged_events": [
    "evt_local_001"
  ],
  "actions": [
    {
      "id": "act_456",
      "project_id": "prj_123",
      "type": "pause_command",
      "created_at": "2026-05-25T20:01:06Z"
    }
  ],
  "proposals": [
    {
      "id": "prop_456",
      "project_id": "prj_123",
      "type": "change_command",
      "created_at": "2026-05-25T20:01:06Z",
      "requires_local_approval": true,
      "summary": "Change command",
      "new_command": "npm run dev",
      "new_command_hash": "sha256:..."
    }
  ],
  "updates": [
    {
      "version": "1.4.3",
      "channel": "stable",
      "url": "https://vibe.example/releases/vibeengine-1.4.3-linux-arm64",
      "sha256": "...",
      "signature": "..."
    }
  ]
}
```

Contract rules:

- all timestamps are UTC
- local events are idempotent by local event id
- server acknowledgements are per event id
- actions are idempotent by action id
- proposals are idempotent by proposal id
- proposals are not executed until locally approved
- response must not include executable command changes as ordinary actions
- command text is not sent in normal events or status summaries
- command text may appear in proposals because the local user must review it
- inactive subscription can authenticate successfully while disabling features
- unacknowledged local events stay local until acknowledged or retention policy
  expires

Inactive entitlement example:

```json
{
  "entitlement": {
    "status": "inactive",
    "plan": "pro",
    "features": {
      "cloud_upload": false,
      "remote_actions": false,
      "hosted_history": false
    }
  }
}
```

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

### Command Settings Contract

The web UI may let the user create and edit command settings, but saving a
setting in the cloud is not the same as applying executable code locally.

State language should be explicit:

```text
Saved in cloud
Waiting for device
Pending local approval
Approved locally
Applied locally
Rejected locally
```

Rules:

- web UI can save command proposals
- new shell command text never runs automatically from the cloud
- new shell command text requires `vibeengine sync` on the developer machine
- new shell health-check command text also requires `vibeengine sync`
- browser URL health checks are not shell execution and do not need the same
  approval gate
- disabling command automation from the web can apply automatically
- enabling command automation can apply automatically only when the exact command
  hash is already locally approved
- pause, resume, and restart can apply automatically only for an already locally
  approved command
- editing an already-approved command creates a new pending proposal and does
  not change the running local command until approved
- if the engine is offline, the web UI shows the proposal as saved but not yet
  seen by the device
- if the engine is online but the proposal is unapproved, the UI shows pending
  local approval
- if command automation is disabled, dependent controls such as pause during
  prompt, auto-pause after ready, and command health checks should appear muted,
  not erased

Health checks follow the command model:

- URL/browser health checks can be configured from the web without local command
  approval
- shell health checks are executable code and require local approval
- command health checks do not run when command automation is disabled
- command health checks do not run for a deleted project

The UI must not imply that a command is active just because a proposal was saved
in the cloud. It should show where the setting currently lives:

```text
Cloud: saved
Device: pending approval
Running command: old approved command
```

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

## Engine Credentials And Revocation

Do not build user-facing device management in the first product version. Users
should not need to name devices, browse device lists, or understand which laptop
is "MacBook Pro 3".

Internally, the server still needs engine credentials so installs can be revoked
cleanly.

Credential model:

- one account can have unlimited installed engines
- each install receives its own refresh token
- the server stores only a hash of the refresh token
- access tokens are short-lived, for example 5-15 minutes
- refresh tokens live in the local restricted auth store, separate from event
  metadata
- credentials keep minimal server metadata:
  - credential id
  - account id
  - user id
  - refresh token hash
  - created time
  - last seen time
  - revoked time

This is not a device-management product surface. It is just enough state to make
auth secure, debuggable, and revocable.

### Disconnect Installs

The hosted UI should expose a simple security action:

```text
Disconnect my installs
```

It should not require the user to manage individual devices. For the first
version, expose only user-scoped disconnect. Account-wide disconnect can exist
as an internal/admin security operation later.

User-scoped behavior:

- revoke only credentials created by the current user
- do not affect other users in a future shared workspace
- keep project data untouched
- keep other future workspace members' engines connected

Account-wide behavior:

- admin/security action only
- future workspace feature, not V1 UI
- revokes all engine credentials in the account/workspace
- useful after suspected account or workspace compromise
- should require stronger confirmation and audit logging

Server behavior:

- mark matching engine credentials as revoked
- bump a user-level `engine_auth_epoch` for user-scoped disconnect
- optionally bump an account-level `engine_auth_epoch` for account-wide
  emergency disconnect
- reject refresh attempts from older credentials
- reject existing access tokens whose epoch no longer matches
- keep audit records for the security action

The epoch is important because deleting refresh tokens alone does not invalidate
already-issued JWT access tokens until they expire. Use both:

- short access-token TTL to limit exposure
- account/user token epoch to invalidate active tokens immediately

Engine access tokens should include enough claims for server-side revocation:

```text
user_id
account_id
credential_id
user_engine_auth_epoch
account_engine_auth_epoch
```

Every engine API request should verify:

- JWT signature is valid
- credential exists
- credential has not been revoked
- token user epoch matches the current user epoch in the database
- token account epoch matches the current account epoch in the database
- subscription/entitlement still allows the requested cloud feature

This means revocation is immediate from the server's point of view. The local
engine does not need a push notification. It discovers the disconnect on its
next check-in or token refresh.

Example revoked request:

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json

{
  "code": "engine_signed_out",
  "message": "This install was disconnected. Run the install command again."
}
```

If the engine is offline, it cannot learn about the disconnect until it comes
back online. That is acceptable because offline engines cannot use cloud
features anyway. Local hooks and command automation can continue under the local
contract.

Local engine behavior after revocation:

- stop cloud upload
- stop remote actions
- keep local hooks working
- keep local command automation working
- keep local queued events up to the retention limit
- mark the account as signed out locally
- do not retry aggressively forever
- tell the user to run the install command again from the web app

Example:

```text
$ vibeengine status
Account: signed out
Engine: running
Hooks: installed
Local capture: ok
Cloud: disconnected
Command automation: ok
Queued local events: 12

Fix:
  Open the web app, copy the install command, and run it in this repo again.
```

### Reconnect After Revocation

Re-running the install command is the recovery path.

The installer should:

- exchange a fresh one-time bootstrap token
- receive a new engine refresh token
- register or update the local project mapping
- leave existing local queued events intact
- resume upload after auth succeeds

If the local machine was previously connected to a different account, the
installer should stop clearly and require an explicit local choice. It should not
silently move local projects between accounts.

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
6. Build hosted dashboard with global stats, search, compact project cards, and
   project side panel.
7. Add HTTP check-in endpoint for upload, config, actions, and updates.
8. Add hosted dashboard actions that resolve through check-in.
9. Add install script that installs engine and hooks, then prints status.
10. Add project views.
11. Add billing only after the core loop is reliable.

## Principle

Local first, cloud uploaded.

The hook path should be local, durable, and fast. The cloud should make the data
more useful, not make the local workflow fragile.
