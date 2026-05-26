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

- [ ] Confirm product name, repository name, and service folder names.
- [ ] Confirm V1 scope: personal accounts, projects, metadata-only stats, local
  engine, hosted dashboard.
- [ ] Confirm V1 excludes teams, seats, public API, prompt content upload,
  command logs, repo/file metadata, and device management UI.
- [ ] Confirm first adapters: Codex and Claude Code.
- [ ] Confirm next adapter backlog: Cursor, GitHub Copilot, Windsurf,
  Antigravity, Qwen Code, Cline, OpenCode.
- [ ] Confirm private beta first, paid beta later.
- [ ] Confirm `docs/PLATFORM.md` is the source of truth for product decisions.

Acceptance gate:

- [ ] No unresolved product decision blocks implementation.

## Phase 1 - Clean Repository Scaffold

- [ ] Create final clean root structure:
  - `.gitignore`
  - `.env.example`
  - `README.md`
  - `docker-compose.yml`
  - `docs/`
  - hosted web/API service folder
  - local engine/CLI service folder
- [ ] Move or remove old prototype noise only when it is no longer needed.
- [ ] Keep framework-generated files inside the relevant service folder.
- [ ] Add root README with local development commands.
- [ ] Add root `.env.example` with only required local variables.
- [ ] Add Docker Compose for Postgres, hosted app, and proxy only if needed.
- [ ] Add basic formatting/linting/test commands per service.

Acceptance gate:

- [ ] A new developer can understand the repository layout in a few seconds.
- [ ] `docker compose up` starts the development dependencies.

## Phase 2 - Hosted App Foundation

- [ ] Create Next.js App Router app with TypeScript.
- [ ] Add Tailwind CSS.
- [ ] Add lucide-react.
- [ ] Add base layout with black background and island-style surfaces.
- [ ] Add public pages:
  - `/`
  - `/pricing`
  - `/login`
  - `/terms`
  - `/privacy`
  - `/refund`
  - `/contact`
- [ ] Write legal/product page copy that matches the actual product:
  metadata-only upload, no prompt content, no command logs, trial terms,
  refund/contact path, and support email.
- [ ] Add authenticated `/projects` route.
- [ ] Add route protection for authenticated pages.
- [ ] Add server-side environment validation.
- [ ] Add app health endpoint.

Acceptance gate:

- [ ] App starts locally.
- [ ] Public pages render.
- [ ] `/projects` redirects unauthenticated users to login.

## Phase 3 - Hosted Database Foundation

- [ ] Add Drizzle ORM.
- [ ] Add Postgres connection.
- [ ] Add migrations for:
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
- [ ] Add unique constraints and idempotency constraints.
- [ ] Add UTC timestamp policy.
- [ ] Add seed script for private beta invite codes.
- [ ] Add migration test or smoke command.

Acceptance gate:

- [ ] Migrations apply from empty database.
- [ ] Migrations are repeatable in local dev.

## Phase 3A - Hosted API Skeleton

- [ ] Add browser auth routes:
  - `POST /api/auth/request-code`
  - `POST /api/auth/verify-code`
  - `POST /api/auth/logout`
  - `GET /api/me`
- [ ] Add browser project routes:
  - `GET /api/projects`
  - `POST /api/projects`
  - `GET /api/projects/:projectId`
  - `PATCH /api/projects/:projectId`
  - `DELETE /api/projects/:projectId`
- [ ] Add install/bootstrap browser route:
  - `POST /api/projects/:projectId/bootstrap-token`
- [ ] Add remote-control browser routes:
  - `POST /api/projects/:projectId/command-proposals`
  - `POST /api/projects/:projectId/actions`
- [ ] Add subscription browser routes:
  - `GET /api/subscription`
  - `POST /api/subscription/subscribe`
  - `POST /api/subscription/cancel`
- [ ] Add account export routes:
  - `POST /api/account/export`
  - `GET /api/account/export/:exportId`
- [ ] Add account deletion routes:
  - `POST /api/account/delete/request-code`
  - `POST /api/account/delete/confirm`
