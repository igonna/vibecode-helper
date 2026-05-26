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

### Implementation Stack

Use a split stack:

- local engine: Go
- hosted web app/API: Next.js App Router with TypeScript
- styling: Tailwind CSS
- icons: lucide-react
- database: Postgres
- ORM/migrations: Drizzle ORM
- payments: YooKassa
- auth: custom email-code auth
- deployment: Docker Compose on a VPS or cloud VM
- reverse proxy/TLS: Caddy or Nginx
- Redis: optional later for distributed rate limiting, not required on day one

The `vibeengine` local engine should stay Go because it needs to be a small,
fast, cross-platform local binary. It owns hooks, local SQLite, command
automation, health checks, local status, updates, and cloud check-in.

The hosted platform should be TypeScript/Next.js because most product work is
web UI, account flows, project settings, billing, and API route handlers.

Drizzle ORM is the TypeScript database layer for Postgres. It gives typed SQL,
schema definitions, and migrations without hiding the database too much. Use it
for the hosted Postgres schema, not for the local engine SQLite unless there is
a separate reason later.

Prefer Drizzle over Prisma for this product because the backend needs precise
control over SQL-heavy workflows:

- idempotent event ingest
- check-in state
- subscription state
- token and credential revocation
- pending actions and proposals
- export jobs
- rate-limit counters
- webhook idempotency
- audit/security records

Prisma is a good CRUD-oriented ORM, but it is a heavier abstraction with its own
schema language and generated client. Drizzle is thinner, closer to SQL, easier
to mix with raw SQL, and easier to reason about for queues, locks, upserts,
partial indexes, and analytics-style queries.

Avoid external hosted auth/payment assumptions in V1:

- no Clerk/Auth0 dependency
- no Stripe dependency for the Russian launch
- no Supabase requirement

Custom email-code auth keeps login simple:

```text
email -> one-time code -> signed session cookie
```

YooKassa should handle subscription payments:

- 14-day trial starts without card
- subscribe flow creates a YooKassa payment
- successful payment saves a reusable payment method when supported
- YooKassa webhooks update subscription state
- monthly billing charges the saved payment method
- the server stores YooKassa customer/payment references, never card data

Keep everything in one Next.js app first:

```text
/dashboard
/login
/api/engine/check-in
/api/install/bootstrap
/api/yookassa/webhook
/api/export
```

If check-in traffic outgrows the web app, move `/api/engine/check-in` to a
separate service later. Do not split services before there is load to justify
it.

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

Local SQLite V1 tables:

- `local_kv`
  - key
  - value
  - updated_at
- `projects`
  - project_id
  - account_id
  - name
  - repo_path
  - state
  - hooks_installed_at
  - last_cloud_seen_at
  - last_status
- `events`
  - local_event_id
  - local_sequence
  - project_id
  - session_id
  - prompt_instance_id
  - type
  - payload_json
  - created_at_utc
  - delivery_state: pending/delivered/dropped
  - delivery_attempts
  - delivered_at
  - last_error
  - supervisor_processed_at
- `pending_actions`
  - action_id
  - project_id
  - source: cloud/hook/timer/local
  - idempotency_key
  - type
  - target_command_hash
  - payload_json
  - state: pending/processing/applied/failed/reported
  - attempt_count
  - lease_until
  - created_at
  - applied_at
  - result_message
- `active_prompts`
  - prompt_instance_id
  - project_id
  - session_id
  - started_event_id
  - finished_event_id
  - started_at_utc
  - finished_at_utc
  - stale_at_utc
  - state: active/finished/stale
  - pause_requested
- `pending_approvals`
  - proposal_id
  - project_id
  - type
  - summary
  - payload_json
  - state
  - created_at
  - decided_at
- `approved_commands`
  - project_id
  - command_hash
  - command_preview
  - approved_at
  - source_proposal_id
- `command_state`
  - project_id
  - automation_enabled
  - runtime_state: disabled/stopped/starting/running/ready/pausing/paused/restarting/failed
  - run_id
  - state_version
  - process_id
  - process_group_id
  - prompt_state: idle/prompting/stale
  - active_prompt_ids_json
  - last_prompt_event_at
  - paused_by
  - ready_since
  - pause_during_prompt
  - auto_pause_after_ready
  - health_mode
  - health_state
  - restart_attempts
  - next_restart_after
  - last_error
  - last_transition_at
- `update_state`
  - current_version
  - channel
  - last_check_at
  - last_result
  - previous_version
  - rollback_path

Auth tokens should live separately from ordinary metadata, for example a
permission-restricted auth file. They should not be mixed into the normal event
queue.

The hosted database is different. In V1, server Postgres owns users, accounts,
personal subscriptions, billing, uploaded events, hosted project records, audit
logs, remote action records, and proposal history. Teams, workspaces,
memberships, roles, and per-seat billing are future concepts, not V1
requirements.

