import { and, eq, isNull, sql } from "drizzle-orm";
import type { NextRequest } from "next/server";
import { createDb } from "@/db/client";
import { accounts, auditEvents, engineCredentials, users } from "@/db/schema";
import {
  checkRateLimit,
  checkRequestSize,
  checkSameOrigin,
  jsonError,
  requireVerifiedBrowserSession,
} from "@/lib/api/guards";
import { incrementMetric, logInfo } from "@/lib/metrics";

export async function POST(request: NextRequest) {
  const guard =
    checkRequestSize(request) ??
    checkRateLimit(request, "browser") ??
    checkSameOrigin(request);
  if (guard) {
    return guard;
  }

  const { response, auth } = await requireVerifiedBrowserSession(request);
  if (response) {
    return response;
  }

  const { db, client } = createDb();
  try {
    const [account] = await db
      .select({ id: accounts.id })
      .from(accounts)
      .where(and(eq(accounts.ownerUserId, auth.userId), isNull(accounts.deletedAt)))
      .limit(1);

    if (!account) {
      return jsonError(404, "account_not_found", "Account not found.");
    }

    await db.transaction(async (tx) => {
      const now = new Date();
      await tx
        .update(engineCredentials)
        .set({ revokedAt: now })
        .where(eq(engineCredentials.accountId, account.id));
      await tx
        .update(users)
        .set({ engineAuthEpoch: sql`${users.engineAuthEpoch} + 1` })
        .where(eq(users.id, auth.userId));
      await tx.insert(auditEvents).values({
        accountId: account.id,
        userId: auth.userId,
        type: "engine_installs_disconnected",
      });
    });

    incrementMetric("engine_installs_disconnected_total");
    logInfo("engine_installs_disconnected", { account_id: account.id });

    return Response.json({ ok: true });
  } finally {
    await client.end();
  }
}
