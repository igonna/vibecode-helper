# Private Beta Runbook

Use this runbook for the first private beta only. The old
`vibecode-dev-notify` helper is separate and must not be stopped by these
steps.

## Local Platform

1. Start the new platform:

```bash
docker compose up -d
```

2. Apply database migrations and seed the private beta invite from the web
   container:

```bash
docker compose exec -T web npm run db:migrate
docker compose exec -T web npm run seed:invite
```

3. Check health:

```bash
docker compose exec -T web wget -qO- http://127.0.0.1:8788/api/health
```

4. Keep the old helper untouched:

```bash
docker ps --filter name=vibecode-dev-notify-helper-1
```

## Engine Artifacts

For local beta testing, build static Linux artifacts and stage them into the web
container:

```bash
cd engine
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../hosted/releases/vibeengine-linux-amd64 .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o ../hosted/releases/vibeengine-linux-arm64 .
chmod 700 ../hosted/releases/vibeengine-linux-*
cd ..
docker cp hosted/releases/vibeengine-linux-amd64 vibeengine-platform-web-1:/app/releases/vibeengine-linux-amd64
docker cp hosted/releases/vibeengine-linux-arm64 vibeengine-platform-web-1:/app/releases/vibeengine-linux-arm64
```

Signed production releases belong to the release-signing phase.

## Beta User Flow

1. Open `/login`.
2. Sign up with email and the current invite code.
3. Create a project in `/projects`.
4. Open the project side panel and copy the install command.
5. Run it inside a separate test repo.
6. Start Codex or Claude Code in that repo and run a prompt.
7. Confirm:
   - local hooks are installed
   - `vibeengine status` is understandable
   - prompt events upload after check-in
   - dashboard status and global stats update
   - sound plays after browser sound is enabled

## Privacy Checks

Normal event upload must never persist prompt text, responses, transcript paths,
command output, command text, file paths, repo metadata, environment variables,
tokens, auth headers, or cookies.

If a user reports missing cloud data, local capture can still continue. Ask them
to run:

```bash
vibeengine status
```

If credentials are revoked or stale, they should run the install command from
the web app again.
