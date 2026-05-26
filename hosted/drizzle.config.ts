import { defineConfig } from "drizzle-kit";

const defaultDatabaseUrl =
  "postgres://vibeengine:vibeengine_dev_password@localhost:55432/vibeengine";

export default defineConfig({
  schema: "./src/db/schema.ts",
  out: "./drizzle",
  dialect: "postgresql",
  dbCredentials: {
    url: process.env.HOSTED_DATABASE_URL ?? defaultDatabaseUrl,
  },
});
