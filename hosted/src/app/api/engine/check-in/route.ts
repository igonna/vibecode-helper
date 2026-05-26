import type { NextRequest } from "next/server";
import { and, asc, eq, inArray, isNull, sql } from "drizzle-orm";
import { z } from "zod";
import { createDb } from "@/db/client";
import {
  dailyProjectStats,
  commandProposals,
  projectEvents,
  projects,
  remoteActions,
  subscriptions,
  users,
} from "@/db/schema";
import {
  checkRateLimit,
  checkRequestSize,
  jsonError,
} from "@/lib/api/guards";
import { verifyEngineAccessToken } from "@/lib/engine-auth";
import { containsForbiddenEventPayload } from "@/lib/event-privacy";
import { incrementMetric, logInfo } from "@/lib/metrics";
import {
  cloudEntitlementActive,
  effectiveSubscriptionState,
} from "@/lib/subscription";

const eventSchema = z.object({
  localEventId: z.string().min(1).max(128),
  projectId: z.string().uuid(),
  sessionId: z.string().max(128).nullable().optional(),
  promptInstanceId: z.string().max(128).nullable().optional(),
  toolAdapter: z.string().min(1).max(64),
  nativeEventName: z.string().max(128).nullable().optional(),
  nativeEventId: z.string().max(256).nullable().optional(),
  type: z.string().min(1).max(64),
  payloadJson: z.unknown().optional(),
  createdAtUtc: z.string().datetime(),
});

const checkInSchema = z.object({
  events: z.array(eventSchema).max(500).default([]),
  projectStatuses: z
    .array(
      z.object({
        projectId: z.string().uuid(),
        status: z.string().max(64),
      }),
    )
    .max(200)
    .default([]),
  lastAppliedRemoteActionId: z.string().max(128).optional(),
  proposalResults: z
    .array(
      z.object({
        proposalId: z.string().uuid(),
        state: z.enum(["applied", "rejected", "failed"]),
      }),
    )
    .max(100)
    .default([]),
});

