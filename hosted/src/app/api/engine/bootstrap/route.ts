import type { NextRequest } from "next/server";
import { randomBytes } from "node:crypto";
import { and, eq, gt, isNull } from "drizzle-orm";
import { z } from "zod";
import { createDb } from "@/db/client";
import {
  bootstrapTokens,
  engineCredentials,
  projects,
  users,
} from "@/db/schema";
import {
  checkRateLimit,
  checkRequestSize,
  jsonError,
} from "@/lib/api/guards";
import { hashSecret } from "@/lib/auth";
import { createEngineAccessToken } from "@/lib/engine-auth";

const bootstrapSchema = z.object({
  token: z.string().min(20),
});

export async function POST(request: NextRequest) {
  const guard =
    checkRequestSize(request) ?? checkRateLimit(request, "engine_bootstrap");
  if (guard) {
    return guard;
  }

  const body = bootstrapSchema.safeParse(await request.json().catch(() => null));
  if (!body.success) {
    return jsonError(400, "invalid_bootstrap", "Bootstrap token is required.");
  }

  const tokenHash = hashSecret(body.data.token);
  const refreshToken = randomBytes(32).toString("base64url");
  const now = new Date();
  const { db, client } = createDb();

  try {
    const result = await db.transaction(async (tx) => {
      const [bootstrap] = await tx
        .select({
          id: bootstrapTokens.id,
          accountId: bootstrapTokens.accountId,
          projectId: bootstrapTokens.projectId,
          userId: bootstrapTokens.userId,
          projectName: projects.name,
          engineAuthEpoch: users.engineAuthEpoch,
        })
        .from(bootstrapTokens)
        .innerJoin(projects, eq(bootstrapTokens.projectId, projects.id))
        .innerJoin(users, eq(bootstrapTokens.userId, users.id))
        .where(
          and(
            eq(bootstrapTokens.tokenHash, tokenHash),
            gt(bootstrapTokens.expiresAt, now),
            isNull(bootstrapTokens.consumedAt),
            isNull(projects.deletedAt),
            isNull(users.deletedAt),
          ),
        )
        .limit(1);

      if (!bootstrap) {
        return null;
      }

      const [consumed] = await tx
        .update(bootstrapTokens)
        .set({ consumedAt: now })
        .where(
          and(
            eq(bootstrapTokens.id, bootstrap.id),
            isNull(bootstrapTokens.consumedAt),
          ),
        )
        .returning({ id: bootstrapTokens.id });

      if (!consumed) {
        return null;
      }

      await tx.insert(engineCredentials).values({
        accountId: bootstrap.accountId,
        userId: bootstrap.userId,
        refreshTokenHash: hashSecret(refreshToken),
        engineAuthEpoch: bootstrap.engineAuthEpoch,
      });

      return bootstrap;
    });

    if (!result) {
      return jsonError(
        401,
        "bootstrap_invalid",
        "Install token is expired, already used, or invalid.",
      );
    }

    const access = createEngineAccessToken({
      accountId: result.accountId,
      userId: result.userId,
      epoch: result.engineAuthEpoch,
    });

    return Response.json({
      accountId: result.accountId,
      projectId: result.projectId,
      projectName: result.projectName,
      userId: result.userId,
      refreshToken,
      accessToken: access.token,
      accessTokenExpiresAt: access.expiresAt.toISOString(),
      engineAuthEpoch: result.engineAuthEpoch,
    });
  } finally {
    await client.end();
  }
}
