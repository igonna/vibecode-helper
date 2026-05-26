import type { NextRequest } from "next/server";
import { and, eq, isNull } from "drizzle-orm";
import { z } from "zod";
import { createDb } from "@/db/client";
import { engineCredentials, users } from "@/db/schema";
import { checkRateLimit, checkRequestSize, jsonError } from "@/lib/api/guards";
import { hashSecret } from "@/lib/auth";
import { createEngineAccessToken } from "@/lib/engine-auth";

const tokenSchema = z.object({
  refreshToken: z.string().min(20),
});

export async function POST(request: NextRequest) {
  const guard = checkRequestSize(request) ?? checkRateLimit(request, "engine");
  if (guard) {
    return guard;
  }

  const body = tokenSchema.safeParse(await request.json().catch(() => null));
  if (!body.success) {
    return jsonError(400, "invalid_refresh", "Refresh token is required.");
  }

  const { db, client } = createDb();
  try {
    const [credential] = await db
      .select({
        accountId: engineCredentials.accountId,
        userId: engineCredentials.userId,
        credentialEpoch: engineCredentials.engineAuthEpoch,
        userEpoch: users.engineAuthEpoch,
      })
      .from(engineCredentials)
      .innerJoin(users, eq(engineCredentials.userId, users.id))
      .where(
        and(
          eq(engineCredentials.refreshTokenHash, hashSecret(body.data.refreshToken)),
          isNull(engineCredentials.revokedAt),
          isNull(users.deletedAt),
        ),
      )
      .limit(1);

    if (!credential || credential.credentialEpoch !== credential.userEpoch) {
      return jsonError(401, "engine_signed_out", "Install is signed out.");
    }

    await db
      .update(engineCredentials)
      .set({ lastSeenAt: new Date() })
      .where(
        eq(engineCredentials.refreshTokenHash, hashSecret(body.data.refreshToken)),
      );

    const access = createEngineAccessToken({
      accountId: credential.accountId,
      userId: credential.userId,
      epoch: credential.userEpoch,
    });

    return Response.json({
      accessToken: access.token,
      accessTokenExpiresAt: access.expiresAt.toISOString(),
      engineAuthEpoch: credential.userEpoch,
    });
  } finally {
    await client.end();
  }
}