- [ ] Add YooKassa webhook route:
  - `POST /api/yookassa/webhook`
- [ ] Add engine routes:
  - `POST /api/engine/bootstrap`
  - `POST /api/engine/token`
  - `POST /api/engine/check-in`
- [ ] Enforce signed session cookies only on browser routes.
- [ ] Enforce bearer access tokens only on engine routes.
- [ ] Add CSRF protection or strict same-site cookie strategy for mutating
  browser routes.
- [ ] Add common JSON error shape.
- [ ] Add request size limits.
- [ ] Add route-level rate-limit hooks.

Acceptance gate:

- [ ] API skeleton matches the V1 API shape in `docs/PLATFORM.md`.
- [ ] Browser auth cannot access engine routes and engine tokens cannot access
  browser routes.

## Phase 4 - Auth And Private Beta Access

- [ ] Implement email-code login.
- [ ] Implement signed session cookie.
- [ ] Implement invite-code requirement for new account creation.
- [ ] Let existing users log in with email code only.
- [ ] Hash email login codes.
- [ ] Rate-limit login code requests and attempts.
- [ ] Add account creation with personal account.
- [ ] Add logout.
- [ ] Add account popover shell in top right.
- [ ] Add basic auth audit events.
- [ ] Add email provider adapter.
- [ ] Configure RuSender as primary email provider.
- [ ] Keep Unisender Go as fallback option in config/docs.

Acceptance gate:

- [ ] New user can sign up with email plus invite code.
- [ ] Existing user can log in with email code.
- [ ] Session survives refresh and expires correctly.

## Phase 5 - Projects UI And CRUD

- [ ] Build `/projects` dashboard shell.
- [ ] Add global stats area.
- [ ] Add search bar.
- [ ] Add `+ Project` button.
- [ ] Add create-project modal.
- [ ] Enforce project name uniqueness inside account.
- [ ] Add compact project cards with project name and humane status.
- [ ] Add right-side project management panel.
- [ ] Add delete project action.
- [ ] Make delete free the project name without deleting history.
- [ ] Store deleted projects as `state=deleted`, not hard-deleted rows.
- [ ] Ensure old offline queued events for a deleted project backfill into the
  old project history by immutable `project_id`.
- [ ] Ensure a new project with the same name receives a new `project_id`.
- [ ] Stop remote actions/proposals for deleted projects.
- [ ] Stop local command automation for deleted projects after the engine learns
  deletion state.
- [ ] Keep global stats only; do not add per-project stats panels.
- [ ] Add account popover modal with blur background.
- [ ] Add sound status control near dashboard controls.

Acceptance gate:

- [ ] User can create, search, open, and delete projects.
- [ ] Deleted project history remains in DB and name can be reused.

## Phase 6 - Local Engine Foundation

- [ ] Create Go local engine/CLI service.
- [ ] Add `vibeengine` CLI entrypoint.
- [ ] Add cross-platform build targets for macOS, Linux, and Windows.
- [ ] Add local app-data path resolver.
- [ ] Add local lock so only one background engine owns supervision.
- [ ] Add local SQLite database.
- [ ] Enable SQLite WAL mode.
- [ ] Add migrations for:
  - `local_kv`
  - `projects`
  - `events`
  - `pending_actions`
  - `active_prompts`
  - `pending_approvals`
  - `approved_commands`
  - `command_state`
  - `update_state`
- [ ] Store auth separately from ordinary event metadata.
- [ ] Enforce user-only permissions where supported.
- [ ] Add redacted logging.
- [ ] Add `vibeengine --version`.

Acceptance gate:

- [ ] CLI starts on supported platforms.
- [ ] Local SQLite initializes idempotently.
- [ ] Local auth/state paths are not inside the user repo.

## Phase 7 - Local Hook Event Capture

