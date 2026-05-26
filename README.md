# Vibeengine

Vibeengine is being rebuilt from the old local Vibecode Helper prototype into a
hosted platform plus a small local Go engine.

Current direction:

- hosted web app/API with Next.js and Postgres
- local Go engine/CLI for hook capture, durable local queue, command automation,
  status, and cloud check-in
- metadata-only prompt stats
- no prompt content upload
- no command logs upload
- no repo/file metadata upload
- first hook adapters: Codex and Claude Code

## Current Helper Isolation

The old local Vibecode Helper can keep running while Vibeengine is built.

Do not stop or remove the old Compose project `vibecode-dev-notify`, service
`helper`, port `8766`, or its volumes. New platform development must use
separate names and ports, documented in `docs/PLATFORM.md`.

## Active Docs

Use these files as the source of truth:

- [docs/PLATFORM.md](docs/PLATFORM.md) - product and architecture direction
- [docs/IMPLEMENTATION_CHECKLIST.md](docs/IMPLEMENTATION_CHECKLIST.md) - build
  checklist from scaffold to production readiness
- [docs/README.md](docs/README.md) - documentation index

## Repository Layout

The active code is intentionally small:

- `hosted/` - hosted Next.js web/API service
- `engine/` - local Go engine/CLI service
- `docs/` - product spec, implementation checklist, and documentation index
- `legacy/vibecode-dev-notify/` - old prototype source kept for reference only

The legacy folder is not used by the new Docker Compose project.

## Local Development

Local development dependencies:

```bash
docker compose up -d
docker compose ps
```

This starts the new `vibeengine-platform` Compose project only. It does not stop
or modify the old `vibecode-dev-notify` helper on port `8766`.

Production deployment starts from `docker-compose.prod.yml` and
`.env.production.example`; see [docs/LAUNCH_RUNBOOK.md](docs/LAUNCH_RUNBOOK.md).
