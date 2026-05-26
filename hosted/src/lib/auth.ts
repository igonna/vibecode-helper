import { createHash, randomBytes, timingSafeEqual } from "node:crypto";
import type { NextRequest } from "next/server";
import { and, eq, gt, isNull } from "drizzle-orm";
import { createDb } from "@/db/client";
import { sessions, users } from "@/db/schema";

export const sessionCookieName = "vibeengine_session";
export const loginCodePurpose = "login";
export const accountDeleteCodePurpose = "account_delete";

export function normalizeEmail(email: string) {
  return email.trim().toLowerCase();
}

export function hashSecret(secret: string) {
  return createHash("sha256").update(secret).digest("hex");
}

export function verifySecret(secret: string, hash: string) {
  const expected = Buffer.from(hashSecret(secret), "hex");
  const actual = Buffer.from(hash, "hex");
  return expected.length === actual.length && timingSafeEqual(expected, actual);
}

export function generateLoginCode() {
  return String(Math.floor(100000 + Math.random() * 900000));
}

export function generateSessionToken() {
  return randomBytes(32).toString("base64url");
}

export function sessionCookieOptions(expires: Date) {
  return {
    httpOnly: true,
    sameSite: "lax" as const,
    secure: process.env.NODE_ENV === "production",
    path: "/",
    expires,
  };
}

export async function getBrowserAuth(request: NextRequest) {
  const token = request.cookies.get(sessionCookieName)?.value;
  if (!token) {
    return null;
  }

  const sessionHash = hashSecret(token);
  const { db, client } = createDb();

  try {
    const [row] = await db
      .select({
        sessionId: sessions.id,
        userId: users.id,
        email: users.email,
        deletedAt: users.deletedAt,
      })
      .from(sessions)
      .innerJoin(users, eq(sessions.userId, users.id))
      .where(
        and(
          eq(sessions.sessionHash, sessionHash),
          isNull(sessions.revokedAt),
          gt(sessions.expiresAt, new Date()),
          isNull(users.deletedAt),
        ),
      )
      .limit(1);

    return row ?? null;
  } finally {
    await client.end();
  }
}
