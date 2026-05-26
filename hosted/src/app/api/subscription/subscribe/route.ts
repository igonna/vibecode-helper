import { and, eq, isNull } from "drizzle-orm";
import type { NextRequest } from "next/server";
import { accounts, billingEvents, subscriptions } from "@/db/schema";
import {
  checkRateLimit,
  checkRequestSize,
  checkSameOrigin,
  jsonError,
  requireVerifiedBrowserSession,
} from "@/lib/api/guards";
import { getServerEnv } from "@/lib/env";
import { createDb } from "@/db/client";
import { incrementMetric, logInfo } from "@/lib/metrics";
import {
  createSubscriptionPayment,
  yookassaConfigured,
} from "@/lib/yookassa";

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

  if (!yookassaConfigured()) {
    return jsonError(
      503,
      "billing_not_configured",
      "Billing is not configured yet.",
    );
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

    const env = getServerEnv();
    const payment = await createSubscriptionPayment({
      accountId: account.id,
      returnUrl: `${env.HOSTED_PUBLIC_URL}/projects?billing=return`,
    });

    if (!payment.confirmation?.confirmation_url) {
      return jsonError(
        502,
        "billing_confirmation_missing",
        "Payment provider did not return a confirmation URL.",
      );
    }

    await db.insert(billingEvents).values({
      accountId: account.id,
      provider: "yookassa",
      type: "subscription_payment_created",
      payloadJson: {
        payment_id: payment.id,
        status: payment.status,
      },
    });

    await db
      .update(subscriptions)
      .set({
        yookassaLastPaymentId: payment.id,
        updatedAt: new Date(),
      })
      .where(eq(subscriptions.accountId, account.id));

    incrementMetric("billing_checkout_created_total");
    logInfo("billing_checkout_created", { account_id: account.id });

    return Response.json({
      checkoutUrl: payment.confirmation.confirmation_url,
      paymentId: payment.id,
    });
  } catch (error) {
    if (error instanceof Error && error.message === "yookassa_not_configured") {
      return jsonError(
        503,
        "billing_not_configured",
        "Billing is not configured yet.",
      );
    }
    throw error;
  } finally {
    await client.end();
  }
}
