# Vibeengine Implementation Checklist

This checklist turns `docs/PLATFORM.md` into an implementation tracker. Keep it
boring: do the phases in order, do not add teams/public APIs/advanced analytics
before the core loop is reliable, and do not mark a phase complete until its
acceptance gate passes.

This is intentionally shorter than `docs/PLATFORM.md`. The platform document is
the detailed spec and rationale. This file is the execution checklist. When a
task needs exact behavior, use the linked platform section as the source of
truth.

## Coverage Map

| Platform section | Checklist coverage |
| --- | --- |
| Product thesis, architecture split, implementation stack | Phases 0-3 |
| Hook event flow and tool adapter strategy | Phases 7-9 |
| Local durability and local SQLite | Phases 6-7, 10, 13, 20 |
| Project identity, local storage, and project lifecycle | Phases 5-6, 9-10 |
| Account deletion, export, privacy, upload policy | Phases 8, 10, 14, 19 |
| Retention policy | Phases 10, 13, 19-20 |
| Legal and product pages | Phases 2, 14, 19 |
| Hosted dashboard, stats, status vocabulary, sound | Phases 5, 11-12 |
| Background engine, cloud check-in, API contract | Phases 3A, 10, 13 |
| Security hardening, email, monitoring | Phases 4, 20-21 |
| Pricing, trial, subscription gating, YooKassa | Phase 15 |
| Remote control, command settings, local approval UX | Phases 16-17 |
| Engine updates and release process | Phase 18 |
| CLI UX, status diagnostics, install UX | Phases 9, 13, 21 |
| Credentials, revocation, reconnect | Phases 10, 13, 15, 21 |
| Failure modes, polish bar, risk register | Phases 20-21 |
| MVP scope cut, private beta, paid beta, later | Phases 14-22 |

## Phase 0 - Decision Lock

- [x] Confirm product name, repository name, and service folder names.
- [x] Confirm V1 scope: personal accounts, projects, metadata-only stats, local
  engine, hosted dashboard.
- [x] Confirm V1 excludes teams, seats, public API, prompt content upload,
  command logs, repo/file metadata, and device management UI.
- [x] Confirm first adapters: Codex and Claude Code.
- [x] Confirm next adapter backlog: Cursor, GitHub Copilot, Windsurf,
  Antigravity, Qwen Code, Cline, OpenCode.
- [x] Confirm private beta first, paid beta later.
- [x] Confirm `docs/PLATFORM.md` is the source of truth for product decisions.

Acceptance gate:

- [x] No unresolved product decision blocks implementation.

## Phase 1 - Clean Repository Scaffold

- [x] Snapshot current old helper runtime before making scaffold changes:
  - old Compose project `vibecode-dev-notify`
  - old service `helper`
  - old port `8766`
  - old volumes `helper-data` and `helper-cache`
- [x] Do not run `docker compose down` for the old helper project.
- [x] Do not stop or remove old helper containers, images, or volumes.
- [x] Do not delete or rewrite old helper databases.
- [x] Keep the old helper running while building the new platform.
- [x] Create final clean root structure:
  - `.gitignore`
  - `.env.example`
  - `README.md`
  - `docker-compose.yml`
  - `docs/`
  - hosted web/API service folder
  - local engine/CLI service folder
- [x] Use new Docker Compose project name `vibeengine-platform`.
- [x] Use new service names: `web`, `postgres`, and optional `proxy`.
- [x] Use development web port `8788`, not old helper port `8766`.
- [x] Use Postgres host port `55432` only if a host port is needed.
- [x] Do not reuse old helper volume names.
- [x] Move or remove old prototype noise only when it is no longer needed.
- [x] Keep framework-generated files inside the relevant service folder.
- [x] Add root README with local development commands.
- [x] Add root `.env.example` with only required local variables.
- [x] Add Docker Compose for Postgres, hosted app, and proxy only if needed.
- [x] Add basic formatting/linting/test commands per service.