- [ ] Add `vibeengine hook session-start`.
- [ ] Add `vibeengine hook session-end`.
- [ ] Add `vibeengine hook prompt-started`.
- [ ] Add `vibeengine hook prompt-finished`.
- [ ] Add `vibeengine hook prompt-failed`.
- [ ] Add `vibeengine hook permission-asked`.
- [ ] Add `vibeengine hook tool-started`.
- [ ] Add `vibeengine hook tool-finished`.
- [ ] Add `--project`.
- [ ] Add `--tool`.
- [ ] Add native event metadata fields.
- [ ] Generate local event ids.
- [ ] Generate local monotonic sequence.
- [ ] Create prompt/session ids when native ids are missing.
- [ ] Persist every hook event before exiting.
- [ ] Never call cloud API from hook command.
- [ ] Keep hook command fast with timeout-safe behavior.
- [ ] Add duplicate event/idempotency tests.

Acceptance gate:

- [ ] Hook command records events offline.
- [ ] Duplicate hook delivery does not create duplicate logical prompts.

## Phase 8 - Tool Adapter Installers

- [ ] Implement generated hook ownership metadata.
- [ ] Implement safe repair that replaces only owned generated blocks.
- [ ] Preserve manual hooks.
- [ ] Implement Codex adapter mapping.
- [ ] Implement Claude Code adapter mapping.
- [ ] Map native events to canonical local events.
- [ ] Drop/redact content-like native payload fields.
- [ ] Store `tool_adapter`, `native_event_name`, and safe native ids.
- [ ] Support installing multiple adapters in one repo.
- [ ] Add adapter tests with sample native payloads.

Acceptance gate:

- [ ] Codex prompts produce correct local events.
- [ ] Claude Code prompts produce correct local events.
- [ ] Prompt content, response content, transcript paths, command output, file
  paths, and repo metadata are not uploaded in normal events.

## Phase 9 - Bootstrap Install And Repair

- [ ] Add hosted bootstrap token creation.
- [ ] Generate fresh short-lived one-time token every time install command is
  viewed or copied.
- [ ] Add install command in project side panel.
- [ ] Add macOS/Linux install script.
- [ ] Add Windows PowerShell install script.
- [ ] Detect OS and CPU architecture.
- [ ] Download latest stable engine.
- [ ] Install engine user-locally.
- [ ] Add PATH shim/symlink only if needed.
- [ ] Exchange bootstrap token for engine/project credentials.
- [ ] Reuse local auth when it belongs to the same account.
- [ ] Stop clearly if local machine is logged into another account.
- [ ] Detect current repo.
- [ ] Register or update local project mapping.
- [ ] Install or repair selected adapter hooks.
- [ ] Start or repair background engine.
- [ ] Run status checks at end of install.
- [ ] Print short success summary.

Acceptance gate:

- [ ] User can copy one command from web UI and get a working local install.
- [ ] Re-running the command repairs installation without deleting local data.

## Phase 10 - Background Engine And Check-In

- [ ] Implement background engine process.
- [ ] Start engine from `session-start` or equivalent adapter event when missing.
- [ ] Start or repair engine from `vibeengine status` when safe.
- [ ] Implement engine token refresh.
- [ ] Implement `POST /api/engine/check-in`.
- [ ] Upload queued local events in batches.
- [ ] Enforce max 500 events per check-in.
- [ ] Enforce max 1 MB check-in request body.
- [ ] Acknowledge accepted events.
- [ ] Mark local events delivered only after server acknowledgement.
- [ ] Retry failed uploads with backoff.
- [ ] Avoid aggressive retries when signed out/revoked.
- [ ] Include project status summaries.
- [ ] Include last applied remote action id.
- [ ] Include entitlement state in check-in response.
- [ ] Include update notices in check-in response.
- [ ] Make uploads idempotent by account/project/local event id.
- [ ] Reject or drop forbidden content fields in normal uploaded events.
- [ ] Add server-side tests that prompt text, responses, transcript paths,
  command output, command text, file paths, repo metadata, env vars, and tokens
  cannot enter `project_events`.
- [ ] Rebuild `daily_project_stats` from source events.
- [ ] Keep hosted source events as the source of truth for aggregates.
- [ ] Preserve uploaded historical events for deleted projects.
- [ ] Pause cloud upload when entitlement is inactive, while local capture
  continues.
