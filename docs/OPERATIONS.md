# Operations Runbook

## Metrics

Private beta metrics are exposed at `/api/internal/metrics`.

In production set `HOSTED_METRICS_TOKEN` and request the route with:

```sh
curl -H "Authorization: Bearer $HOSTED_METRICS_TOKEN" https://example.com/api/internal/metrics
```

Initial alert thresholds:

- sustained `request_rate_limited_total` growth: investigate abuse or broken
  clients
- any sustained `engine_check_in_rejected_total` growth: check auth,
  subscription, and payload validation
- any sustained `yookassa_webhook_rejected_total` growth: check YooKassa
  credentials and replay attempts
- any sustained `email_send_failed_total` growth: check SMTP provider
- any failed backup job: fix before paid beta traffic

## SQL Views

Useful private-beta checks:

```sql
select state, count(*) from subscriptions group by state order by state;

select date_trunc('hour', created_at) as hour, count(*)
from project_events
where created_at > now() - interval '24 hours'
group by hour
order by hour;

select provider, type, count(*)
from billing_events
where created_at > now() - interval '7 days'
group by provider, type
order by provider, type;

select type, count(*)
from audit_events
where created_at > now() - interval '7 days'
group by type
order by type;
```

## Postgres Backup

For the Docker Compose deployment:

```sh
mkdir -p backups
docker compose exec -T postgres pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" \
  > "backups/vibeengine-$(date -u +%Y%m%dT%H%M%SZ).sql"
```

Before paid beta:

- run backups automatically at least daily
- store backups off-host
- test restore into a fresh Postgres container
- monitor backup age

## High-Severity Risks

- Command automation loops or stale prompt events: covered by local state
  machine tests, stale prompt id checks, and restart backoff.
- Wrong day stats: source events are UTC; dashboard day grouping uses browser
  timezone only for display.
- Billing locks out active users: YooKassa webhooks are idempotent, renewal
  failures move to grace, and entitlement checks understand grace.
- Unsafe updates: install/update verify checksum and RSA signature before swap.
- Sensitive data in exports/logs/status: exports are allowlisted, command
  proposal text is excluded from normal events, and redaction tests cover local
  status/log helpers.
- Old helper hooks double count events: `vibeengine hooks cleanup-legacy`
  removes only generated old-helper commands and preserves manual hooks.
