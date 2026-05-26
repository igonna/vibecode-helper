import { createHash, randomBytes } from "node:crypto";
import postgres from "postgres";

const databaseUrl =
  process.env.HOSTED_DATABASE_URL ??
  "postgres://vibeengine:vibeengine_dev_password@postgres:5432/vibeengine";
const apiBaseUrl = process.env.HOSTED_SMOKE_API ?? "http://127.0.0.1:8788";
const smokeOrigin = process.env.HOSTED_SMOKE_ORIGIN;
const sessionCookieName = "vibeengine_session";

function hashSecret(value: string) {
  return createHash("sha256").update(value).digest("hex");
}

function assert(condition: unknown, message: string): asserts condition {
  if (!condition) {
    throw new Error(message);
  }
}

async function requestJson<T>(
  path: string,
  init: RequestInit & { expectedStatus?: number; sessionToken?: string } = {},
) {
  const expectedStatus = init.expectedStatus ?? 200;
  const headers = new Headers(init.headers);
  if (smokeOrigin) {
    headers.set("origin", smokeOrigin);
  }
  if (init.body && !headers.has("content-type")) {
    headers.set("content-type", "application/json");
  }
  if (init.sessionToken) {
    headers.set("cookie", `${sessionCookieName}=${init.sessionToken}`);
  }

  const response = await fetch(`${apiBaseUrl}${path}`, {
    ...init,
    headers,
  });
  const text = await response.text();
  const body = text ? (JSON.parse(text) as T) : (null as T);
  if (response.status !== expectedStatus) {
    throw new Error(
      `${path} returned ${response.status}, expected ${expectedStatus}: ${text}`,
    );
  }
  return body;
}

async function createAccount(
  sql: postgres.Sql,
  suffix: string,
  subscriptionState: "trialing" | "active" = "active",
) {
  const email = `lifecycle-smoke-${suffix}@example.com`;
  const [user] = await sql<{ id: string }[]>`
    insert into users (email, email_verified_at)
    values (${email}, now())
    returning id
  `;
  const [account] = await sql<{ id: string }[]>`
    insert into accounts (owner_user_id, name)
    values (${user.id}, ${email})
    returning id
  `;
  await sql`
    insert into subscriptions (
      account_id,
      state,
      plan,
      price_rub,
      trial_ends_at,
      current_period_end,
      yookassa_payment_method_id
    )
    values (
      ${account.id},
      ${subscriptionState},
      'personal',
      399,
      now() + interval '14 days',
      now() + interval '30 days',
      ${`pm_${suffix}`}
    )
  `;
  const [project] = await sql<{ id: string }[]>`
    insert into projects (account_id, name, state, last_status)
    values (${account.id}, ${`lifecycle-smoke-${suffix}`}, 'active', 'Idle')
    returning id
  `;
  return { email, userId: user.id, accountId: account.id, projectId: project.id };
}

async function createSession(sql: postgres.Sql, userId: string) {
  const sessionToken = randomBytes(32).toString("base64url");
  await sql`
    insert into sessions (user_id, session_hash, expires_at)
    values (${userId}, ${hashSecret(sessionToken)}, now() + interval '1 hour')
  `;
  return sessionToken;
}

async function createBootstrapToken(
  sql: postgres.Sql,
  accountId: string,
  projectId: string,
  userId: string,
) {
  const token = randomBytes(32).toString("base64url");
  await sql`
    insert into bootstrap_tokens (account_id, project_id, user_id, token_hash, expires_at)
    values (${accountId}, ${projectId}, ${userId}, ${hashSecret(token)}, now() + interval '15 minutes')
  `;
  return token;
}

async function createCredential(sql: postgres.Sql, accountId: string, userId: string) {
  const refreshToken = randomBytes(32).toString("base64url");
  await sql`
    insert into engine_credentials (account_id, user_id, refresh_token_hash, engine_auth_epoch)
    values (${accountId}, ${userId}, ${hashSecret(refreshToken)}, 0)
  `;
  return refreshToken;
}

async function getAccessToken(refreshToken: string, expectedStatus = 200) {
  return requestJson<{
    accessToken: string;
    engineAuthEpoch: number;
    error?: { code: string };
  }>("/api/engine/token", {
    method: "POST",
    body: JSON.stringify({ refreshToken }),
    expectedStatus,
  });
}

async function checkIn(
  accessToken: string,
  projectId: string,
  localEventId: string,
  expectedStatus = 200,
) {
  return requestJson<{
    acceptedEventIds?: string[];
    error?: { code: string };
  }>("/api/engine/check-in", {
    method: "POST",
    headers: { authorization: `Bearer ${accessToken}` },
    body: JSON.stringify({
      events: [
        {
          localEventId,
          projectId,
          toolAdapter: "codex",
          nativeEventName: "Stop",
          type: "prompt_finished",
          createdAtUtc: new Date().toISOString(),
        },
      ],
      projectStatuses: [{ projectId, status: "success" }],
    }),
    expectedStatus,
  });
}

