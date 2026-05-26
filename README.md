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

## Active Docs

Use these files as the source of truth:

- [docs/PLATFORM.md](docs/PLATFORM.md) - product and architecture direction
- [docs/IMPLEMENTATION_CHECKLIST.md](docs/IMPLEMENTATION_CHECKLIST.md) - build
  checklist from scaffold to production readiness
- [docs/README.md](docs/README.md) - documentation index

## Implementation Status

Planning is documented. The next step is Phase 1 in
[docs/IMPLEMENTATION_CHECKLIST.md](docs/IMPLEMENTATION_CHECKLIST.md): create the
clean repository scaffold.