Acceptance gate:

- [x] A new developer can understand the repository layout in a few seconds.
- [x] `docker compose up` starts the development dependencies.
- [x] Current old helper remains running and reachable on port `8766`.

## Phase 2 - Hosted App Foundation

- [x] Create Next.js App Router app with TypeScript.
- [x] Add Tailwind CSS.
- [x] Add lucide-react.
- [x] Add base layout with black background and island-style surfaces.
- [x] Add public pages:
  - `/`
  - `/pricing`
  - `/login`
  - `/terms`
  - `/privacy`
  - `/refund`
  - `/contact`
- [x] Write legal/product page copy that matches the actual product:
  metadata-only upload, no prompt content, no command logs, trial terms,
  refund/contact path, and support email.
- [x] Add authenticated `/projects` route.
- [x] Add route protection for authenticated pages.
- [x] Add server-side environment validation.
- [x] Add app health endpoint.

Acceptance gate:

- [x] App starts locally.
- [x] Public pages render.
- [x] `/projects` redirects unauthenticated users to login.

## Phase 3 - Hosted Database Foundation

- [x] Add Drizzle ORM.
- [x] Add Postgres connection.
- [x] Add migrations for:
  - `users`
  - `accounts`
  - `sessions`
  - `email_login_codes`
  - `invite_codes`
  - `projects`
  - `project_events`
  - `daily_project_stats`
  - `engine_credentials`
  - `bootstrap_tokens`
  - `remote_actions`
  - `command_proposals`
  - `subscriptions`
  - `webhook_events`
  - `billing_events`
  - `audit_events`
- [x] Add unique constraints and idempotency constraints.
- [x] Add UTC timestamp policy.
- [x] Add seed script for private beta invite codes.
- [x] Add migration test or smoke command.

Acceptance gate:

- [x] Migrations apply from empty database.
- [x] Migrations are repeatable in local dev.

## Phase 3A - Hosted API Skeleton

- [x] Add browser auth routes:
  - `POST /api/auth/request-code`
  - `POST /api/auth/verify-code`
  - `POST /api/auth/logout`
  - `GET /api/me`
- [x] Add browser project routes:
  - `GET /api/projects`
  - `POST /api/projects`
  - `GET /api/projects/:projectId`
  - `PATCH /api/projects/:projectId`
  - `DELETE /api/projects/:projectId`
- [x] Add install/bootstrap browser route:
  - `POST /api/projects/:projectId/bootstrap-token`
- [x] Add remote-control browser routes:
  - `POST /api/projects/:projectId/command-proposals`
  - `POST /api/projects/:projectId/actions`
- [x] Add subscription browser routes:
  - `GET /api/subscription`
  - `POST /api/subscription/subscribe`
  - `POST /api/subscription/cancel`
- [x] Add account export routes:
  - `POST /api/account/export`
  - `GET /api/account/export/:exportId`
- [x] Add account deletion routes:
  - `POST /api/account/delete/request-code`
  - `POST /api/account/delete/confirm`
- [x] Add YooKassa webhook route:
  - `POST /api/yookassa/webhook`
- [x] Add engine routes:
  - `POST /api/engine/bootstrap`
  - `POST /api/engine/token`
  - `POST /api/engine/check-in`
- [x] Enforce signed session cookies only on browser routes.
- [x] Enforce bearer access tokens only on engine routes.
- [x] Add CSRF protection or strict same-site cookie strategy for mutating
  browser routes.
- [x] Add common JSON error shape.
- [x] Add request size limits.
- [x] Add route-level rate-limit hooks.

Acceptance gate:

- [x] API skeleton matches the V1 API shape in `docs/PLATFORM.md`.
- [x] Browser auth cannot access engine routes and engine tokens cannot access
  browser routes.

## Phase 4 - Auth And Private Beta Access