Hosted Postgres V1 tables:

- `users`
  - id
  - email
  - email_verified_at
  - engine_auth_epoch
  - created_at
  - deleted_at
- `accounts`
  - id
  - owner_user_id
  - name
  - created_at
  - deleted_at
- `sessions`
  - id
  - user_id
  - session_hash
  - expires_at
  - created_at
  - revoked_at
- `email_login_codes`
  - id
  - email
  - code_hash
  - purpose: login/delete_account
  - expires_at
  - attempt_count
  - consumed_at
  - created_at
- `projects`
  - id
  - account_id
  - name
  - state: active/deleted
  - created_at
  - deleted_at
  - last_seen_at
  - last_status
- `project_events`
  - id
  - account_id
  - project_id
  - local_event_id
  - type
  - occurred_at
  - duration_ms
  - result
  - status
  - created_at
- `daily_project_stats`
  - account_id
  - project_id
  - day_utc
  - prompt_count
  - prompt_duration_ms
  - success_count
  - failure_count
  - updated_at
- `engine_credentials`
  - id
  - account_id
  - user_id
  - refresh_token_hash
  - engine_auth_epoch
  - created_at
  - last_seen_at
  - revoked_at
- `bootstrap_tokens`
  - id
  - account_id
  - project_id
  - user_id
  - token_hash
  - expires_at
  - consumed_at
  - created_at
- `remote_actions`
  - id
  - account_id
  - project_id
  - type
  - target_command_hash
  - state: pending/sent/applied/failed
  - created_at
  - sent_at
  - finished_at
  - result_message
- `command_proposals`
  - id
  - account_id
  - project_id
  - type
  - state: pending_device/pending_local_approval/approved/rejected/applied/failed
  - payload_json
  - command_hash
  - created_at
  - seen_at
  - decided_at
  - applied_at
- `subscriptions`
  - id
  - account_id
  - state: trialing/active/grace/past_due/inactive/canceled
  - plan
  - price_rub
  - trial_ends_at
  - current_period_end
  - yookassa_customer_id
  - yookassa_payment_method_id
  - created_at
  - updated_at
- `webhook_events`
  - id
  - provider
  - provider_event_id
  - payload_json
  - received_at
  - processed_at
  - processing_error
- `billing_events`
  - id
  - account_id
  - provider
  - provider_payment_id
  - type
  - amount_rub
  - state
  - occurred_at
  - created_at
- `export_jobs`
  - id
  - account_id
  - state: pending/running/ready/failed/expired
  - file_path_or_object_key
  - expires_at
  - created_at
  - finished_at

Important hosted constraints:

- active project names are unique per account:
  `unique(account_id, lower(name)) where state = 'active'`
- uploaded events are idempotent:
  `unique(project_id, local_event_id)`
- daily stats are unique per project/day:
  `unique(project_id, day_utc)`
- webhook events are idempotent:
  `unique(provider, provider_event_id)`

For V1, command proposal payloads may contain command text because the user
explicitly typed it in the web UI and the local machine must review it. Command
text must not appear in normal events, exports, logs, or status summaries.

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
delete. Project delete keeps hosted history.

## Account Deletion, Export, And Privacy

V1 should include a limited safe data export, but not a raw database export and
not a per-project erase-history feature. Keep the privacy model simple:

- project delete keeps history and stats
- account export provides user-facing history and stats
- account delete is the destructive hosted-data deletion path
- local data on the user's machine is not remotely wiped

### Data Export

Data export should be useful for the user without leaking sensitive operational
or security data. It should export product data, not internals.

Export should include:

- account email
- account creation time
- subscription status and plan name
- project list, including active and deleted projects
- project names
- project created/deleted timestamps
- project status history at a high level
- prompt event timestamps
- prompt durations
- prompt result statuses such as success, failure, or skipped
- aggregated daily/weekly/monthly stats
- command automation enabled/disabled history
- health check mode/result history without sensitive payloads
- cloud delivery status summaries

Export should not include:

- refresh tokens or access tokens
- token hashes
- engine credential records
- bootstrap tokens
- signing keys
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
- raw action/proposal payloads
- internal audit/security records
- payment processor raw objects
- internal database ids unless needed to distinguish projects

V1 export format:

- one ZIP file named `vibeengine-export-YYYY-MM-DD.zip`
- `account.json` for account, subscription, and export metadata
- `projects.json` for active/deleted project records
- `events.csv` for prompt event timestamps, durations, result statuses, and
  project references
- `daily_stats.csv` for aggregated daily project/account stats

Do not offer multiple export formats in V1. A ZIP with JSON metadata and CSV
tables is easy to inspect, easy to import into spreadsheets, and stable enough
for support/debugging.

Export should be available from the account popover/settings flow. The export
link should be short-lived and require the user to be signed in. If abuse or
sensitivity becomes a concern, require an email code confirmation before
generating the export.

