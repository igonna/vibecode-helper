import type { NextRequest } from "next/server";
import { and, eq, gt, isNull, lt, or } from "drizzle-orm";
import { z } from "zod";
import { createDb } from "@/db/client";
import { emailLoginCodes, inviteCodes, users } from "@/db/schema";
import {
  checkRateLimit,
  checkRequestSize,
  jsonError,
} from "@/lib/api/guards";
import {
  generateLoginCode,
  hashSecret,
  loginCodePurpose,
  normalizeEmail,
} from "@/lib/auth";
import { sendLoginCode } from "@/lib/email";
import { incrementMetric } from "@/lib/metrics";

const requestCodeSchema = z.object({
  email: z.string().email(),
  inviteCode: z.string().min(1).optional(),
});

export async function POST(request: NextRequest) {
  const blocked =
    checkRequestSize(request) ?? checkRateLimit(request, "auth_request_code");
  if (blocked) {
    return blocked;
  }

  const body = requestCodeSchema.safeParse(await request.json().catch(() => null));
  if (!body.success) {
    return jsonError(400, "invalid_request", "Valid email is required.");
  }

  const email = normalizeEmail(body.data.email);
  const inviteCodeHash = body.data.inviteCode
    ? hashSecret(body.data.inviteCode)
    : null;
  const code = generateLoginCode();
  const codeHash = hashSecret(code);
  const expiresAt = new Date(Date.now() + 10 * 60 * 1000);
  const { db, client } = createDb();

  try {
    const [existingUser] = await db
      .select({ id: users.id })
      .from(users)
      .where(eq(users.email, email))
      .limit(1);

    if (!existingUser) {
      if (!inviteCodeHash) {
        return jsonError(
          403,
          "invite_required",
          "Invite code is required for new accounts.",
        );
      }

      const [invite] = await db
        .select({ id: inviteCodes.id })
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
        return jsonError(403, "invalid_invite", "Invite code is not valid.");
      }
    }

    await db
      .update(emailLoginCodes)
      .set({ consumedAt: new Date() })
      .where(
        and(
          eq(emailLoginCodes.email, email),
          eq(emailLoginCodes.purpose, loginCodePurpose),
          isNull(emailLoginCodes.consumedAt),
        ),
      );

    await db.insert(emailLoginCodes).values({
      email,
      codeHash,
      purpose: loginCodePurpose,
      inviteCodeHash,
      expiresAt,
    });

    await sendLoginCode({ email, code });
    incrementMetric("auth_code_requested_total");

    return Response.json({
      ok: true,
      devCode: process.env.NODE_ENV === "production" ? undefined : code,
    });
  } finally {
    await client.end();
  }
}