- [x] Implement email-code login.
- [x] Implement signed session cookie.
- [x] Implement invite-code requirement for new account creation.
- [x] Let existing users log in with email code only.
- [x] Hash email login codes.
- [x] Rate-limit login code requests and attempts.
- [x] Add account creation with personal account.
- [x] Add logout.
- [x] Add account popover shell in top right.
- [x] Add basic auth audit events.
- [x] Add email provider adapter.
- [x] Configure RuSender as primary email provider.
- [x] Keep Unisender Go as fallback option in config/docs.

Acceptance gate:

- [x] New user can sign up with email plus invite code.
- [x] Existing user can log in with email code.
- [x] Session survives refresh and expires correctly.

## Phase 5 - Projects UI And CRUD

- [x] Build `/projects` dashboard shell.
- [x] Add global stats area.
- [x] Add search bar.
- [x] Add `+ Project` button.
- [x] Add create-project modal.
- [x] Enforce project name uniqueness inside account.
- [x] Add compact project cards with project name and humane status.
- [x] Add right-side project management panel.
- [x] Add delete project action.
- [x] Make delete free the project name without deleting history.
- [x] Store deleted projects as `state=deleted`, not hard-deleted rows.
- [x] Ensure old offline queued events for a deleted project backfill into the
  old project history by immutable `project_id`.
- [x] Ensure a new project with the same name receives a new `project_id`.
- [x] Stop remote actions/proposals for deleted projects.
- [x] Stop local command automation for deleted projects after the engine learns
  deletion state.
- [x] Keep global stats only; do not add per-project stats panels.
- [x] Add account popover modal with blur background.
- [x] Add sound status control near dashboard controls.

Acceptance gate:

- [x] User can create, search, open, and delete projects.
- [x] Deleted project history remains in DB and name can be reused.

## Phase 6 - Local Engine Foundation

- [x] Create Go local engine/CLI service.
- [x] Add `vibeengine` CLI entrypoint.
- [x] Add cross-platform build targets for macOS, Linux, and Windows.
- [x] Add local app-data path resolver.
- [x] Add local lock so only one background engine owns supervision.
- [x] Add local SQLite database.
- [x] Enable SQLite WAL mode.
- [x] Add migrations for:
  - `local_kv`
  - `projects`
  - `events`
  - `pending_actions`
  - `active_prompts`
  - `pending_approvals`
  - `approved_commands`
  - `command_state`
  - `update_state`
- [x] Store auth separately from ordinary event metadata.
- [x] Enforce user-only permissions where supported.
- [x] Add redacted logging.
- [x] Add `vibeengine --version`.

Acceptance gate:

- [x] CLI starts on supported platforms.
- [x] Local SQLite initializes idempotently.
- [x] Local auth/state paths are not inside the user repo.

## Phase 7 - Local Hook Event Capture

- [x] Add `vibeengine hook session-start`.
- [x] Add `vibeengine hook session-end`.
- [x] Add `vibeengine hook prompt-started`.
- [x] Add `vibeengine hook prompt-finished`.
- [x] Add `vibeengine hook prompt-failed`.
- [x] Add `vibeengine hook permission-asked`.
- [x] Add `vibeengine hook tool-started`.
- [x] Add `vibeengine hook tool-finished`.
- [x] Add `--project`.
- [x] Add `--tool`.
- [x] Add native event metadata fields.
- [x] Generate local event ids.
- [x] Generate local monotonic sequence.
- [x] Create prompt/session ids when native ids are missing.
- [x] Persist every hook event before exiting.
- [x] Never call cloud API from hook command.
- [x] Keep hook command fast with timeout-safe behavior.
- [x] Add duplicate event/idempotency tests.

Acceptance gate:

- [x] Hook command records events offline.
- [x] Duplicate hook delivery does not create duplicate logical prompts.

## Phase 8 - Tool Adapter Installers

- [x] Implement generated hook ownership metadata.
- [x] Implement safe repair that replaces only owned generated blocks.
- [x] Preserve manual hooks.
- [x] Use a separate test repo/project for new hook adapter development.
- [x] Do not install new generated hooks into a repo actively used by the old
  helper unless the user explicitly asks to test replacement behavior.
