import { and, eq, gte, isNull, sql } from "drizzle-orm";
import type { NextRequest } from "next/server";
import { createDb } from "@/db/client";
import {
  accounts,
  auditEvents,
  projectEvents,
  projects,
  subscriptions,
} from "@/db/schema";
import {
  checkRateLimit,
  checkRequestSize,
  checkSameOrigin,
  jsonError,
  requireVerifiedBrowserSession,
} from "@/lib/api/guards";
import {
  csvEscape,
  newExportId,
  writeSafeExportZip,
} from "@/lib/safe-export";
import { incrementMetric } from "@/lib/metrics";

export async function POST(request: NextRequest) {
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

  const { db, client } = createDb();
  try {
    const [account] = await db
      .select({
        id: accounts.id,
        name: accounts.name,
        createdAt: accounts.createdAt,
      })
      .from(accounts)
      .where(and(eq(accounts.ownerUserId, auth.userId), isNull(accounts.deletedAt)))
      .limit(1);
    if (!account) {
      return jsonError(404, "account_not_found", "Account not found.");
    }

    const dayStart = new Date();
    dayStart.setUTCHours(0, 0, 0, 0);
    const [{ count }] = await db
      .select({ count: sql<number>`count(*)::int` })
      .from(auditEvents)
      .where(
        and(
          eq(auditEvents.accountId, account.id),
          eq(auditEvents.type, "export_requested"),
          gte(auditEvents.createdAt, dayStart),
        ),
      );
    if (Number(count) >= 3) {
      return jsonError(
        429,
        "export_limit_reached",
        "Export limit reached. Try again later.",
      );
    }

    const [subscription] = await db
      .select({
        state: subscriptions.state,
        plan: subscriptions.plan,
        priceRub: subscriptions.priceRub,
        trialEndsAt: subscriptions.trialEndsAt,
        currentPeriodEnd: subscriptions.currentPeriodEnd,
      })
      .from(subscriptions)
      .where(eq(subscriptions.accountId, account.id))
      .limit(1);
    const projectRows = await db
      .select({
        id: projects.id,
        name: projects.name,
        state: projects.state,
        createdAt: projects.createdAt,
        deletedAt: projects.deletedAt,
      })
      .from(projects)
      .where(eq(projects.accountId, account.id));
    const eventRows = await db
      .select({
        projectId: projectEvents.projectId,
        type: projectEvents.type,
        occurredAt: projectEvents.occurredAt,
        status: projectEvents.status,
        durationMs: projectEvents.durationMs,
        toolAdapter: projectEvents.toolAdapter,
      })
      .from(projectEvents)
      .where(eq(projectEvents.accountId, account.id));

    const exportId = newExportId();
    await writeSafeExportZip(exportId, account.id, [
      {
        name: "account.json",
        body: JSON.stringify(
          {
            exported_at_utc: new Date().toISOString(),
            account: {
              id: account.id,
              email: auth.email,
              name: account.name,
              created_at: account.createdAt,
            },
            subscription,
            project_count: projectRows.length,
            event_summary_count: eventRows.length,
          },
          null,
          2,
        ),
      },
      {
        name: "projects.json",
        body: JSON.stringify({ projects: projectRows }, null, 2),
      },
      {
        name: "prompt-events.csv",
        body: [
          [
            "project_id",
            "type",
            "occurred_at_utc",
            "status",
            "duration_ms",
            "tool_adapter",
          ]
            .map(csvEscape)
            .join(","),
          ...eventRows.map((event) =>
            [
              event.projectId,
              event.type,
              event.occurredAt.toISOString(),
              event.status,
              event.durationMs,
              event.toolAdapter,
            ]
              .map(csvEscape)
              .join(","),
          ),
        ].join("\n"),
      },
    ]);

    await db.insert(auditEvents).values({
      accountId: account.id,
      userId: auth.userId,
      type: "export_requested",
      metadataJson: { export_id: exportId },
    });
    incrementMetric("account_export_created_total");

    return Response.json({
      exportId,
      downloadUrl: `/api/account/export/${exportId}`,
      expiresInSeconds: 900,
    });
  } finally {
    await client.end();
  }
}
