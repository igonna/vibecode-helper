import { and, desc, eq, gt, isNull, lt, or, sql } from "drizzle-orm";
import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { z } from "zod";
import { createDb } from "@/db/client";
import {
  accounts,
  auditEvents,
  emailLoginCodes,
  inviteCodes,
  sessions,
  subscriptions,
  users,
} from "@/db/schema";
import {
  checkRateLimit,
  checkRequestSize,
  jsonError,
} from "@/lib/api/guards";
import {
  generateSessionToken,
  hashSecret,
  loginCodePurpose,
  normalizeEmail,
  sessionCookieName,
  sessionCookieOptions,
  verifySecret,
} from "@/lib/auth";
import { incrementMetric } from "@/lib/metrics";

const verifyCodeSchema = z.object({
  email: z.string().email(),
  code: z.string().min(4).max(12),
  inviteCode: z.string().min(1).optional(),
});

export async function POST(request: NextRequest) {
  const blocked =
    checkRequestSize(request) ?? checkRateLimit(request, "auth_verify_code");
  if (blocked) {
    return blocked;
  }

  const body = verifyCodeSchema.safeParse(await request.json().catch(() => null));
  if (!body.success) {
    return jsonError(400, "invalid_request", "Valid email and code are required.");
  }

  const email = normalizeEmail(body.data.email);
  const { db, client } = createDb();

  try {
    const [loginCode] = await db
      .select()
      .from(emailLoginCodes)
      .where(
        and(
          eq(emailLoginCodes.email, email),
          eq(emailLoginCodes.purpose, loginCodePurpose),
          isNull(emailLoginCodes.consumedAt),
          gt(emailLoginCodes.expiresAt, new Date()),
        ),
      )
      .orderBy(desc(emailLoginCodes.createdAt))
      .limit(1);

    if (!loginCode) {
      incrementMetric("auth_code_verify_failed_total");
      return jsonError(400, "code_not_found", "Code is expired or missing.");
    }

    if (loginCode.attemptCount >= 5) {
      incrementMetric("auth_code_verify_failed_total");
      return jsonError(429, "too_many_attempts", "Too many code attempts.");
    }

    if (!verifySecret(body.data.code, loginCode.codeHash)) {
      incrementMetric("auth_code_verify_failed_total");
      await db
        .update(emailLoginCodes)
        .set({ attemptCount: loginCode.attemptCount + 1 })
        .where(eq(emailLoginCodes.id, loginCode.id));
      return jsonError(400, "invalid_code", "Code is not valid.");
    }

    const inviteCodeHash = body.data.inviteCode
      ? hashSecret(body.data.inviteCode)
      : loginCode.inviteCodeHash;
    const sessionToken = generateSessionToken();
    const sessionHash = hashSecret(sessionToken);
    const sessionExpiresAt = new Date(Date.now() + 30 * 24 * 60 * 60 * 1000);
    const trialEndsAt = new Date(Date.now() + 14 * 24 * 60 * 60 * 1000);

    const authResult = await db.transaction(async (tx) => {
      const [existingUser] = await tx
        .select({ id: users.id })
        .from(users)
        .where(eq(users.email, email))
        .limit(1);

      let userId = existingUser?.id;
      let accountId: string | null = null;

      if (!userId) {
        if (!inviteCodeHash) {
          throw new Error("invite_required");
        }

        const [invite] = await tx
          .select({
            id: inviteCodes.id,
            redeemedCount: inviteCodes.redeemedCount,
          })
          .from(inviteCodes)
          .where(
            and(
              eq(inviteCodes.codeHash, inviteCodeHash),
              isNull(inviteCodes.disabledAt),
              or(isNull(inviteCodes.expiresAt), gt(inviteCodes.expiresAt, new Date())),
              or(
                isNull(inviteCodes.maxRedemptions),
                lt(inviteCodes.redeemedCount, inviteCodes.maxRedemptions),
              ),
            ),
          )
          .limit(1);

        if (!invite) {
          throw new Error("invalid_invite");
        }

        const [newUser] = await tx
          .insert(users)
          .values({ email, emailVerifiedAt: new Date() })
          .returning({ id: users.id });
        userId = newUser.id;

        const [newAccount] = await tx
          .insert(accounts)
          .values({ ownerUserId: userId, name: email })
          .returning({ id: accounts.id });
        accountId = newAccount.id;

        await tx.insert(subscriptions).values({
          accountId,
          state: "trialing",
          plan: "personal",
          priceRub: 399,
          trialEndsAt,
        });

        await tx
          .update(inviteCodes)
          .set({ redeemedCount: sql`${inviteCodes.redeemedCount} + 1` })
          .where(eq(inviteCodes.id, invite.id));
      } else {
        await tx
          .update(users)
          .set({ emailVerifiedAt: new Date() })
          .where(eq(users.id, userId));

        const [account] = await tx
          .select({ id: accounts.id })
          .from(accounts)
          .where(eq(accounts.ownerUserId, userId))
          .limit(1);
        accountId = account?.id ?? null;
      }

      await tx
        .update(emailLoginCodes)
        .set({ consumedAt: new Date() })
        .where(eq(emailLoginCodes.id, loginCode.id));

      await tx.insert(sessions).values({
        userId,
        sessionHash,
        expiresAt: sessionExpiresAt,
      });

      await tx.insert(auditEvents).values({
        accountId,
        userId,
        type: existingUser ? "login" : "signup",
      });

      return { userId, accountId };
    });

    const response = NextResponse.json({ ok: true, ...authResult });
    incrementMetric("auth_code_verified_total");
    response.cookies.set(
      sessionCookieName,
      sessionToken,
      sessionCookieOptions(sessionExpiresAt),
    );
    return response;
  } catch (error) {
    if (error instanceof Error && error.message === "invite_required") {
      return jsonError(403, "invite_required", "Invite code is required.");
    }
    if (error instanceof Error && error.message === "invalid_invite") {
      return jsonError(403, "invalid_invite", "Invite code is not valid.");
    }
    throw error;
  } finally {
    await client.end();
  }
}
