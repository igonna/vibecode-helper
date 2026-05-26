import { and, eq, isNull } from "drizzle-orm";
import type { NextRequest } from "next/server";
import { z } from "zod";
import { createDb } from "@/db/client";
import { accounts, projects } from "@/db/schema";
import {
  jsonError,
  requireVerifiedBrowserSession,
} from "@/lib/api/guards";

const updateProjectSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1)
    .max(64)
    .regex(/^[a-zA-Z0-9][a-zA-Z0-9_-]*$/)
    .optional(),
});

type ProjectContext = {
  params: Promise<{ projectId: string }>;
};

async function getAccountId(userId: string) {
  const { db, client } = createDb();
  try {
    const [account] = await db
      .select({ id: accounts.id })
      .from(accounts)
      .where(and(eq(accounts.ownerUserId, userId), isNull(accounts.deletedAt)))
      .limit(1);
    return account?.id ?? null;
  } finally {
    await client.end();
  }
}

export async function GET(request: NextRequest, context: ProjectContext) {
  const { response, auth } = await requireVerifiedBrowserSession(request);
  if (response) {
    return response;
  }

  const accountId = await getAccountId(auth.userId);
  const { projectId } = await context.params;
  if (!accountId) {
    return jsonError(404, "account_not_found", "Account not found.");
  }

  const { db, client } = createDb();
  try {
    const [project] = await db
      .select()
      .from(projects)
      .where(
        and(
          eq(projects.id, projectId),
          eq(projects.accountId, accountId),
          isNull(projects.deletedAt),
        ),
      )
      .limit(1);

    if (!project) {
      return jsonError(404, "project_not_found", "Project not found.");
    }

    return Response.json({ project });
  } finally {
    await client.end();
  }
}

export async function PATCH(request: NextRequest, context: ProjectContext) {
  const { response, auth } = await requireVerifiedBrowserSession(request);
  if (response) {
    return response;
  }

  const body = updateProjectSchema.safeParse(await request.json().catch(() => null));
  if (!body.success) {
    return jsonError(400, "invalid_project_update", "Project update is invalid.");
  }

  const accountId = await getAccountId(auth.userId);
  const { projectId } = await context.params;
  if (!accountId) {
    return jsonError(404, "account_not_found", "Account not found.");
  }

  const { db, client } = createDb();
  try {
    const [project] = await db
      .update(projects)
      .set(body.data)
      .where(
        and(
          eq(projects.id, projectId),
          eq(projects.accountId, accountId),
          isNull(projects.deletedAt),
        ),
      )
      .returning();

    if (!project) {
      return jsonError(404, "project_not_found", "Project not found.");
    }

    return Response.json({ project });
  } catch (error) {
    if (
      typeof error === "object" &&
      error !== null &&
      "code" in error &&
      error.code === "23505"
    ) {
      return jsonError(409, "project_name_taken", "Project name is already used.");
    }
    throw error;
  } finally {
    await client.end();
  }
}

export async function DELETE(request: NextRequest, context: ProjectContext) {
  const { response, auth } = await requireVerifiedBrowserSession(request);
  if (response) {
    return response;
  }

  const accountId = await getAccountId(auth.userId);
  const { projectId } = await context.params;
  if (!accountId) {
    return jsonError(404, "account_not_found", "Account not found.");
  }

  const { db, client } = createDb();
  try {
    const [project] = await db
      .update(projects)
      .set({ state: "deleted", deletedAt: new Date() })
      .where(
        and(
          eq(projects.id, projectId),
          eq(projects.accountId, accountId),
          isNull(projects.deletedAt),
        ),
      )
      .returning({ id: projects.id });

    if (!project) {
      return jsonError(404, "project_not_found", "Project not found.");
    }

    return Response.json({ ok: true });
  } finally {
    await client.end();
  }
}
