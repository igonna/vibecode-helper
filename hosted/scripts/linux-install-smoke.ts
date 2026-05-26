import { randomBytes, createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { mkdirSync, mkdtempSync, statSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import postgres from "postgres";

const databaseUrl =
  process.env.HOSTED_DATABASE_URL ??
  "postgres://vibeengine:vibeengine_dev_password@postgres:5432/vibeengine";
const apiBaseUrl = process.env.HOSTED_SMOKE_API ?? "http://127.0.0.1:8788";
const enginePath = path.join(process.cwd(), "releases", "vibeengine-linux-amd64");

function hashSecret(value: string) {
  return createHash("sha256").update(value).digest("hex");
}

function runEngine(home: string, args: string[], cwd: string) {
  return execFileSync(enginePath, args, {
    cwd,
    env: { ...process.env, VIBEENGINE_HOME: home },
    encoding: "utf8",
    stdio: ["pipe", "pipe", "pipe"],
  });
}

async function main() {
  statSync(enginePath);
  const sql = postgres(databaseUrl, { max: 1 });
  const suffix = randomBytes(6).toString("hex");
  const email = `linux-smoke-${suffix}@example.com`;
  const token = randomBytes(32).toString("base64url");
  const refreshTokenLabel = `linux-smoke-${suffix}`;
  const repo = mkdtempSync(path.join(os.tmpdir(), "vibeengine-smoke-repo-"));
  const home = mkdtempSync(path.join(os.tmpdir(), "vibeengine-smoke-home-"));
  mkdirSync(path.join(repo, ".git"));

  try {
    const [user] = await sql`
      insert into users (email, email_verified_at)
      values (${email}, now())
      returning id
    `;
    const [account] = await sql`
      insert into accounts (owner_user_id, name)
      values (${user.id}, ${email})
      returning id
    `;
    await sql`
      insert into subscriptions (account_id, state, plan, price_rub, trial_ends_at)
      values (${account.id}, 'trialing', 'personal', 399, now() + interval '14 days')
    `;
    const [project] = await sql`
      insert into projects (account_id, name, state, last_status)
      values (${account.id}, ${`linux-smoke-${suffix}`}, 'active', 'Setup')
      returning id
    `;
    await sql`
      insert into bootstrap_tokens (account_id, project_id, user_id, token_hash, expires_at)
      values (${account.id}, ${project.id}, ${user.id}, ${hashSecret(token)}, now() + interval '15 minutes')
    `;

    runEngine(
      home,
      [
        "bootstrap",
        "--api",
        apiBaseUrl,
        "--token",
        token,
        "--repo",
        repo,
        "--adapter",
        "codex",
        "--adapter",
        "claude-code",
      ],
      repo,
    );

    runEngine(
      home,
      ["adapter", "codex", "UserPromptSubmit", "--project", project.id],
      repo,
    );
    runEngine(home, ["adapter", "codex", "Stop", "--project", project.id], repo);
    runEngine(
      home,
      ["adapter", "claude-code", "UserPromptSubmit", "--project", project.id],
      repo,
    );
    runEngine(home, ["adapter", "claude-code", "Stop", "--project", project.id], repo);
    runEngine(home, ["check-in"], repo);

    const [events] = await sql`
      select count(*)::int as count
      from project_events
      where project_id = ${project.id}
    `;
    if (events.count < 4) {
      throw new Error(`expected at least 4 uploaded events, got ${events.count}`);
    }

    const status = runEngine(home, ["status", "--no-repair"], repo);
    if (!status.includes("Projects: 1 mapped")) {
      throw new Error(`unexpected status output: ${status}`);
    }

    console.log(`linux install smoke passed (${refreshTokenLabel})`);
  } finally {
    await sql.end();
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