- [ ] Handle revoked credentials:
  - reject old refresh tokens
  - reject access tokens with stale auth epoch
  - return `engine_signed_out`
  - stop cloud upload locally after the engine sees revocation
  - keep local capture under retention limits

Acceptance gate:

- [ ] Offline events upload after connectivity returns.
- [ ] Duplicate check-ins do not duplicate hosted events or stats.
- [ ] Forbidden private data cannot be persisted by normal event upload.

## Phase 11 - Dashboard Live Status And Stats

- [ ] Implement project statuses:
  - `Prompting`
  - `Ready`
  - `Running`
  - `Paused`
  - `Offline`
  - `Attention`
  - `Setup`
- [ ] Keep prompt activity independent from command automation state.
- [ ] Show command-off state only where command automation exists.
- [ ] Show setup/offline correctly when no runner/engine is connected.
- [ ] Implement global today stats.
- [ ] Use UTC source events.
- [ ] Use account/user timezone only for display day grouping.
- [ ] Backfill stats when old offline events arrive.
- [ ] Implement dashboard polling.
- [ ] Avoid endless "applying" states.
- [ ] Add stale prompt detection.

Acceptance gate:

- [ ] Dashboard shows correct project status after prompt start/finish.
- [ ] Stats remain correct after late offline uploads.

## Phase 12 - Browser Sound UX

- [ ] Add browser sound enablement state.
- [ ] Show `Enable sound` until browser allows sound.
- [ ] Play success sound on prompt finished success.
- [ ] Play attention sound on permission requested.
- [ ] Play error sound on command automation or health failure later.
- [ ] Do not play sounds for ordinary check-ins/uploads/retries/config saves.
- [ ] Throttle repeated sounds by project/reason.
- [ ] Do not require desktop notifications in V1.

Acceptance gate:

- [ ] User hears success sound after a prompt finishes when sound is enabled.
- [ ] Sounds do not spam during retries or repeated check-ins.

## Phase 13 - `vibeengine status`

- [ ] Implement one status command.
- [ ] Show local engine state.
- [ ] Show local capture health.
- [ ] Show queued event count.
- [ ] Show oldest pending event age.
- [ ] Show local DB size and configured cap.
- [ ] Show retention health.
- [ ] Show cloud reachability.
- [ ] Show subscription/entitlement state when known.
- [ ] Show project mappings.
- [ ] Show installed adapters.
- [ ] Show command automation availability.
- [ ] Start or repair engine when safe.
- [ ] Check API DNS resolution.
- [ ] Check HTTPS connectivity.
- [ ] Check TLS verification.
- [ ] Explain proxy/network/container issues clearly.
- [ ] Explain inactive subscription clearly: local capture continues, cloud
  upload/dashboard require subscription.
- [ ] Explain revoked/signed-out engine credentials clearly and tell the user to
  rerun the install command from the web app.
- [ ] Warn before local queue approaches the 30-day/100 MB retention cap.
- [ ] Redact tokens, command text, env vars, auth headers, cookies, and DB paths.
- [ ] Add golden tests for status output.

Acceptance gate:

- [ ] `vibeengine status` explains what is wrong and what to do next.
- [ ] No separate normal-use doctor/sync/upload/stats command is needed.

## Phase 14 - Free Private Beta Release

- [ ] Landing page is public.
- [ ] Pricing page explains private beta / trial timing.
- [ ] Login works.
- [ ] Invite codes work.
- [ ] `/projects` works.
- [ ] Project creation/deletion works.
- [ ] Install command works.
- [ ] Engine bootstrap works.
- [ ] Codex adapter works.
- [ ] Claude Code adapter works.
- [ ] Local SQLite capture works.
- [ ] Check-in upload works.
- [ ] Dashboard statuses work.
- [ ] Global today stats work.
- [ ] Success sound works.
- [ ] `vibeengine status` works.
- [ ] Metadata-only upload policy is implemented.
- [ ] Basic rate limits are enabled.
- [ ] Basic logs/metrics are available.
- [ ] Log redaction is verified.
- [ ] Private beta runbook exists.

Acceptance gate:

- [ ] A real user can create a project, install the engine, run a supported AI
  tool, finish a prompt, hear sound, and see dashboard stats.

## Phase 15 - Billing And Entitlements

- [ ] Add YooKassa subscription flow.
- [ ] Add 14-day trial.
- [ ] Add one personal plan at `399 RUB / month`.
- [ ] Do not require card for trial.
- [ ] Start trial when account is created.
- [ ] Add subscription states:
  - `trialing`
  - `active`
  - `grace`
  - `past_due`
  - `inactive`
  - `canceled`
- [ ] Add YooKassa webhook endpoint.
- [ ] Validate webhook authenticity.
- [ ] Make webhook handling idempotent.
- [ ] Add renewal reconciliation job.
- [ ] Add grace behavior around payment processing.
- [ ] Gate cloud API by entitlement.
- [ ] Pause cloud upload and hosted dashboard/history when trial/subscription is
  inactive.
- [ ] Let local engine keep local capture under local retention when cloud access
  is unavailable.
- [ ] Resume upload and dashboard updates after user subscribes/reactivates.
- [ ] Add clear UI for inactive/past-due state.
- [ ] Add subscription cancellation when account is deleted.
- [ ] Add subscription state to `vibeengine status`.

Acceptance gate:

- [ ] Subscription state cannot be bypassed for cloud service.
- [ ] Payment edge cases do not incorrectly lock out active users during normal
  renewal processing.

## Phase 16 - Command Automation Local

- [ ] Implement local supervisor state machine.
- [ ] Ensure browser/UI never directly mutates process state.
- [ ] Add approved command storage by command hash.
- [ ] Add start command.
- [ ] Add pause command.
- [ ] Add resume command.
- [ ] Add restart command.
- [ ] Add pause during prompt.
- [ ] Add resume/restart after prompt finish.
- [ ] Add auto-pause after ready.
- [ ] Add health command.
- [ ] Make health a subpart of command automation.
- [ ] Disable/mute dependent UI options when command automation is off without
  discarding saved choices.
- [ ] Add health `run_id` handling so stale results are ignored.
- [ ] Add pre-launch URL health behavior.
- [ ] Prevent restart loops.
- [ ] Prevent stale prompt finish from resuming old command.
- [ ] Add table tests for state transitions.
- [ ] Add race/recovery tests.

Acceptance gate:

- [ ] Prompt start can pause command and prompt finish can resume/restart it.
- [ ] Auto-pause after ready works after any restart/reactivation.
- [ ] No duplicate or stale event causes overlapping process control.

## Phase 17 - Remote Actions And Local Approval

- [ ] Add remote action model through check-in.
- [ ] Add safe action delivery.
- [ ] Add command proposals from web UI.
- [ ] Store command proposal text only in proposal payload.
- [ ] Do not store command text in normal events/logs/status/export stats.
- [ ] Add `vibeengine sync`.
- [ ] Show full proposed command diff locally.
- [ ] Require local confirmation for risky changes.
- [ ] Protect against accidental confirmation.
- [ ] Report approval/rejection/apply result to hosted API.
- [ ] Add pending proposal state in dashboard.
- [ ] Add tests for approval/rejection/idempotency.

Acceptance gate:

- [ ] Web can propose command changes, but local machine only applies them after
  explicit local approval.

## Phase 18 - Engine Updates And Release Signing

- [ ] Generate signing key outside the repository.
- [ ] Add dev signing key path to `.gitignore`.
- [ ] Add signed release artifact format.
- [ ] Verify signatures before install/update.
- [ ] Add auto-update check.
- [ ] Add safe binary swap.
- [ ] Add rollback path.
- [ ] Add update status in `vibeengine status`.
- [ ] Add release process checklist.
- [ ] Add old prototype hook cleanup.
- [ ] Preserve manual hooks during cleanup.
- [ ] Add update signature/rollback tests.

Acceptance gate:

- [ ] Engine updates cannot install unsigned or wrongly signed artifacts.
- [ ] Failed update rolls back without losing local data.

## Phase 19 - Export, Account Deletion, And Privacy