Account deletion should require strong confirmation:

- user starts delete account from the account popover/settings flow
- server sends a one-time confirmation code to the account email
- user must enter the email code
- user must also type the account email or an explicit confirmation phrase
- deletion is blocked if the confirmation code is expired or invalid

Account deletion should remove or revoke:

- user account
- active subscription access
- future subscription renewals
- projects
- hosted events and stats
- engine credentials and refresh tokens
- pending remote actions
- pending command/config proposals

If the user deletes an account with an active subscription, account deletion
must also cancel future billing in YooKassa/the billing system. The product
account should be disabled immediately after confirmation, even if payment
provider cancellation/receipt handling finishes asynchronously. Do not keep the
product account alive just because billing cleanup is still processing.

Keep only minimal records required for billing, fraud prevention, legal, tax, or
payment processor obligations.

After hosted account deletion:

- local engines become signed out
- cloud upload stops
- remote actions stop
- local hooks and local command automation may continue only as local behavior
- `vibeengine status` should say the account no longer exists and that the user
  must reconnect with a new install command if they create a new account

## Data Upload Policy

Default upload is metadata-only.

Upload by default:

- account, device, and project ids
- event type
- UTC event timestamps
- UTC check-in timestamps
- durations derived from timestamps
- high-level result such as success, failure, or skipped
- high-level app status such as Prompting, Ready, Running, Paused, Offline,
  Attention, or Setup
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

## Retention Policy

Hosted data:

- hosted events and stats are kept while the account exists
- project delete keeps hosted history and stats
- deleted project names can be reused because old history is tied to immutable
  `project_id`
- account deletion is the hosted-data deletion path
- after account deletion, keep only minimal billing/legal/fraud records required
  by payment processor, tax, accounting, or legal obligations

Local data:

- undelivered local events are kept for up to 30 days or until local DB cap is
  reached
- delivered local events are kept for 7 days for diagnostics
- default local DB cap is 100 MB
- compact old delivered events first
- if cap pressure remains, drop oldest delivered diagnostics next
- drop undelivered events only when unavoidable because the local DB cap or disk
  safety requires it
- never delete undelivered events just because the server is down, rate-limited,
  or temporarily unreachable

Inactive subscription:

- cloud upload pauses
- hosted dashboard/history pauses
- local hooks and local command automation continue
- local queue still follows the same 30-day/100 MB retention cap
- `vibeengine status` should explain that local capture continues but cloud
  upload requires an active subscription

`vibeengine status` should show retention health:

```text
Queue: 24 pending events
Oldest pending: 3h ago
Local DB: 18 MB / 100 MB
Retention: ok
```

Warning example:

```text
Queue: 12,402 pending events
Oldest pending: 27d ago
Local DB: 92 MB / 100 MB
Retention: near limit

Local capture still works, but old queued events may be dropped if this machine
cannot upload before the local retention limit is reached.
```

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

### Status Vocabulary

Project cards should show one primary status. Do not expose runner/internal
mechanics in the card label.

V1 project card statuses:

- `Prompting`: prompt is currently active
- `Ready`: no prompt active, and the project is healthy or does not need command
  automation
- `Running`: command is starting/restarting or waiting for health
- `Paused`: command automation is intentionally paused
- `Offline`: engine has not checked in recently enough to trust live state
- `Attention`: user action needed, permission requested, command failed, health
  failed, or local approval is pending
- `Setup`: project exists in the web app but has never been installed

Do not use these as primary card labels:

- `Idle`
- `Command off`
- `Applying`
- `Runner connected`
- `Runner disconnected`
- `Updating`
- `Checking`

Primary status priority:

```text
Attention
Prompting
Running
Paused
Offline
Setup
Ready
```

`Offline` should beat `Ready` because the dashboard cannot honestly say the
project is ready if the engine has not checked in recently.

Stale-state rule:

- show `Prompting` only when prompt activity is fresh enough to trust
- if the last known prompt state is stale, show `Offline`
- the side panel may show `Last known state: Prompting, 12m ago`
- never let a dead project look actively prompting forever

Color/motion direction:

- `Prompting`: active accent with pulsing dot
- `Ready`: calm good/green or neutral good
- `Running`: blue/active motion
- `Paused`: muted yellow or gray
- `Offline`: gray
- `Attention`: red/orange
- `Setup`: neutral outline

Side panel can show precise secondary details:

- cloud connected
- waiting for local approval
- command paused
- health check passing
- last seen time
- queued event count
- sound enabled

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

Sound rules:

- sound is a browser/account preference, not project config
- no desktop notifications in V1
- play success sound when a prompt finishes successfully
- play attention sound when permission is requested
- play error sound when command automation fails or health check fails
- do not play sound for ordinary check-ins
- do not play sound for successful uploads
- do not play sound for retries/backoff
- do not play sound for config being saved or applied
- do not play sound for project idle/offline state changes unless attention is
  required
