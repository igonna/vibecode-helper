import { and, eq, gte, isNull, lt, sql } from "drizzle-orm";
import type { NextRequest } from "next/server";
import { z } from "zod";
import { createDb } from "@/db/client";
import { accounts, projectEvents, projects } from "@/db/schema";
import {
  checkRateLimit,
  checkRequestSize,
  checkSameOrigin,
  jsonError,
  requireVerifiedBrowserSession,
} from "@/lib/api/guards";
import { incrementMetric } from "@/lib/metrics";

const createProjectSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1)
    .max(64)
    .regex(/^[a-zA-Z0-9][a-zA-Z0-9_-]*$/),
});

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

export async function GET(request: NextRequest) {
  const guard = checkRequestSize(request) ?? checkRateLimit(request, "browser");
  if (guard) {
    return guard;
  }
  const { response, auth } = await requireVerifiedBrowserSession(request);
  if (response) {
    return response;
  }

  const accountId = await getAccountId(auth.userId);
  if (!accountId) {
    return jsonError(404, "account_not_found", "Account not found.");
  }

  const { db, client } = createDb();
  try {
    const rows = await db
      .select({
        id: projects.id,
        name: projects.name,
        state: projects.state,
        lastStatus: projects.lastStatus,
        lastSeenAt: projects.lastSeenAt,
        createdAt: projects.createdAt,
      })
      .from(projects)
      .where(and(eq(projects.accountId, accountId), isNull(projects.deletedAt)))
      .orderBy(projects.createdAt);

    const timezone = safeTimeZone(request.nextUrl.searchParams.get("timezone"));
    const today = utcDayRangeForTimeZone(timezone, new Date());

    const [stats] = await db
      .select({
        prompts: sql<number>`count(*) filter (where ${projectEvents.type} = 'prompt_started')::int`,
        successes: sql<number>`count(*) filter (where ${projectEvents.type} = 'prompt_finished')::int`,
        failures: sql<number>`count(*) filter (where ${projectEvents.type} = 'prompt_failed')::int`,
        attention: sql<number>`count(*) filter (where ${projectEvents.type} = 'permission_requested')::int`,
        promptTimeMs: sql<number>`coalesce(sum(${projectEvents.durationMs}), 0)::int`,
      })
      .from(projectEvents)
      .where(
        and(
          eq(projectEvents.accountId, accountId),
          gte(projectEvents.occurredAt, today.start),
          lt(projectEvents.occurredAt, today.end),
        ),
      );

    return Response.json({
      projects: rows.map((project) => ({
        ...project,
        displayStatus: displayProjectStatus(project.lastStatus, project.lastSeenAt),
      })),
      stats: {
        prompts: stats?.prompts ?? 0,
        successes: stats?.successes ?? 0,
        failures: stats?.failures ?? 0,
        attention: stats?.attention ?? 0,
        promptTimeMs: stats?.promptTimeMs ?? 0,
      },
    });
  } finally {
    await client.end();
  }
}

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

  const body = createProjectSchema.safeParse(await request.json().catch(() => null));
  if (!body.success) {
    return jsonError(
      400,
      "invalid_project_name",
      "Project name must be 1-64 letters, numbers, dashes, or underscores.",
    );
  }

  const accountId = await getAccountId(auth.userId);
  if (!accountId) {
    return jsonError(404, "account_not_found", "Account not found.");
  }

  const { db, client } = createDb();
  try {
    const dayStart = new Date();
    dayStart.setUTCHours(0, 0, 0, 0);
    const [{ count }] = await db
      .select({ count: sql<number>`count(*)::int` })
      .from(projects)
      .where(and(eq(projects.accountId, accountId), gte(projects.createdAt, dayStart)));
    if (Number(count) >= 50) {
      return jsonError(
        429,
        "project_limit_reached",
        "Project creation limit reached. Try again later.",
      );
    }

    const [project] = await db
      .insert(projects)
      .values({
        accountId,
        name: body.data.name,
        state: "active",
        lastStatus: "Setup",
      })
      .returning({
        id: projects.id,
        name: projects.name,
        state: projects.state,
        lastStatus: projects.lastStatus,
        createdAt: projects.createdAt,
      });
    incrementMetric("project_created_total");

    return Response.json({ project }, { status: 201 });
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

function displayProjectStatus(lastStatus: string | null, lastSeenAt: Date | null) {
  if (!lastSeenAt) {
    return "Setup";
  }
  if (lastStatus === "Prompting") {
    if (Date.now() - lastSeenAt.getTime() > 2 * 60 * 60 * 1000) {
      return "Attention";
    }
    return "Prompting";
  }
  if (Date.now() - lastSeenAt.getTime() > 10 * 60 * 1000) {
    return "Offline";
  }
  switch (lastStatus) {
    case "Ready":
    case "Running":
    case "Paused":
    case "Attention":
      return lastStatus;
    default:
      return "Ready";
  }
}

function safeTimeZone(value: string | null) {
  if (!value) {
    return "UTC";
  }
  try {
    new Intl.DateTimeFormat("en-US", { timeZone: value }).format(new Date());
    return value;
  } catch {
    return "UTC";
  }
}

function utcDayRangeForTimeZone(timezone: string, now: Date) {
  const parts = localDateParts(timezone, now);
  const nextDay = new Date(Date.UTC(parts.year, parts.month - 1, parts.day + 1));
  return {
    start: zonedTimeToUtc(timezone, parts.year, parts.month, parts.day),
    end: zonedTimeToUtc(
      timezone,
      nextDay.getUTCFullYear(),
      nextDay.getUTCMonth() + 1,
      nextDay.getUTCDate(),
    ),
  };
}

function zonedTimeToUtc(
  timezone: string,
  year: number,
  month: number,
  day: number,
) {
  let candidate = new Date(Date.UTC(year, month - 1, day));
  const target = Date.UTC(year, month - 1, day);

  for (let index = 0; index < 3; index += 1) {
    const parts = localDateTimeParts(timezone, candidate);
    const actual = Date.UTC(
      parts.year,
      parts.month - 1,
      parts.day,
      parts.hour,
      parts.minute,
      parts.second,
    );
    candidate = new Date(candidate.getTime() + target - actual);
  }

  return candidate;
}

function localDateParts(timezone: string, date: Date) {
  const parts = localDateTimeParts(timezone, date);
  return { year: parts.year, month: parts.month, day: parts.day };
}

function localDateTimeParts(timezone: string, date: Date) {
  const formatter = new Intl.DateTimeFormat("en-US", {
    timeZone: timezone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
  });
  const values = Object.fromEntries(
    formatter.formatToParts(date).map((part) => [part.type, part.value]),
  );
  return {
    year: Number(values.year),
    month: Number(values.month),
    day: Number(values.day),
    hour: Number(values.hour),
    minute: Number(values.minute),
    second: Number(values.second),
  };
}