- [x] Implement Codex adapter mapping.
- [x] Implement Claude Code adapter mapping.
- [x] Map native events to canonical local events.
- [x] Drop/redact content-like native payload fields.
- [x] Store `tool_adapter`, `native_event_name`, and safe native ids.
- [x] Support installing multiple adapters in one repo.
- [x] Add adapter tests with sample native payloads.

Acceptance gate:

- [x] Codex prompts produce correct local events.
- [x] Claude Code prompts produce correct local events.
- [x] Prompt content, response content, transcript paths, command output, file
  paths, and repo metadata are not uploaded in normal events.

## Phase 9 - Bootstrap Install And Repair

- [x] Add hosted bootstrap token creation.
- [x] Generate fresh short-lived one-time token every time install command is
  viewed or copied.
- [x] Add install command in project side panel.
- [x] Add macOS/Linux install script.
- [x] Add Windows PowerShell install script.
- [x] Detect OS and CPU architecture.
- [x] Download latest stable engine.
- [x] Install engine user-locally.
- [x] Add PATH shim/symlink only if needed.
- [x] Exchange bootstrap token for engine/project credentials.
- [x] Reuse local auth when it belongs to the same account.
- [x] Stop clearly if local machine is logged into another account.
- [x] Detect current repo.
- [x] Register or update local project mapping.
- [x] Install or repair selected adapter hooks.
- [x] Start or repair background engine.
- [x] Run status checks at end of install.
- [x] Print short success summary.

Acceptance gate:

- [x] User can copy one command from web UI and get a working local install.
- [x] Re-running the command repairs installation without deleting local data.

## Phase 10 - Background Engine And Check-In

- [x] Implement background engine process.
- [x] Start engine from `session-start` or equivalent adapter event when missing.
- [x] Start or repair engine from `vibeengine status` when safe.
- [x] Implement engine token refresh.
- [x] Implement `POST /api/engine/check-in`.
- [x] Upload queued local events in batches.
- [x] Enforce max 500 events per check-in.
- [x] Enforce max 1 MB check-in request body.
- [x] Acknowledge accepted events.
- [x] Mark local events delivered only after server acknowledgement.
- [x] Retry failed uploads with backoff.
- [x] Avoid aggressive retries when signed out/revoked.
- [x] Include project status summaries.
- [x] Include last applied remote action id.
- [x] Include entitlement state in check-in response.
- [x] Include update notices in check-in response.
- [x] Make uploads idempotent by account/project/local event id.
- [x] Reject or drop forbidden content fields in normal uploaded events.
- [x] Add server-side tests that prompt text, responses, transcript paths,
  command output, command text, file paths, repo metadata, env vars, and tokens
  cannot enter `project_events`.
- [x] Rebuild `daily_project_stats` from source events.
- [x] Keep hosted source events as the source of truth for aggregates.
- [x] Preserve uploaded historical events for deleted projects.
- [x] Pause cloud upload when entitlement is inactive, while local capture
  continues.
- [x] Handle revoked credentials:
  - [x] reject old refresh tokens
  - [x] reject access tokens with stale auth epoch
  - [x] return `engine_signed_out`
  - [x] stop cloud upload locally after the engine sees revocation
  - [x] keep local capture under retention limits

Acceptance gate:

- [x] Offline events upload after connectivity returns.
- [x] Duplicate check-ins do not duplicate hosted events or stats.
- [x] Forbidden private data cannot be persisted by normal event upload.

## Phase 11 - Dashboard Live Status And Stats

- [x] Implement project statuses:
  - `Prompting`
  - `Ready`
  - `Running`
  - `Paused`
  - `Offline`
  - `Attention`
  - `Setup`
