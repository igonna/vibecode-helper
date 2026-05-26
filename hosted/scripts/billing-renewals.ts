import { and, eq, inArray, isNotNull, lte } from "drizzle-orm";
import { createDb } from "../src/db/client";
import { billingEvents, subscriptions } from "../src/db/schema";
import { createRenewalPayment, yookassaConfigured } from "../src/lib/yookassa";

if (!yookassaConfigured()) {
  console.log("billing renewals skipped: YooKassa is not configured");
  process.exit(0);
}

const { db, client } = createDb();

try {
  const now = new Date();
  const dueSubscriptions = await db
    .select({
      accountId: subscriptions.accountId,
      paymentMethodId: subscriptions.yookassaPaymentMethodId,
      currentPeriodEnd: subscriptions.currentPeriodEnd,
    })
    .from(subscriptions)
    .where(
      and(
        inArray(subscriptions.state, ["active", "grace", "past_due"]),
        isNotNull(subscriptions.yookassaPaymentMethodId),
        lte(subscriptions.currentPeriodEnd, now),
      ),
    )
    .limit(500);

  for (const subscription of dueSubscriptions) {
    if (!subscription.paymentMethodId) {
      continue;
    }
    try {
      const payment = await createRenewalPayment({
        accountId: subscription.accountId,
        paymentMethodId: subscription.paymentMethodId,
        idempotenceKey: `renewal:${subscription.accountId}:${subscription.currentPeriodEnd?.toISOString() ?? "missing"}`,
      });

      if (payment.status === "succeeded") {
        await db
          .update(subscriptions)
          .set({
            state: "active",
            currentPeriodEnd: addMonths(now, 1),
            yookassaLastPaymentId: payment.id,
            updatedAt: now,
          })
          .where(eq(subscriptions.accountId, subscription.accountId));
      } else {
        await db
          .update(subscriptions)
          .set({
            state: "grace",
            currentPeriodEnd: addDays(now, 3),
            yookassaLastPaymentId: payment.id,
            updatedAt: now,
          })
          .where(eq(subscriptions.accountId, subscription.accountId));
      }

      await db.insert(billingEvents).values({
        accountId: subscription.accountId,
        provider: "yookassa",
        type: "renewal_created",
        payloadJson: {
          payment_id: payment.id,
          status: payment.status,
        },
      });
    } catch (error) {
      await db
        .update(subscriptions)
        .set({
          state: "grace",
          currentPeriodEnd: addDays(now, 3),
          updatedAt: now,
        })
        .where(eq(subscriptions.accountId, subscription.accountId));

      await db.insert(billingEvents).values({
        accountId: subscription.accountId,
        provider: "yookassa",
        type: "renewal_failed",
        payloadJson: {
          reason: error instanceof Error ? error.message : "unknown",
        },
      });
    }
  }

  console.log(`processed ${dueSubscriptions.length} renewal candidates`);
} finally {
  await client.end();
}

function addDays(date: Date, days: number) {
  return new Date(date.getTime() + days * 24 * 60 * 60 * 1000);
}

function addMonths(date: Date, months: number) {
  const next = new Date(date);
  next.setUTCMonth(next.getUTCMonth() + months);
  return next;
}
