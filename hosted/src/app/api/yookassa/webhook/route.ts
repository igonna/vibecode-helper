import { eq } from "drizzle-orm";
import type { NextRequest } from "next/server";
import { z } from "zod";
import { createDb } from "@/db/client";
import { billingEvents, subscriptions, webhookEvents } from "@/db/schema";
import {
  checkRateLimit,
  checkRequestSize,
  jsonError,
} from "@/lib/api/guards";
import { incrementMetric, logInfo } from "@/lib/metrics";
import { getYookassaPayment, yookassaConfigured } from "@/lib/yookassa";

const webhookSchema = z.object({
  type: z.literal("notification"),
  event: z.enum(["payment.succeeded", "payment.canceled"]),
  object: z.object({
    id: z.string().min(1),
    status: z.enum(["succeeded", "canceled"]),
    paid: z.boolean().optional(),
    amount: z
      .object({
        value: z.string().optional(),
        currency: z.string().optional(),
      })
      .optional(),
    metadata: z
      .object({
        account_id: z.string().uuid().optional(),
      })
      .optional(),
    payment_method: z
      .object({
        id: z.string().optional(),
        saved: z.boolean().optional(),
      })
      .optional(),
  }),
});

export async function POST(request: NextRequest) {
  const guard =
    checkRequestSize(request) ?? checkRateLimit(request, "yookassa_webhook");
  if (guard) {
    return guard;
  }

  if (!yookassaConfigured()) {
    return jsonError(
      503,
      "billing_not_configured",
      "Billing is not configured yet.",
    );
  }

  const body = webhookSchema.safeParse(await request.json().catch(() => null));
  if (!body.success) {
    incrementMetric("yookassa_webhook_rejected_total");
    return jsonError(400, "invalid_webhook", "Webhook payload is invalid.");
  }

  const currentPayment = await getYookassaPayment(body.data.object.id);
  if (
    currentPayment.id !== body.data.object.id ||
    currentPayment.status !== body.data.object.status
  ) {
    incrementMetric("yookassa_webhook_rejected_total");
    return jsonError(
      400,
      "webhook_not_authentic",
      "Payment status could not be verified.",
    );
  }

  const providerEventId = `${body.data.event}:${body.data.object.id}:${body.data.object.status}`;
  const accountId =
    currentPayment.metadata?.account_id ?? body.data.object.metadata?.account_id;

  const { db, client } = createDb();
  try {
    await db.transaction(async (tx) => {
      const inserted = await tx
        .insert(webhookEvents)
        .values({
          provider: "yookassa",
          providerEventId,
          payloadJson: {
            event: body.data.event,
            payment_id: body.data.object.id,
            status: body.data.object.status,
            amount: body.data.object.amount,
          },
          processedAt: new Date(),
        })
        .onConflictDoNothing()
        .returning({ id: webhookEvents.id });

      if (inserted.length === 0) {
        return;
      }

      if (!accountId) {
        await tx.insert(billingEvents).values({
          provider: "yookassa",
          type: "webhook_missing_account",
          payloadJson: {
            payment_id: body.data.object.id,
            event: body.data.event,
          },
        });
        return;
      }

      if (body.data.event === "payment.succeeded") {
        const paymentMethodId =
          currentPayment.payment_method?.saved === true
            ? currentPayment.payment_method.id
            : body.data.object.payment_method?.id;
        await tx
          .update(subscriptions)
          .set({
            state: "active",
            currentPeriodEnd: addMonths(new Date(), 1),
            yookassaPaymentMethodId: paymentMethodId ?? null,
            yookassaLastPaymentId: body.data.object.id,
            updatedAt: new Date(),
          })
          .where(eq(subscriptions.accountId, accountId));

        await tx.insert(billingEvents).values({
          accountId,
          provider: "yookassa",
          type: "payment_succeeded",
          payloadJson: {
            payment_id: body.data.object.id,
            amount: body.data.object.amount,
            saved_payment_method: Boolean(paymentMethodId),
          },
        });
      }

      if (body.data.event === "payment.canceled") {
        await tx
          .update(subscriptions)
          .set({
            state: "past_due",
            yookassaLastPaymentId: body.data.object.id,
            updatedAt: new Date(),
          })
          .where(eq(subscriptions.accountId, accountId));

        await tx.insert(billingEvents).values({
          accountId,
          provider: "yookassa",
          type: "payment_canceled",
          payloadJson: {
            payment_id: body.data.object.id,
          },
        });
      }
    });

    incrementMetric("yookassa_webhook_processed_total");
    logInfo("yookassa_webhook_processed", {
      event: body.data.event,
      payment_id: body.data.object.id,
    });

    return Response.json({ ok: true });
  } finally {
    await client.end();
  }
}

function addMonths(date: Date, months: number) {
  const next = new Date(date);
  next.setUTCMonth(next.getUTCMonth() + months);
  return next;
}
