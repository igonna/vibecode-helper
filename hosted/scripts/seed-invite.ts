import { createHash, randomBytes } from "node:crypto";
import postgres from "postgres";

const databaseUrl =
  process.env.HOSTED_DATABASE_URL ??
  "postgres://vibeengine:vibeengine_dev_password@localhost:55432/vibeengine";

const label = process.env.INVITE_LABEL ?? "private-beta";
const maxRedemptions = Number(process.env.INVITE_MAX_REDEMPTIONS ?? "25");
const code = process.env.INVITE_CODE ?? randomBytes(12).toString("base64url");
const codeHash = createHash("sha256").update(code).digest("hex");

async function main() {
  const sql = postgres(databaseUrl, { max: 1 });

  try {
    await sql`
      insert into invite_codes (code_hash, label, max_redemptions)
      values (${codeHash}, ${label}, ${maxRedemptions})
      on conflict (code_hash) do nothing
    `;

    console.log(`invite code: ${code}`);
  } finally {
    await sql.end();
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