- [ ] Add safe export ZIP job.
- [ ] Export JSON metadata.
- [ ] Export CSV prompt event summaries.
- [ ] Use one export format only: ZIP with JSON metadata and CSV event
  summaries.
- [ ] Make export links short-lived.
- [ ] Limit export generation to max 3 per day per account.
- [ ] Exclude prompt text, responses, command text, env vars, tokens, file paths,
  repo metadata, logs, and raw DB dumps.
- [ ] Add account deletion request.
- [ ] Require email code confirmation for account deletion.
- [ ] Require strong confirmation, such as typing email.
- [ ] Cancel active subscription on account deletion.
- [ ] Soft-delete account/project records as documented.
- [ ] Keep only minimal billing/legal/fraud records required by payment, tax,
  accounting, or legal obligations after account deletion.
- [ ] Keep hosted events/stats while the account exists.
- [ ] Keep project history after project deletion.
- [ ] Apply local retention:
  - undelivered local events up to 30 days or DB cap
  - delivered local events 7 days
  - default local DB cap 100 MB
  - compact delivered diagnostics first
  - drop undelivered events only when disk/cap safety requires it
- [ ] Add privacy policy pages matching implementation.

Acceptance gate:

- [ ] User can export safe data.
- [ ] User can delete account with email-code confirmation.
- [ ] Deleted account cannot continue using cloud service.

## Phase 20 - Monitoring, Abuse Limits, And Hardening

- [ ] Add structured logs.
- [ ] Add metrics for check-ins, accepted/rejected events, queue depth, auth
  attempts, email sends, YooKassa webhooks, export/delete jobs, and errors.
- [ ] Add alert thresholds.
- [ ] Add basic admin/ops SQL views or scripts.
- [ ] Add request rate limits.
- [ ] Add account/project abuse limits.
- [ ] Add initial protective limits:
  - check-in token bucket around 60 requests/minute per engine credential
  - account check-ins around 600 requests/minute
  - event batch size max 500
  - check-in body max 1 MB
  - project creation max 50/day/account
  - bootstrap token create/exchange max 30/hour/account
  - strict auth/email-code limits per email/IP
  - export generation max 3/day/account
- [ ] Add check-in event batch caps.
- [ ] Add payload size caps.
- [ ] Add auth brute-force protections.
- [ ] Add webhook replay protection.
- [ ] Add audit events.
- [ ] Add security headers.
- [ ] Add CSRF protection where needed.
- [ ] Add secrets redaction tests.
- [ ] Add backup/restore plan for Postgres.
- [ ] Review risk register before private beta.
- [ ] Add tests or runbook items for every high-severity risk.

Acceptance gate:

- [ ] Real abuse produces "try again later" behavior instead of server failure.
- [ ] Operators can detect and debug production issues without raw sensitive
  data.

## Phase 21 - Production Readiness

- [ ] Prepare production Docker Compose.
- [ ] Configure domain and TLS.
- [ ] Configure production Postgres.
- [ ] Configure backups.
- [ ] Configure email provider.
- [ ] Configure YooKassa production credentials.
- [ ] Configure signing key storage.
- [ ] Configure server environment variables.
- [ ] Run clean install on Linux.
- [ ] Run clean install on macOS.
- [ ] Run clean install on Windows.
- [ ] Test Codex adapter on a real repo.
- [ ] Test Claude Code adapter on a real repo.
- [ ] Test dev container/network-restricted status diagnostics.
- [ ] Test hosted API outage behavior.
- [ ] Test local engine down behavior.
- [ ] Test disk-full/local DB failure behavior.
- [ ] Test account revocation/reinstall flow.
- [ ] Test `Disconnect my installs` behavior.
- [ ] Test access-token auth epoch invalidation.
- [ ] Test subscription inactive flow.
- [ ] Test account deletion with active subscription.
- [ ] Test project deletion with late offline event upload.
- [ ] Test local retention cap behavior.
- [ ] Run privacy checklist.
- [ ] Run security checklist.
- [ ] Run billing checklist.
- [ ] Run legal page checklist.
- [ ] Write launch runbook.

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
