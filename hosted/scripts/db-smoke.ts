import postgres from "postgres";

const databaseUrl =
  process.env.HOSTED_DATABASE_URL ??
  "postgres://vibeengine:vibeengine_dev_password@localhost:55432/vibeengine";

async function main() {
  const sql = postgres(databaseUrl, { max: 1 });

  try {
    const result = await sql<{ ok: number }[]>`select 1 as ok`;
    if (result[0]?.ok !== 1) {
      throw new Error("unexpected smoke query result");
    }
    console.log("database smoke ok");
  } finally {
    await sql.end();
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
