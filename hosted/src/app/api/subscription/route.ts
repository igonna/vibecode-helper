import type { NextRequest } from "next/server";
import { and, eq, isNull } from "drizzle-orm";
import { createDb } from "@/db/client";
import { accounts, subscriptions } from "@/db/schema";
import {
  checkRateLimit,
  checkRequestSize,
  jsonError,
  requireVerifiedBrowserSession,
} from "@/lib/api/guards";
import { effectiveSubscriptionState } from "@/lib/subscription";

export async function GET(request: NextRequest) {
  const guard = checkRequestSize(request) ?? checkRateLimit(request, "browser");
  if (guard) {
    return guard;
  }
  const { response, auth } = await requireVerifiedBrowserSession(request);
  if (response) {
    return response;
  }

  const { db, client } = createDb();
  try {
    const [subscription] = await db
      .select({
        state: subscriptions.state,
        plan: subscriptions.plan,
        priceRub: subscriptions.priceRub,
        trialEndsAt: subscriptions.trialEndsAt,
        currentPeriodEnd: subscriptions.currentPeriodEnd,
      })
      .from(subscriptions)
      .innerJoin(accounts, eq(subscriptions.accountId, accounts.id))
      .where(and(eq(accounts.ownerUserId, auth.userId), isNull(accounts.deletedAt)))
      .limit(1);

    if (!subscription) {
      return jsonError(404, "subscription_not_found", "Subscription not found.");
    }

    return Response.json({
      subscription: {
        ...subscription,
        effectiveState: effectiveSubscriptionState(subscription),
      },
    });
  } finally {
    await client.end();
  }
}