async function smokeRevocationAndReinstall(sql: postgres.Sql) {
  const suffix = randomBytes(6).toString("hex");
  const account = await createAccount(sql, `revocation-${suffix}`);
  const sessionToken = await createSession(sql, account.userId);
  const refreshToken = await createCredential(sql, account.accountId, account.userId);
  const firstToken = await getAccessToken(refreshToken);
  assert(firstToken.accessToken, "expected initial access token");

  const firstCheckIn = await checkIn(
    firstToken.accessToken,
    account.projectId,
    `before-revoke-${suffix}`,
  );
  assert(
    firstCheckIn.acceptedEventIds?.includes(`before-revoke-${suffix}`),
    "expected check-in before revocation to be accepted",
  );

  await requestJson<{ ok: boolean }>("/api/account/disconnect-installs", {
    method: "POST",
    sessionToken,
    expectedStatus: 200,
  });

  const revokedAccess = await checkIn(
    firstToken.accessToken,
    account.projectId,
    `after-revoke-${suffix}`,
    401,
  );
  assert(
    revokedAccess.error?.code === "engine_signed_out",
    "expected old access token to be rejected after disconnect",
  );

  const revokedRefresh = await getAccessToken(refreshToken, 401);
  assert(
    revokedRefresh.error?.code === "engine_signed_out",
    "expected old refresh token to be rejected after disconnect",
  );

  const bootstrapToken = await createBootstrapToken(
    sql,
    account.accountId,
    account.projectId,
    account.userId,
  );
  const reinstall = await requestJson<{
    accessToken: string;
    refreshToken: string;
    engineAuthEpoch: number;
  }>("/api/engine/bootstrap", {
    method: "POST",
    body: JSON.stringify({ token: bootstrapToken }),
    expectedStatus: 200,
  });
  assert(reinstall.accessToken, "expected reinstall access token");
  assert(reinstall.refreshToken, "expected reinstall refresh token");
  assert(reinstall.engineAuthEpoch === 1, "expected reinstall to use bumped epoch");

  const reinstalledCheckIn = await checkIn(
    reinstall.accessToken,
    account.projectId,
    `after-reinstall-${suffix}`,
  );
  assert(
    reinstalledCheckIn.acceptedEventIds?.includes(`after-reinstall-${suffix}`),
    "expected check-in after reinstall to be accepted",
  );
}

async function smokeDeletedProjectLateUpload(sql: postgres.Sql) {
  const suffix = randomBytes(6).toString("hex");
  const account = await createAccount(sql, `deleted-project-${suffix}`);
  const sessionToken = await createSession(sql, account.userId);
  const refreshToken = await createCredential(sql, account.accountId, account.userId);
  const token = await getAccessToken(refreshToken);

  await requestJson<{ ok: boolean }>(`/api/projects/${account.projectId}`, {
    method: "DELETE",
    sessionToken,
    expectedStatus: 200,
  });

  const lateEventId = `late-offline-${suffix}`;
  const lateCheckIn = await checkIn(token.accessToken, account.projectId, lateEventId);
  assert(
    lateCheckIn.acceptedEventIds?.includes(lateEventId),
    "expected late event for deleted project to be accepted for history",
  );

  const [eventCount] = await sql<{ count: number }[]>`
    select count(*)::int as count
    from project_events
    where project_id = ${account.projectId}
      and local_event_id = ${lateEventId}
  `;
  assert(eventCount.count === 1, "expected late deleted-project event in history");

  const [project] = await sql<{ state: string; deleted_at: Date | null }[]>`
    select state, deleted_at
    from projects
    where id = ${account.projectId}
  `;
  assert(project.state === "deleted", "expected project to remain deleted");
  assert(project.deleted_at, "expected deleted project timestamp to stay set");
}

async function smokeAccountDeletionWithActiveSubscription(sql: postgres.Sql) {
  const suffix = randomBytes(6).toString("hex");
  const account = await createAccount(sql, `account-delete-${suffix}`, "active");
  const sessionToken = await createSession(sql, account.userId);
  await createCredential(sql, account.accountId, account.userId);
  await sql`
    insert into project_events (
      account_id,
      project_id,
      local_event_id,
      tool_adapter,
      type,
      occurred_at
    )
    values (
      ${account.accountId},
      ${account.projectId},
      ${`delete-smoke-${suffix}`},
      'codex',
      'prompt_finished',
      now()
    )
  `;

  const deleteCode = "123456";
  await sql`
    insert into email_login_codes (email, code_hash, purpose, expires_at)
    values (${account.email}, ${hashSecret(deleteCode)}, 'account_delete', now() + interval '10 minutes')
  `;

  await requestJson<{ ok: boolean }>("/api/account/delete/confirm", {
    method: "POST",
    sessionToken,
    body: JSON.stringify({ email: account.email, code: deleteCode }),
    expectedStatus: 200,
  });

  const [row] = await sql<{
    user_deleted_at: Date | null;
    account_deleted_at: Date | null;
    project_deleted_at: Date | null;
    subscription_state: string;
    revoked_credentials: number;
    remaining_events: number;
  }[]>`
    select
      users.deleted_at as user_deleted_at,
      accounts.deleted_at as account_deleted_at,
      projects.deleted_at as project_deleted_at,
      subscriptions.state as subscription_state,
      (
        select count(*)::int
        from engine_credentials
        where account_id = accounts.id and revoked_at is not null
      ) as revoked_credentials,
      (
        select count(*)::int
        from project_events
        where account_id = accounts.id
      ) as remaining_events
    from accounts
    join users on users.id = accounts.owner_user_id
    join projects on projects.account_id = accounts.id
    join subscriptions on subscriptions.account_id = accounts.id
    where accounts.id = ${account.accountId}
  `;

  assert(row.user_deleted_at, "expected account deletion to soft-delete user");
  assert(row.account_deleted_at, "expected account deletion to soft-delete account");
  assert(row.project_deleted_at, "expected account deletion to soft-delete projects");
  assert(
    row.subscription_state === "canceled",
    "expected active subscription to be canceled",
  );
  assert(row.revoked_credentials > 0, "expected engine credentials to be revoked");
  assert(row.remaining_events === 0, "expected account deletion to remove event data");
}

async function main() {
  const sql = postgres(databaseUrl, { max: 1 });
  try {
    await smokeRevocationAndReinstall(sql);
    await smokeDeletedProjectLateUpload(sql);
    await smokeAccountDeletionWithActiveSubscription(sql);
    console.log("lifecycle smoke passed");
  } finally {
    await sql.end();
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