- [x] Keep prompt activity independent from command automation state.
- [x] Show command-off state only where command automation exists.
- [x] Show setup/offline correctly when no runner/engine is connected.
- [x] Implement global today stats.
- [x] Use UTC source events.
- [x] Use account/user timezone only for display day grouping.
- [x] Backfill stats when old offline events arrive.
- [x] Implement dashboard polling.
- [x] Avoid endless "applying" states.
- [x] Add stale prompt detection.

Acceptance gate:

- [x] Dashboard shows correct project status after prompt start/finish.
- [x] Stats remain correct after late offline uploads.

## Phase 12 - Browser Sound UX

- [x] Add browser sound enablement state.
- [x] Show `Enable sound` until browser allows sound.
- [x] Play success sound on prompt finished success.
- [x] Play attention sound on permission requested.
- [x] Play error sound on command automation or health failure later.
- [x] Do not play sounds for ordinary check-ins/uploads/retries/config saves.
- [x] Throttle repeated sounds by project/reason.
- [x] Do not require desktop notifications in V1.

Acceptance gate:

- [x] User hears success sound after a prompt finishes when sound is enabled.
- [x] Sounds do not spam during retries or repeated check-ins.

## Phase 13 - `vibeengine status`

- [x] Implement one status command.
- [x] Show local engine state.
- [x] Show local capture health.
- [x] Show queued event count.
- [x] Show oldest pending event age.
- [x] Show local DB size and configured cap.
- [x] Show retention health.
- [x] Show cloud reachability.
- [x] Show subscription/entitlement state when known.
- [x] Show project mappings.
- [x] Show installed adapters.
- [x] Show command automation availability.
- [x] Start or repair engine when safe.
- [x] Check API DNS resolution.
- [x] Check HTTPS connectivity.
- [x] Check TLS verification.
- [x] Explain proxy/network/container issues clearly.
- [x] Explain inactive subscription clearly: local capture continues, cloud
  upload/dashboard require subscription.
- [x] Explain revoked/signed-out engine credentials clearly and tell the user to
  rerun the install command from the web app.
- [x] Warn before local queue approaches the 30-day/100 MB retention cap.
- [x] Redact tokens, command text, env vars, auth headers, cookies, and DB paths.
- [x] Add golden tests for status output.

Acceptance gate:

- [x] `vibeengine status` explains what is wrong and what to do next.
- [x] No separate normal-use doctor/sync/upload/stats command is needed.

## Phase 14 - Free Private Beta Release

- [x] Landing page is public.
- [x] Pricing page explains private beta / trial timing.
- [x] Login works.
- [x] Invite codes work.
- [x] `/projects` works.
- [x] Project creation/deletion works.
- [x] Install command works.
- [x] Engine bootstrap works.
- [x] Codex adapter works.
- [x] Claude Code adapter works.
- [x] Local SQLite capture works.
- [x] Check-in upload works.
- [x] Dashboard statuses work.
- [x] Global today stats work.
- [x] Success sound works.
- [x] `vibeengine status` works.
- [x] Metadata-only upload policy is implemented.
- [x] Basic rate limits are enabled.
- [x] Basic logs/metrics are available.
- [x] Log redaction is verified.
- [x] Private beta runbook exists.

Acceptance gate:

- [ ] A real user can create a project, install the engine, run a supported AI
  tool, finish a prompt, hear sound, and see dashboard stats.

## Phase 15 - Billing And Entitlements

- [x] Add YooKassa subscription flow.
- [x] Add 14-day trial.
- [x] Add one personal plan at `399 RUB / month`.
- [x] Do not require card for trial.
- [x] Start trial when account is created.
- [x] Add subscription states:
  - `trialing`
  - `active`
  - `grace`
  - `past_due`
  - `inactive`
  - `canceled`
- [x] Add YooKassa webhook endpoint.
- [x] Validate webhook authenticity.
- [x] Make webhook handling idempotent.
- [x] Add renewal reconciliation job.
- [x] Add grace behavior around payment processing.
- [x] Gate cloud API by entitlement.
- [x] Pause cloud upload and hosted dashboard/history when trial/subscription is
  inactive.
