import { drizzle } from "drizzle-orm/postgres-js";
import postgres from "postgres";
import { getServerEnv } from "@/lib/env";
import * as schema from "./schema";

export function createDb(databaseUrl = getServerEnv().HOSTED_DATABASE_URL) {
  if (!databaseUrl) {
    throw new Error("HOSTED_DATABASE_URL is required for database access");
  }

  const client = postgres(databaseUrl, { max: 5 });
  return {
    client,
    db: drizzle(client, { schema }),
  };
}
