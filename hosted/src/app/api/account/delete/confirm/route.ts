import { and, desc, eq, gt, isNull, sql } from "drizzle-orm";
import { NextResponse, type NextRequest } from "next/server";
import { z } from "zod";
import { createDb } from "@/db/client";
import {
  accounts,
  auditEvents,
  bootstrapTokens,
  commandProposals,
  dailyProjectStats,
  emailLoginCodes,
  engineCredentials,
  projectEvents,
  projects,
  remoteActions,
  sessions,
  subscriptions,
  users,
} from "@/db/schema";
import {
  checkRateLimit,
  checkRequestSize,
  checkSameOrigin,
  jsonError,
  requireVerifiedBrowserSession,
} from "@/lib/api/guards";
import {
  accountDeleteCodePurpose,
  sessionCookieName,
  verifySecret,
} from "@/lib/auth";
import { incrementMetric } from "@/lib/metrics";

const confirmDeleteSchema = z.object({
  email: z.string().email(),
  code: z.string().min(4).max(12),
});

export async function POST(request: NextRequest) {
  const guard =
    checkRequestSize(request) ??
    checkRateLimit(request, "auth_verify_code") ??
    checkSameOrigin(request);
  if (guard) {
    return guard;
  }

  const { response, auth } = await requireVerifiedBrowserSession(request);
  if (response) {
    return response;
  }

  const body = confirmDeleteSchema.safeParse(await request.json().catch(() => null));
  if (!body.success || body.data.email.trim().toLowerCase() !== auth.email) {
    return jsonError(
      400,
      "delete_confirmation_invalid",
      "Type your account email and a valid code to delete the account.",
    );
  }

  const { db, client } = createDb();
  try {
    const [code] = await db
      .select()
      .from(emailLoginCodes)
      .where(
        and(
          eq(emailLoginCodes.email, auth.email),
          eq(emailLoginCodes.purpose, accountDeleteCodePurpose),
          isNull(emailLoginCodes.consumedAt),
          gt(emailLoginCodes.expiresAt, new Date()),
        ),
      )
      .orderBy(desc(emailLoginCodes.createdAt))
      .limit(1);

    if (!code) {
      incrementMetric("account_delete_failed_total");
      return jsonError(400, "code_not_found", "Code is expired or missing.");
    }
    if (code.attemptCount >= 5) {
      incrementMetric("account_delete_failed_total");
      return jsonError(429, "too_many_attempts", "Too many code attempts.");
    }
    if (!verifySecret(body.data.code, code.codeHash)) {
      incrementMetric("account_delete_failed_total");
      await db
        .update(emailLoginCodes)
        .set({ attemptCount: code.attemptCount + 1 })
        .where(eq(emailLoginCodes.id, code.id));
      return jsonError(400, "invalid_code", "Code is not valid.");
    }

    await db.transaction(async (tx) => {
      const [account] = await tx
        .select({ id: accounts.id })
        .from(accounts)
        .where(and(eq(accounts.ownerUserId, auth.userId), isNull(accounts.deletedAt)))
        .limit(1);

      if (!account) {
        throw new Error("account_not_found");
      }

      const now = new Date();
      await tx
        .update(emailLoginCodes)
        .set({ consumedAt: now })
        .where(eq(emailLoginCodes.id, code.id));
      await tx
        .update(subscriptions)
        .set({ state: "canceled", updatedAt: now })
        .where(eq(subscriptions.accountId, account.id));
      await tx
        .update(projects)
        .set({ state: "deleted", deletedAt: now })
        .where(eq(projects.accountId, account.id));
      await tx
        .update(engineCredentials)
        .set({ revokedAt: now })
        .where(eq(engineCredentials.accountId, account.id));
      await tx
        .delete(bootstrapTokens)
        .where(eq(bootstrapTokens.accountId, account.id));
      await tx
        .delete(commandProposals)
        .where(eq(commandProposals.accountId, account.id));
      await tx
        .delete(remoteActions)
        .where(eq(remoteActions.accountId, account.id));
      await tx
        .delete(dailyProjectStats)
        .where(eq(dailyProjectStats.accountId, account.id));
      await tx
        .delete(projectEvents)
        .where(eq(projectEvents.accountId, account.id));
      await tx
        .update(sessions)
        .set({ revokedAt: now })
        .where(eq(sessions.userId, auth.userId));
      await tx
        .update(users)
        .set({
          deletedAt: now,
          engineAuthEpoch: sql`${users.engineAuthEpoch} + 1`,
        })
        .where(eq(users.id, auth.userId));
      await tx
        .update(accounts)
        .set({ deletedAt: now })
        .where(eq(accounts.id, account.id));
      await tx.insert(auditEvents).values({
        accountId: account.id,
        userId: auth.userId,
        type: "account_deleted",
      });
    });

    const result = NextResponse.json({ ok: true });
    incrementMetric("account_deleted_total");
    result.cookies.set(sessionCookieName, "", {
      path: "/",
      expires: new Date(0),
    });
    return result;
  } catch (error) {
    if (error instanceof Error && error.message === "account_not_found") {
      return jsonError(404, "account_not_found", "Account not found.");
    }
    throw error;
  } finally {
    await client.end();
  }
}