- [x] Let local engine keep local capture under local retention when cloud access
  is unavailable.
- [x] Resume upload and dashboard updates after user subscribes/reactivates.
- [x] Add clear UI for inactive/past-due state.
- [x] Add subscription cancellation when account is deleted.
- [x] Add subscription state to `vibeengine status`.

Acceptance gate:

- [x] Subscription state cannot be bypassed for cloud service.
- [x] Payment edge cases do not incorrectly lock out active users during normal
  renewal processing.

## Phase 16 - Command Automation Local

- [x] Implement local supervisor state machine.
- [x] Ensure browser/UI never directly mutates process state.
- [x] Add approved command storage by command hash.
- [x] Add start command.
- [x] Add pause command.
- [x] Add resume command.
- [x] Add restart command.
- [x] Add pause during prompt.
- [x] Add resume/restart after prompt finish.
- [x] Add auto-pause after ready.
- [x] Add health command.
- [x] Make health a subpart of command automation.
- [x] Disable/mute dependent UI options when command automation is off without
  discarding saved choices.
- [x] Add health `run_id` handling so stale results are ignored.
- [x] Add pre-launch URL health behavior.
- [x] Prevent restart loops.
- [x] Prevent stale prompt finish from resuming old command.
- [x] Add table tests for state transitions.
- [x] Add race/recovery tests.

Acceptance gate:

- [x] Prompt start can pause command and prompt finish can resume/restart it.
- [x] Auto-pause after ready works after any restart/reactivation.
- [x] No duplicate or stale event causes overlapping process control.

## Phase 17 - Remote Actions And Local Approval

- [x] Add remote action model through check-in.
- [x] Add safe action delivery.
- [x] Add command proposals from web UI.
- [x] Store command proposal text only in proposal payload.
- [x] Do not store command text in normal events/logs/status/export stats.
- [x] Add `vibeengine sync`.
- [x] Show full proposed command diff locally.
- [x] Require local confirmation for risky changes.
- [x] Protect against accidental confirmation.
- [x] Report approval/rejection/apply result to hosted API.
- [x] Add pending proposal state in dashboard.
- [x] Add tests for approval/rejection/idempotency.

Acceptance gate:

- [x] Web can propose command changes, but local machine only applies them after
  explicit local approval.

## Phase 18 - Engine Updates And Release Signing

- [x] Generate signing key outside the repository.
- [x] Add dev signing key path to `.gitignore`.
- [x] Add signed release artifact format.
- [x] Verify signatures before install/update.
- [x] Add auto-update check.
- [x] Add safe binary swap.
- [x] Add rollback path.
- [x] Add update status in `vibeengine status`.
- [x] Add release process checklist.
- [x] Add old prototype hook cleanup.
- [x] Preserve manual hooks during cleanup.
- [x] Add update signature/rollback tests.

Acceptance gate:

- [x] Engine updates cannot install unsigned or wrongly signed artifacts.
- [x] Failed update rolls back without losing local data.

## Phase 19 - Export, Account Deletion, And Privacy

- [x] Add safe export ZIP job.
- [x] Export JSON metadata.
- [x] Export CSV prompt event summaries.
- [x] Use one export format only: ZIP with JSON metadata and CSV event
  summaries.
- [x] Make export links short-lived.
- [x] Limit export generation to max 3 per day per account.
- [x] Exclude prompt text, responses, command text, env vars, tokens, file paths,
  repo metadata, logs, and raw DB dumps.
- [x] Add account deletion request.
- [x] Require email code confirmation for account deletion.
- [x] Require strong confirmation, such as typing email.
- [x] Cancel active subscription on account deletion.
- [x] Soft-delete account/project records as documented.
- [x] Keep only minimal billing/legal/fraud records required by payment, tax,
  accounting, or legal obligations after account deletion.