export async function POST(request: NextRequest) {
  const guard = checkRequestSize(request) ?? checkRateLimit(request, "engine");
  if (guard) {
    incrementMetric("engine_check_in_rejected_total");
    return guard;
  }

  const authorization = request.headers.get("authorization");
  if (!authorization?.startsWith("Bearer ")) {
    incrementMetric("engine_check_in_rejected_total");
    return jsonError(
      401,
      "engine_bearer_required",
      "Use an engine bearer token for this route.",
    );
  }

  const access = verifyEngineAccessToken(authorization.slice("Bearer ".length));
  if (!access) {
    incrementMetric("engine_check_in_rejected_total");
    return jsonError(401, "engine_signed_out", "Install is signed out.");
  }

  const body = checkInSchema.safeParse(await request.json().catch(() => null));
  if (!body.success) {
    incrementMetric("engine_check_in_rejected_total");
    return jsonError(400, "invalid_check_in", "Check-in payload is invalid.");
  }

  for (const event of body.data.events) {
    if (containsForbiddenEventPayload(event.payloadJson)) {
      incrementMetric("engine_check_in_rejected_total");
      return jsonError(
        400,
        "forbidden_event_payload",
        "Event payload contains private fields.",
      );
    }
  }

  const { db, client } = createDb();
  try {
    const [user] = await db
      .select({ engineAuthEpoch: users.engineAuthEpoch })
      .from(users)
      .where(and(eq(users.id, access.userId), isNull(users.deletedAt)))
      .limit(1);

    if (!user || user.engineAuthEpoch !== access.epoch) {
      incrementMetric("engine_check_in_rejected_total");
      return jsonError(401, "engine_signed_out", "Install is signed out.");
    }

    const [subscription] = await db
      .select({
        state: subscriptions.state,
        trialEndsAt: subscriptions.trialEndsAt,
        currentPeriodEnd: subscriptions.currentPeriodEnd,
      })
      .from(subscriptions)
      .where(eq(subscriptions.accountId, access.accountId))
      .limit(1);

    if (!cloudEntitlementActive(subscription)) {
      incrementMetric("engine_check_in_rejected_total");
      return jsonError(
        402,
        "entitlement_inactive",
        "Local capture continues. Cloud upload and dashboard updates require an active subscription.",
      );
    }

    incrementMetric("engine_check_in_total");
    incrementMetric("engine_check_in_events_received_total", body.data.events.length);
    const acceptedEventIds: string[] = [];
    const now = new Date();

    await db.transaction(async (tx) => {
      if (body.data.lastAppliedRemoteActionId) {
        await tx
          .update(remoteActions)
          .set({
            state: "applied",
            finishedAt: now,
            resultMessage: "applied locally",
          })
          .where(
            and(
              eq(remoteActions.id, body.data.lastAppliedRemoteActionId),
              eq(remoteActions.accountId, access.accountId),
            ),
          );
      }

      for (const result of body.data.proposalResults) {
        await tx
          .update(commandProposals)
          .set({
            state:
              result.state === "applied"
                ? "applied"
                : result.state === "rejected"
                  ? "rejected"
                  : "failed",
            decidedAt: now,
            appliedAt: result.state === "applied" ? now : null,
          })
          .where(
            and(
              eq(commandProposals.id, result.proposalId),
              eq(commandProposals.accountId, access.accountId),
            ),
          );
      }

      for (const event of body.data.events) {
        const [project] = await tx
          .select({ id: projects.id })
          .from(projects)
          .where(
            and(
              eq(projects.id, event.projectId),
              eq(projects.accountId, access.accountId),
            ),
          )
          .limit(1);
        if (!project) {
          continue;
        }

        const inserted = await tx
          .insert(projectEvents)
          .values({
            accountId: access.accountId,
            projectId: event.projectId,
            localEventId: event.localEventId,
            toolAdapter: event.toolAdapter,
            nativeEventName: event.nativeEventName ?? null,
            nativeEventId: event.nativeEventId ?? null,
            type: event.type,
            occurredAt: new Date(event.createdAtUtc),
            status: statusFromEventType(event.type),
          })
          .onConflictDoNothing()
          .returning({ id: projectEvents.id });

        if (
          inserted.length > 0 &&
          (event.type === "prompt_started" ||
            event.type === "prompt_finished" ||
            event.type === "prompt_failed")
        ) {
          const occurredAt = new Date(event.createdAtUtc);
          await tx
            .insert(dailyProjectStats)
            .values({
              accountId: access.accountId,
              projectId: event.projectId,
              dayKey: occurredAt.toISOString().slice(0, 10),
              timezone: "UTC",
              promptCount: event.type === "prompt_started" ? 1 : 0,
              successCount: event.type === "prompt_finished" ? 1 : 0,
              failureCount: event.type === "prompt_failed" ? 1 : 0,
              promptDurationMs: 0,
              updatedAt: now,
            })
            .onConflictDoUpdate({
              target: [
                dailyProjectStats.accountId,
                dailyProjectStats.projectId,
                dailyProjectStats.dayKey,
                dailyProjectStats.timezone,
              ],
              set: {
                promptCount:
                  event.type === "prompt_started"
                    ? sql`${dailyProjectStats.promptCount} + 1`
                    : sql`${dailyProjectStats.promptCount}`,
                successCount:
                  event.type === "prompt_finished"
                    ? sql`${dailyProjectStats.successCount} + 1`
                    : sql`${dailyProjectStats.successCount}`,
                failureCount:
                  event.type === "prompt_failed"
                    ? sql`${dailyProjectStats.failureCount} + 1`
                    : sql`${dailyProjectStats.failureCount}`,
                updatedAt: now,
              },
            });
        }

        acceptedEventIds.push(event.localEventId);
      }

      for (const status of body.data.projectStatuses) {
        await tx
          .update(projects)
          .set({ lastSeenAt: now, lastStatus: status.status })
          .where(
            and(
              eq(projects.id, status.projectId),
              eq(projects.accountId, access.accountId),
              isNull(projects.deletedAt),
            ),
          );
      }
    });

    const pendingActions = await db
      .select({
        id: remoteActions.id,
        projectId: remoteActions.projectId,
        type: remoteActions.type,
        targetCommandHash: remoteActions.targetCommandHash,
      })
      .from(remoteActions)
      .where(
        and(
          eq(remoteActions.accountId, access.accountId),
          inArray(remoteActions.state, ["pending", "sent"]),
        ),
      )
      .orderBy(asc(remoteActions.createdAt))
      .limit(50);

    if (pendingActions.length > 0) {
      await db
        .update(remoteActions)
        .set({ state: "sent", sentAt: now })
        .where(
          and(
            eq(remoteActions.accountId, access.accountId),
            inArray(
              remoteActions.id,
              pendingActions.map((action) => action.id),
            ),
          ),
        );
    }

    const pendingProposals = await db
      .select({
        id: commandProposals.id,
        projectId: commandProposals.projectId,
        type: commandProposals.type,
        commandHash: commandProposals.commandHash,
        payloadJson: commandProposals.payloadJson,
      })
      .from(commandProposals)
      .where(
        and(
          eq(commandProposals.accountId, access.accountId),
          inArray(commandProposals.state, [
            "pending_device",
            "pending_local_approval",
          ]),
        ),
      )
      .orderBy(asc(commandProposals.createdAt))
      .limit(50);

    if (pendingProposals.length > 0) {
      await db
        .update(commandProposals)
        .set({ state: "pending_local_approval", seenAt: now })
        .where(
          and(
            eq(commandProposals.accountId, access.accountId),
            inArray(
              commandProposals.id,
              pendingProposals.map((proposal) => proposal.id),
            ),
          ),
        );
    }

    incrementMetric("engine_check_in_events_accepted_total", acceptedEventIds.length);
    logInfo("engine_check_in", {
      account_id: access.accountId,
      event_count: body.data.events.length,
      accepted_count: acceptedEventIds.length,
    });

    return Response.json({
      acceptedEventIds,
      entitlement: { state: effectiveSubscriptionState(subscription) },
      remoteActions: pendingActions,
      commandProposals: pendingProposals,
      lastAppliedRemoteActionId: body.data.lastAppliedRemoteActionId ?? null,
      update: null,
    });
  } finally {
    await client.end();
  }
}

function statusFromEventType(type: string) {
  switch (type) {
    case "prompt_started":
      return "prompting";
    case "prompt_finished":
      return "success";
    case "prompt_failed":
      return "failed";
    case "permission_requested":
      return "attention";
    default:
      return null;
  }
}