- throttle repeated sounds from the same project/reason so one broken project
  cannot spam the browser
- if sound is not enabled by the browser yet, show `Enable sound` instead of
  pretending sound is active

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

### V1 API Shape

Keep the V1 API small and boring. It is an internal product API, not a public
third-party platform API.

Use two auth modes:

- browser routes use signed session cookies
- engine routes use bearer access tokens

Browser routes:

```text
POST /api/auth/request-code
POST /api/auth/verify-code
POST /api/auth/logout
GET  /api/me

GET    /api/projects
POST   /api/projects
GET    /api/projects/:projectId
PATCH  /api/projects/:projectId
DELETE /api/projects/:projectId

POST /api/projects/:projectId/bootstrap-token
POST /api/projects/:projectId/command-proposals
POST /api/projects/:projectId/actions

GET  /api/subscription
POST /api/subscription/subscribe
POST /api/subscription/cancel

POST /api/account/export
GET  /api/account/export/:exportId

POST /api/account/delete/request-code
POST /api/account/delete/confirm

POST /api/yookassa/webhook
```

Engine routes:

```text
POST /api/engine/bootstrap
POST /api/engine/token
POST /api/engine/check-in
```

Route meaning:

- `/api/engine/bootstrap` exchanges a one-time bootstrap token for engine
  credentials
- `/api/engine/token` refreshes a short-lived access token
- `/api/engine/check-in` uploads events/status and receives actions, proposals,
  entitlement, and update notices
- `/api/projects/:projectId/bootstrap-token` creates a fresh one-time install
  token for the browser UI
- `/api/projects/:projectId/command-proposals` saves risky command/config
  changes that require local approval
- `/api/projects/:projectId/actions` creates safe remote actions such as pause,
  resume, or restart
- `/api/account/export` creates the V1 export job
- `/api/account/export/:exportId` downloads the finished short-lived export
- `/api/yookassa/webhook` receives payment events

Rules:

- browser routes never use engine bearer tokens
- engine routes never use browser session cookies
- all mutating browser routes require CSRF protection or strict same-site cookie
  strategy
- all engine routes require bearer token validation, credential revocation
  checks, auth epoch checks, and entitlement checks
- YooKassa webhook handling must validate authenticity and be idempotent
- command text changes are proposals, not direct actions
- safe actions are separate from risky proposals
- project delete is soft delete: free the name, hide active project, keep
  history, stop actions/proposals, allow old queued event backfill

Command proposal example:

```http
POST /api/projects/prj_123/command-proposals
Content-Type: application/json

{
  "type": "change_command",
  "command": "npm run dev"
}
```

Response:

```json
{
  "proposal_id": "prop_123",
  "state": "pending_device",
  "message": "Saved. Run vibeengine sync locally to approve."
}
```

Safe action example:

```http
POST /api/projects/prj_123/actions
Content-Type: application/json

{
  "type": "restart_command"
}
```

Keep out of V1:

- public third-party REST API
- GraphQL
- team/workspace endpoints
- visible device-management endpoints
- admin dashboard endpoints
- outgoing webhooks to user systems

### Security Hardening

V1 security should be practical and strict. Do not store raw secrets, do not log
sensitive payloads, and do not accept unauthenticated state changes.

Token storage:

- generate tokens from at least 32 random bytes
- store only deterministic server-side hashes:
  `hmac_sha256(server_token_secret, token)`
- use this for session tokens, engine refresh tokens, bootstrap tokens, and
  other high-entropy bearer secrets
- for short human email codes, store a slow hash or HMAC plus strict attempt
  limits
- never store raw tokens in Postgres
- never print tokens in logs, status, errors, or exports

Refresh tokens:

- one engine credential gets one refresh-token family
- refresh tokens are long random values
- store only hash
- rotate refresh token on successful use where practical
- revoke the previous token after successful rotation
- if a rotated/revoked token is reused, treat it as token theft and revoke the
  credential family

Access tokens:

- short-lived, 5-15 minutes
- signed JWT or opaque token
- include `credential_id`, `user_id`, `account_id`, and auth epochs
- engine APIs still check credential, revocation, epochs, and entitlement in DB
- JWT signature alone is not enough for engine authorization

Bootstrap tokens:

- one-time use
- 10-15 minute expiry
- scoped to account, project, and user
- stored hashed
- consumed atomically in a DB transaction
- cannot be reused
- rate limited by account, user, and IP
- exchange only for engine credentials, never browser login

Email login and delete-account codes:

- 6-8 digit code or short random code
- expire in 10 minutes
- max 5 verification attempts
- rate limit request and verify by email and IP
- invalidate older unconsumed codes for the same email/purpose when issuing a
  new code
- store only hash
- separate `purpose`, for example `login` and `delete_account`

Browser sessions:

