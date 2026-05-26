# Launch Runbook

## Production Compose

Use `docker-compose.prod.yml` with a production `.env` derived from
`.env.production.example`.

```sh
docker compose -f docker-compose.prod.yml --env-file .env.production up -d --build
docker compose -f docker-compose.prod.yml --env-file .env.production exec -T web npm run db:migrate
```

Terminate TLS at a reverse proxy in front of `127.0.0.1:8788`. The hosted app
must see `HOSTED_PUBLIC_URL=https://your-domain`.

Before a real launch, run:

```sh
cd hosted
npm run prod:check -- --env-file ../.env.production
npm run prod:check -- --env-file ../.env.production --network
```

The first command validates production values and placeholder secrets. The
second also checks DNS/TLS for `HOSTED_PUBLIC_URL` and Postgres connectivity
from the machine where it runs.

## Required External Configuration

- domain and TLS certificate
- RuSender SMTP credentials
- YooKassa shop id and secret key
- production signing key at `~/.config/vibeengine/release-signing-private.pem`
- off-host Postgres backup destination

## Release

1. Build engine artifacts.
2. Run `npm run release:sign` in `hosted/`.
3. Build and restart the web service.
4. Verify `/api/health`.
5. Verify one served artifact signature.
6. Run a clean Linux install from a test repo.

## Final Checklists

- Privacy: safe export excludes sensitive fields; privacy page matches behavior.
- Security: rate limits, CSRF, security headers, signed updates, auth revocation.
- Billing: YooKassa checkout, webhook idempotency, grace behavior, cancellation.
- Legal: terms, privacy, refund, contact pages match actual implementation.