- [x] Keep hosted events/stats while the account exists.
- [x] Keep project history after project deletion.
- [x] Apply local retention:
  - undelivered local events up to 30 days or DB cap
  - delivered local events 7 days
  - default local DB cap 100 MB
  - compact delivered diagnostics first
  - drop undelivered events only when disk/cap safety requires it
- [x] Add privacy policy pages matching implementation.

Acceptance gate:

- [x] User can export safe data.
- [x] User can delete account with email-code confirmation.
- [x] Deleted account cannot continue using cloud service.

## Phase 20 - Monitoring, Abuse Limits, And Hardening

- [x] Add structured logs.
- [x] Add metrics for check-ins, accepted/rejected events, queue depth, auth
  attempts, email sends, YooKassa webhooks, export/delete jobs, and errors.
- [x] Add alert thresholds.
- [x] Add basic admin/ops SQL views or scripts.
- [x] Add request rate limits.
- [x] Add account/project abuse limits.
- [x] Add initial protective limits:
  - check-in token bucket around 60 requests/minute per engine credential
  - account check-ins around 600 requests/minute
  - event batch size max 500
  - check-in body max 1 MB
  - project creation max 50/day/account
  - bootstrap token create/exchange max 30/hour/account
  - strict auth/email-code limits per email/IP
  - export generation max 3/day/account
- [x] Add check-in event batch caps.
- [x] Add payload size caps.
- [x] Add auth brute-force protections.
- [x] Add webhook replay protection.
- [x] Add audit events.
- [x] Add security headers.
- [x] Add CSRF protection where needed.
- [x] Add secrets redaction tests.
- [x] Add backup/restore plan for Postgres.
- [x] Review risk register before private beta.
- [x] Add tests or runbook items for every high-severity risk.

Acceptance gate:

- [x] Real abuse produces "try again later" behavior instead of server failure.
- [x] Operators can detect and debug production issues without raw sensitive
  data.

## Phase 21 - Production Readiness

- [x] Prepare production Docker Compose.
- [ ] Configure domain and TLS.
- [ ] Configure production Postgres.
- [x] Configure backups.
- [ ] Configure email provider.
- [ ] Configure YooKassa production credentials.
- [x] Configure signing key storage.
- [x] Configure server environment variables.
- [x] Run clean install on Linux.
- [ ] Run clean install on macOS.
- [ ] Run clean install on Windows.
- [x] Test Codex adapter on a real repo.
- [x] Test Claude Code adapter on a real repo.
- [x] Test dev container/network-restricted status diagnostics.
- [x] Test hosted API outage behavior.
- [x] Test local engine down behavior.
- [x] Test disk-full/local DB failure behavior.
- [x] Test account revocation/reinstall flow.
- [x] Test `Disconnect my installs` behavior.
- [x] Test access-token auth epoch invalidation.
- [x] Test subscription inactive flow.
- [x] Test account deletion with active subscription.
- [x] Test project deletion with late offline event upload.
- [x] Test local retention cap behavior.
- [x] Run privacy checklist.
- [x] Run security checklist.
- [x] Run billing checklist.
- [x] Run legal page checklist.
- [x] Write launch runbook.

Acceptance gate:

- [ ] Product can be installed and used by a paying user without manual
  developer intervention.

## Phase 22 - Deferred Until After V1

- [ ] Cursor adapter.
- [ ] GitHub Copilot / VS Code Copilot adapter.
- [ ] Windsurf adapter.
- [ ] Antigravity adapter.
- [ ] Qwen Code adapter.
- [ ] Cline adapter.
- [ ] OpenCode adapter.
- [ ] Beta update channel.
- [ ] Staged rollout UI.
- [ ] Account-wide revoke UI.
- [ ] Teams/workspaces/seats.
- [ ] Public API.
- [ ] Advanced analytics.
- [ ] Desktop notifications.
- [ ] Visible device management.
- [ ] Outgoing webhooks.