- session cookie is `HttpOnly`
- session cookie is `Secure`
- session cookie is `SameSite=Lax` or `SameSite=Strict`
- session cookie path is `/`
- sessions have idle/absolute expiry
- logout revokes the session server-side
- mutating browser routes require strict SameSite behavior and/or CSRF token
- mutating browser routes should validate `Origin`/`Referer`

YooKassa webhooks:

- validate provider authenticity according to YooKassa's current webhook
  guidance
- store `provider_event_id` in `webhook_events`
- process idempotently
- webhook retries must not double-credit, double-cancel, or double-extend a
  subscription
- when needed, verify payment/subscription state through YooKassa API before
  final entitlement changes
- avoid logging full webhook payloads if they contain personal or payment data

Command proposals:

- command text may be stored in `command_proposals.payload_json` because the user
  typed it into the web UI and the local machine must review it
- command text must not appear in normal logs
- command text must not appear in exports
- command text must not appear in analytics events
- command text must not appear in normal status summaries
- command text is shown only to the authenticated browser user and local
  `vibeengine sync`
- redact command text from errors
- store command hash separately for comparison
- do not claim encryption unless key management is real

Release signing:

- private signing key is never committed
- public verification key is embedded in the engine
- release artifacts are signed before publishing
- engine verifies signature before install/update
- failed verification means no install/update
- key rotation path must exist
- leaked private key means rotate release key and treat old clients as needing a
  trust update

Server secrets:

- production secrets live in environment/secret manager, not the repo
- local development secrets are ignored by git
- rotate secrets if leaked
- redact logs for:
  - tokens
  - authorization headers
  - cookies
  - email codes
  - command text
  - environment variables
  - webhook secrets
  - payment method references unless explicitly needed for billing debugging

Rate-limit security-sensitive routes:

- login code request
- login code verify
- delete-account code request
- delete-account confirm
- bootstrap token creation
- bootstrap token exchange
- engine token refresh
- engine check-in
- export generation
- YooKassa webhook processing if abused

Initial implementation can use Postgres-backed counters. Redis is optional later
for distributed rate limiting.

Minimal audit events:

- login
- logout
- subscribe/cancel
- delete account requested
- delete account confirmed
- export requested
- command proposal created
- command proposal approved/rejected
- disconnect installs
- refresh token reuse detected

Audit logs must not contain command text, tokens, email codes, auth headers,
cookies, environment variables, or webhook secrets.

### Pricing, Trial, And Abuse Limits

V1 should have one simple personal subscription. Launch region is Russia, so the
first public price should be in RUB.

V1 launch pricing:

- 14-day free trial
- no card required for trial
- `399 RUB / month` after trial
- one plan only
- unlimited projects within reasonable abuse limits
- unlimited local installs within reasonable abuse limits

This is intentionally low-friction. The service is cheap to run, and the product
needs users to experience the local engine before paying. Around May 2026,
public exchange-rate references put USD/RUB roughly in the low-to-mid 70s, so
`399 RUB` is roughly a small single-digit USD price and should feel accessible
for a developer utility in the Russian market.

Trial behavior:

- trial starts when the account is created
- trial account can create projects and install the engine
- trial account gets normal cloud upload and dashboard behavior
- when trial expires, cloud features pause like inactive subscription
- local hooks and local command automation continue working
- local events keep being recorded locally within retention/cap rules
- subscribing resumes cloud upload and dashboard updates

Abuse limits should protect the service, not feel like product restrictions.
They should return clear `429 Too Many Requests` responses with `Retry-After`.
The local engine must treat rate limits like temporary cloud backpressure:

- keep recording hook events locally
- keep command automation local
- keep queued events durable
- back off uploads
- retry later
- show the issue in `vibeengine status`
- never tell the user their local stats are lost just because the cloud rejected
  a burst

Initial protective limits:

- check-in: generous token bucket per engine credential, for example 60 requests
  per minute with burst tolerance
- account check-ins: generous account-level cap to catch runaway installs, for
  example 600 requests per minute
- event batch size: max 500 events per check-in
- request body size: max 1 MB per check-in
- project creation: max 50 projects per day per account
- bootstrap token creation/exchange: max 30 per hour per account
- auth/email-code attempts: strict per email/IP rate limits
- export generation: max 3 per day per account

These are not pricing limits. They are service-protection defaults. If a real
user hits them through normal usage, the limits are wrong and should be raised.

Example rate-limited response:

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 60
Content-Type: application/json

{
  "code": "rate_limited",
  "message": "Too many requests. Local capture continues; cloud upload will retry."
}
```

Example local status:

```text
$ vibeengine status
Cloud: waiting
Reason: server asked us to slow down
Retry: 48s
Local capture: ok
Queued events: 24

