import type { NextRequest } from "next/server";
import { and, eq, isNull } from "drizzle-orm";
import { z } from "zod";
import { createDb } from "@/db/client";
import { accounts, projects, remoteActions } from "@/db/schema";
import {
  checkRateLimit,
  checkRequestSize,
  checkSameOrigin,
  jsonError,
  requireVerifiedBrowserSession,
} from "@/lib/api/guards";

type ProjectContext = {
  params: Promise<{ projectId: string }>;
};

const actionSchema = z.object({
  action: z.enum(["start", "pause", "resume", "restart"]),
});

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
  const body = actionSchema.safeParse(await request.json().catch(() => null));
  if (!body.success) {
    return jsonError(400, "invalid_action", "Command action is invalid.");
  }

  const { projectId } = await context.params;
  const { db, client } = createDb();
  try {
    const [project] = await db
      .select({ id: projects.id, accountId: projects.accountId })
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

    const [action] = await db
      .insert(remoteActions)
      .values({
        accountId: project.accountId,
        projectId: project.id,
        type: `command_${body.data.action}`,
        state: "pending",
      })
      .returning({
        id: remoteActions.id,
        state: remoteActions.state,
        type: remoteActions.type,
      });

    return Response.json({ action }, { status: 201 });
  } finally {
    await client.end();
  }
}
