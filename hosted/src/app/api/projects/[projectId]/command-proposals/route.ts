import type { NextRequest } from "next/server";
import { createHash } from "node:crypto";
import { and, eq, isNull } from "drizzle-orm";
import { z } from "zod";
import { createDb } from "@/db/client";
import { accounts, commandProposals, projects } from "@/db/schema";
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

const proposalSchema = z.object({
  command: z.string().trim().min(1).max(4000),
  pauseDuringPrompt: z.boolean().default(false),
  autoPauseAfterReadySeconds: z.number().int().min(0).max(86400).default(0),
  healthCommand: z.string().trim().max(2000).optional(),
  afterPrompt: z.enum(["resume", "restart", "none"]).default("resume"),
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
  const body = proposalSchema.safeParse(await request.json().catch(() => null));
  if (!body.success) {
    return jsonError(400, "invalid_command_proposal", "Command proposal is invalid.");
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

    const commandHash = createHash("sha256")
      .update(body.data.command)
      .digest("hex");
    const [proposal] = await db
      .insert(commandProposals)
      .values({
        accountId: project.accountId,
        projectId: project.id,
        type: "command_settings",
        state: "pending_device",
        commandHash,
        payloadJson: {
          command: body.data.command,
          pause_during_prompt: body.data.pauseDuringPrompt,
          auto_pause_after_ready_seconds: body.data.autoPauseAfterReadySeconds,
          health_command: body.data.healthCommand ?? "",
          after_prompt: body.data.afterPrompt,
        },
      })
      .returning({
        id: commandProposals.id,
        state: commandProposals.state,
        commandHash: commandProposals.commandHash,
      });

    return Response.json({ proposal }, { status: 201 });
  } finally {
    await client.end();
  }
}