Local work is still being recorded.
```

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

### Command Automation State Machine

Command automation must be a strict local state machine. The browser, hooks, and
cloud check-in must never start or stop processes directly. They only create
requests. One local supervisor per project serializes those requests.

Core invariant:

```text
one project = one command supervisor = one writer for command runtime state
```

Runtime states:

- `disabled`: command automation off; no command should run
- `stopped`: automation on, but command intentionally not running
- `starting`: process launch in progress
- `running`: process is alive but not ready yet
- `ready`: process is alive and ready/healthy
- `pausing`: stop requested, graceful shutdown in progress
- `paused`: intentionally stopped and resumable
- `restarting`: controlled stop then start in progress
- `failed`: command exited unexpectedly or health failed beyond policy

Prompt state is separate:

```text
prompt_state = idle | prompting | stale
```

A project can be prompting while command automation is disabled. Do not hide
prompt activity because command automation is off.

Prompt state should be backed by an active prompt id set, not just a boolean.
This prevents duplicate or overlapping hook events from resuming the command too
early.

Prompt identity:

- if the tool/hook provides a stable prompt id, use it
- otherwise the hook CLI creates a local `prompt_instance_id` on
  `prompt_started`
- `prompt_finished` is matched to the active prompt in the same project/session
- if there are multiple active prompts in the same session, finish closes the
  oldest unfinished prompt unless the hook provides a better id
- every hook event gets a monotonic `local_sequence` from SQLite
- supervisor processes prompt events by `local_sequence`, not by wall-clock time
- prompt ids are local reliability ids; they are not user-facing
- duplicate hook delivery uses the same idempotency key and does not create a
  second active prompt

Supervisor rules:

- one supervisor loop owns state transitions for a project
- one engine process owns supervisors at a time using a local process/file lock
- requests are persisted before execution
- actions are idempotent by action id
- state transitions are written transactionally with `state_version`
- state writes use compare-and-swap:
  `where project_id = ? and state_version = ?`
- each process start gets a new `run_id`
- process exit callbacks, health results, and timers must include `run_id`
- stale callbacks for old `run_id` values are ignored
- repeated restart requests inside a cooldown are collapsed
- incompatible queued requests are superseded by the newest intentional request
- browser tab close/reopen cannot repeat actions
- check-in redelivery cannot repeat an already applied action
- process state comes from the supervised child process, not optimistic UI state
- hook CLI writes hook events only; it does not mutate command runtime state
- SQLite writes use transactions so hooks, check-in, status, and supervisor
  cannot partially update shared state
- safe command actions carry the target approved command hash; if the local
  approved hash no longer matches, the action is rejected as stale
- never hold a SQLite write transaction while waiting on a process, network
  request, timer, or health check

Compare-and-swap transition pattern:

```text
1. read command_state and state_version
2. decide next transition
3. write intent/state with WHERE state_version = old_version
4. if zero rows changed, reload state and recompute
5. perform external process work outside the DB transaction
6. write result only if run_id/state_version still match
```

Allowed runtime transitions:

```text
disabled  -> stopped
stopped   -> starting | disabled
starting  -> running | pausing | failed | disabled
running   -> ready | pausing | restarting | failed | disabled
ready     -> pausing | restarting | failed | disabled
pausing   -> paused | stopped | failed | disabled
paused    -> starting | stopped | disabled
restarting -> starting | running | failed | disabled
failed    -> starting | stopped | disabled
```

Any transition outside this table should be treated as a bug and rejected with a
logged diagnostic.

Race handling:

- prompt start and prompt finish are idempotent
- duplicate prompt start for the same prompt id does not repeat pause
- overlapping prompt starts keep `prompt_state=prompting` until all active prompt
  ids finish or become stale
- prompt finish without known active prompt id records the event but does not
  resume command
- prompt finish for an already finished/stale prompt is a no-op for command
  state
- command approval while a command is running updates approved config first; any
  restart is a single supervised restart, not a UI-triggered loop
- disabling automation cancels pending start/restart/resume timers and moves to
  `disabled`
- project deletion cancels pending command actions and stops automation
- offline/cloud retry must not replay already applied actions
- local user actions should get a new action id and source `local`, so they can
  supersede older cloud actions deterministically
- actions in `processing` with expired `lease_until` are recovered on engine
  restart by inspecting actual command state before retrying

Crash/restart recovery:

- on engine start, acquire the single-engine lock before starting supervisors
- reload persisted project command state from SQLite
- verify whether recorded child process/process group still exists
- if the recorded child is gone, move running/ready/restarting states to
  `failed` or `stopped` with a clear reason
- if the recorded child exists and is known to be ours, reattach supervision
  where the OS allows it
- do not kill unknown external processes during recovery
- recreate timers from persisted state only when their `run_id` and
  `state_version` still match
- prompt state becomes `stale` if prompt activity is older than the freshness
  window
- after crash recovery, report a clear status instead of silently restarting in a
  loop
- if hook events were recorded while the engine was down, process them in local
  event order and apply pause/resume only when the prompt event is still fresh
- a new `session_started` event marks unfinished prompts from older sessions as
  stale for command-resume purposes

Freshness rules:

- dashboard `Prompting` requires a fresh engine check-in
- local pause/resume uses prompt event order and active prompt ids, not dashboard
  freshness
- unfinished prompts from an old session become `stale`, not active forever
- stale prompts can explain history but cannot trigger resume
- default maximum prompt-active window for local recovery should be generous,
  for example 12 hours, so long prompts are not broken but abandoned prompt ids
  do not block automation forever

Health checks are part of command automation:

- no command automation means no command health check
- command not running means no command health check
- shell health checks require local approval
- URL health checks start only after supervised process launch and settle delay
- before launch, URL health should be sampled once if a URL health check is
  configured
- health success moves `running -> ready`
- startup health failure keeps `running` until startup timeout
- health failure after `ready` moves to `failed` or triggers restart only if
  restart policy allows it
- health results are ignored when their `run_id` is stale
- if URL was already healthy before launch, do not blindly claim this command
  made it ready; show attention or an external-ready detail instead
- if pre-launch URL health was already passing, V1 should not auto-restart or
  auto-pause based on that URL as if it belonged to the supervised command
- if the command daemonizes and exits while leaving an external service alive,
  treat the supervised command as exited; V1 does not support fire-and-forget
  daemon commands as healthy supervised commands

Pause during prompt:

```text
on prompt_started(prompt_id):
  tx:
    active_prompts.insert_or_ignore(prompt_id)
    prompt_state = prompting
    if pause_during_prompt && state in starting/running/ready && paused_by is empty:
      mark active prompt pause_requested = true
      enqueue pause(paused_by = "prompt")

