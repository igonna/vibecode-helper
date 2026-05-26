import type { NextRequest } from "next/server";
import { randomBytes } from "node:crypto";
import { and, eq, gte, isNull, sql } from "drizzle-orm";
import { createDb } from "@/db/client";
import { accounts, bootstrapTokens, projects } from "@/db/schema";
import {
  checkRateLimit,
  checkRequestSize,
  checkSameOrigin,
  jsonError,
  requireVerifiedBrowserSession,
} from "@/lib/api/guards";
import { hashSecret } from "@/lib/auth";
import { getServerEnv } from "@/lib/env";
import { incrementMetric } from "@/lib/metrics";

type ProjectContext = {
  params: Promise<{ projectId: string }>;
};

export async function POST(request: NextRequest, context: ProjectContext) {
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

  const { projectId } = await context.params;
  const { db, client } = createDb();

  try {
    const [project] = await db
      .select({
        id: projects.id,
        name: projects.name,
        accountId: projects.accountId,
      })
      .from(projects)
      .innerJoin(accounts, eq(projects.accountId, accounts.id))
      .where(
        and(
          eq(projects.id, projectId),
          eq(accounts.ownerUserId, auth.userId),
          isNull(projects.deletedAt),
          isNull(accounts.deletedAt),
        ),
      )
      .limit(1);

    if (!project) {
      return jsonError(404, "project_not_found", "Project not found.");
    }

    const token = randomBytes(32).toString("base64url");
    const expiresAt = new Date(Date.now() + 15 * 60 * 1000);
    const hourStart = new Date(Date.now() - 60 * 60 * 1000);
    const [{ count }] = await db
      .select({ count: sql<number>`count(*)::int` })
      .from(bootstrapTokens)
      .where(
        and(
          eq(bootstrapTokens.accountId, project.accountId),
          gte(bootstrapTokens.createdAt, hourStart),
        ),
      );
    if (Number(count) >= 30) {
      return jsonError(
        429,
        "bootstrap_limit_reached",
        "Install command limit reached. Try again later.",
      );
    }

    await db.insert(bootstrapTokens).values({
      accountId: project.accountId,
      projectId: project.id,
      userId: auth.userId,
      tokenHash: hashSecret(token),
      expiresAt,
    });
    incrementMetric("bootstrap_token_created_total");

    const baseUrl = getServerEnv().HOSTED_PUBLIC_URL.replace(/\/$/, "");
    const shellCommand = `curl -fsSL ${baseUrl}/install.sh | sh -s -- --api ${baseUrl} --token ${token}`;
    const powershellCommand = `powershell -NoProfile -ExecutionPolicy Bypass -Command "iwr '${baseUrl}/install.ps1' -OutFile $env:TEMP\\vibeengine-install.ps1; & $env:TEMP\\vibeengine-install.ps1 -Api '${baseUrl}' -Token '${token}'"`;

    return Response.json({
      token,
      expiresAt: expiresAt.toISOString(),
      project: { id: project.id, name: project.name },
      commands: {
        shell: shellCommand,
        powershell: powershellCommand,
      },
    });
  } finally {
    await client.end();
  }
}
