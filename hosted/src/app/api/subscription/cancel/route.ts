import { and, eq, isNull } from "drizzle-orm";
import type { NextRequest } from "next/server";
import { createDb } from "@/db/client";
import { accounts, billingEvents, subscriptions } from "@/db/schema";
import {
  checkRateLimit,
  checkRequestSize,
  checkSameOrigin,
  jsonError,
  requireVerifiedBrowserSession,
} from "@/lib/api/guards";
import { incrementMetric, logInfo } from "@/lib/metrics";

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
      .select({ id: accounts.id })
      .from(accounts)
      .where(and(eq(accounts.ownerUserId, auth.userId), isNull(accounts.deletedAt)))
      .limit(1);

    if (!account) {
      return jsonError(404, "account_not_found", "Account not found.");
    }

    const [subscription] = await db
      .update(subscriptions)
      .set({ state: "canceled", updatedAt: new Date() })
      .where(eq(subscriptions.accountId, account.id))
      .returning({
        state: subscriptions.state,
        currentPeriodEnd: subscriptions.currentPeriodEnd,
      });

    await db.insert(billingEvents).values({
      accountId: account.id,
      provider: "yookassa",
      type: "subscription_canceled",
      payloadJson: {
        effective_until: subscription?.currentPeriodEnd?.toISOString() ?? null,
      },
    });

    incrementMetric("billing_subscription_canceled_total");
    logInfo("billing_subscription_canceled", { account_id: account.id });

    return Response.json({ subscription });
  } finally {
    await client.end();
  }
}