on prompt_finished(prompt_id, result):
  tx:
    active_prompts.mark_finished(prompt_id)
    if no active prompts remain:
      prompt_state = idle
      if paused_by == "prompt":
        enqueue resume()
```

Rules:

- resume only if this supervisor paused the command for prompt mode
- resume only after all active prompt ids are finished or stale
- do not resume after manual pause
- do not resume after manual resume
- do not resume after automation was disabled
- do not resume after project deletion
- do not resume from stale/duplicate prompt finish events
- if the user manually pauses while `paused_by=prompt`, change `paused_by` to
  `user` so prompt finish cannot resume it
- if the user manually resumes while prompts are still active, clear
  `paused_by=prompt` so prompt finish does not resume again

Auto-pause after ready:

- when state enters `ready`, store `ready_since` and start a 5-minute timer
- timer includes current `run_id` and `state_version`
- if still ready when timer fires, pause command
- timer is canceled when state leaves `ready`
- restart/resume creates a new `run_id` or state version and resets the timer
- if health never reaches ready, timer does not start
- if no health check is configured, use startup settle success as `ready`
- in-memory timers use monotonic time
- persisted timer deadlines use UTC and are revalidated against `run_id` and
  `state_version` after engine restart

Restart behavior:

- restart requires automation enabled and approved command hash
- restart creates one new supervised run
- repeated restart requests within cooldown collapse into one restart
- restart loops require backoff and max-attempt policy
- failed restart leaves old failure visible instead of spinning forever

Success sound:

- prompt success sound is based on `prompt_finished(result=success)`
- do not play success sound for command ready
- do not play success sound for health passing
- do not play success sound for config applied
- do not play success sound for resume after prompt
- do not replay old queued success events as new browser sounds

Process cleanup:

- supervisor tracks PID/process group
- on Windows, use Job Object or equivalent child-process tracking where needed
- stop sends graceful termination first
- after timeout, kill the process group
- if the child process exits, state updates immediately
- stale unknown processes are not killed unless they are known children
- health checks must not keep a command alive after the child process exited

State-machine invariants:

- `disabled` means no supervised child process should be running
- `ready` requires current `run_id`, live supervised child, and successful
  readiness rule
- `paused` means no supervised child process should be running
- `paused_by=prompt` is the only state that prompt finish may resume
- command health checks require automation enabled and a live supervised child
- URL health success cannot resurrect a process that already exited
- stale action ids, stale `run_id`, stale `state_version`, and stale target
  command hashes cannot change runtime state
- only account/project deletion and automation disable can cancel all pending
  command work immediately

Conflict resolution:

- disable automation beats start/restart/resume
- project deletion beats every command action
- manual pause beats pause-during-prompt resume
- restart beats start, but does not beat disable/delete
- latest explicit user action beats older cloud action when both are pending
- health failure cannot restart forever; restart policy requires backoff and
  maximum attempts

Why this model works:

- At-most-one supervisor: the engine lock prevents two engine processes from
  running competing supervisors.
- At-most-one runtime writer: only the project supervisor writes command runtime
  state.
- No lost state update: `state_version` compare-and-swap forces stale writers to
  reload before writing.
- No stale callback mutation: `run_id` on process exits, health checks, and
  timers prevents old async work from changing the current run.
- No action replay: action id and idempotency key make cloud redelivery and
  browser retries harmless.
- No stale command action: `target_command_hash` prevents a restart/pause action
  created for an old approved command from applying to a newly approved command.
- No early prompt resume: active prompt ids require every active prompt to finish
  or become stale before prompt-created pause can resume.
- No hidden prompt state: prompt state is separate from command automation state.
- No false ready from old service: pre-launch URL health is sampled and cannot be
  treated as proof that the supervised command became ready.
- No infinite restart loop: restart attempts require cooldown, backoff, and max
  attempts.
- No DB lock stalls: external process/network/timer work happens outside SQLite
  write transactions.
- No unknown process damage: recovery and cleanup only affect known child
  processes/process groups.

Implementation acceptance criteria:

- every command action has an idempotency key
- every process start creates a new `run_id`
- every async callback carries `run_id`
- every state write increments `state_version`
- every state write that depends on previous state uses compare-and-swap
- every timer validates `run_id` and `state_version` before acting
- every health result validates `run_id` before acting
- every cloud command action includes `target_command_hash`
- hook CLI inserts durable events and exits; it never starts/stops commands
- supervisor can be killed and restarted without creating duplicate child
  processes or repeating terminal actions

Required tests before shipping:

- duplicate prompt start does not double pause
- duplicate prompt finish does not double resume
- overlapping prompts do not resume until the last prompt finishes
- finish without matching active prompt does not resume
- new session marks old unfinished prompt stale for resume purposes
- manual pause during prompt prevents auto-resume on prompt finish
- manual resume during prompt prevents second resume on prompt finish
- disable automation while starting kills/cancels and ignores stale start result
- compare-and-swap conflict reloads state and does not lose an action
- restart spam collapses into one supervised restart
- stale health success from old `run_id` is ignored
- URL already healthy before launch does not produce false command-owned ready
- pre-launch URL healthy does not trigger auto-pause/restart as if command owned
  it
- child process exit after health success moves out of `ready`
- engine crash during restart recovers without restart loop
- processing action with expired lease is recovered from actual process state
- check-in redelivery does not reapply an already applied action
- cloud restart for old command hash is rejected after command approval changes
- auto-pause timer does not fire after restart/resume changed `run_id`
- no SQLite write transaction is held while waiting for process exit/health/timer
- deleted project stops automation but keeps queued historical events
- command disabled does not hide `Prompting`

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

V1 should support the main developer platforms:

- Linux x64
- Linux arm64
- macOS arm64
- macOS x64
- Windows x64
- Windows arm64 if the release pipeline can produce and test it reliably

Go makes cross-platform engine builds realistic, so Windows should not be
excluded just because it has different shell/service/PATH integration. Treat
Windows support as a release-engineering task, not a reason to avoid the
platform.

Allowed automatic update model:

- update artifacts are signed by an offline release key
- the installed engine contains the trusted public key
- the engine verifies the signature before installing
- failed signature means no install
- update channel is explicit, for example stable or beta
- status shows current version and last update result
- failed update keeps the old binary running
- update install is atomic: download, verify, swap, restart
- keep one previous binary for rollback
- do not offer auto-update disable in V1
- allow stable/beta channel selection

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

`vibeengine status` should show update health:

```text
Version: 1.4.2
Channel: stable
Last update check: 2h ago
Last update result: ok
Rollback available: 1.4.1
```

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

Windows should have an equivalent PowerShell install command in the UI:

```powershell
iwr https://vibe.example/install.ps1 -UseB | iex
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

- detect OS and CPU architecture
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

Install location should be user-local, not repo-local. Exact paths are
implementation details, but the policy is:

- binary lives in an OS app-data/user-local bin location
- local state lives in OS app-data
- repo contains only generated Codex hook configuration
- add a PATH shim/symlink only if needed
- do not require administrator/root privileges for normal install
- on Windows, use the normal user-local app data and PATH model

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

### Prototype Boundary

The existing local Vibecode Helper should be treated as a prototype and reference
only. The platform/engine should not depend on it or preserve compatibility with
its internal state.

Permanent policy:

- do not build an old-helper migration layer
- do not import old local stats
- do not read old local databases
- do not preserve old runner state
- do not support old hook formats forever
- do not shape the new architecture around prototype implementation details
- use the old helper only for lessons learned and UX reference

The only required old-helper behavior is safe cleanup:

- detect old generated Vibecode Helper hooks if present
- remove or replace only generated hook blocks owned by the old helper
- preserve unrelated/manual user hooks
- avoid duplicate events
- print a clear cleanup line such as `Old prototype hooks removed`

This is cleanup, not migration. The new engine starts from the new account,
project, bootstrap token, and hook model.

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
