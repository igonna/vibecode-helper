import { randomUUID } from "node:crypto";
import { getServerEnv } from "@/lib/env";

export type YookassaPayment = {
  id: string;
  status: "pending" | "waiting_for_capture" | "succeeded" | "canceled";
  paid?: boolean;
  amount?: {
    value?: string;
    currency?: string;
  };
  confirmation?: {
    confirmation_url?: string;
  };
  metadata?: {
    account_id?: string;
  };
  payment_method?: {
    id?: string;
    saved?: boolean;
  };
};

export function yookassaConfigured() {
  const env = getServerEnv();
  return Boolean(env.YOOKASSA_SHOP_ID && env.YOOKASSA_SECRET_KEY);
}

export async function createSubscriptionPayment(input: {
  accountId: string;
  returnUrl: string;
  idempotenceKey?: string;
}) {
  const env = getServerEnv();
  const response = await yookassaFetch("/v3/payments", {
    method: "POST",
    idempotenceKey: input.idempotenceKey ?? randomUUID(),
    body: {
      amount: {
        value: "399.00",
        currency: "RUB",
      },
      capture: true,
      confirmation: {
        type: "redirect",
        return_url: input.returnUrl,
      },
      description: "Vibeengine monthly subscription",
      save_payment_method: true,
      metadata: {
        account_id: input.accountId,
      },
    },
    apiBaseUrl: env.YOOKASSA_API_URL,
  });

  return response as YookassaPayment;
}

export async function createRenewalPayment(input: {
  accountId: string;
  paymentMethodId: string;
  idempotenceKey?: string;
}) {
  const env = getServerEnv();
  const response = await yookassaFetch("/v3/payments", {
    method: "POST",
    idempotenceKey: input.idempotenceKey ?? randomUUID(),
    body: {
      amount: {
        value: "399.00",
        currency: "RUB",
      },
      capture: true,
      payment_method_id: input.paymentMethodId,
      description: "Vibeengine monthly subscription renewal",
      metadata: {
        account_id: input.accountId,
      },
    },
    apiBaseUrl: env.YOOKASSA_API_URL,
  });

  return response as YookassaPayment;
}

export async function getYookassaPayment(paymentId: string) {
  const env = getServerEnv();
  const response = await yookassaFetch(`/v3/payments/${paymentId}`, {
    method: "GET",
    apiBaseUrl: env.YOOKASSA_API_URL,
  });

  return response as YookassaPayment;
}

async function yookassaFetch(
  path: string,
  input: {
    method: "GET" | "POST";
    apiBaseUrl: string;
    idempotenceKey?: string;
    body?: unknown;
  },
) {
  const env = getServerEnv();
  if (!env.YOOKASSA_SHOP_ID || !env.YOOKASSA_SECRET_KEY) {
    throw new Error("yookassa_not_configured");
  }

  const headers: Record<string, string> = {
    authorization: `Basic ${Buffer.from(
      `${env.YOOKASSA_SHOP_ID}:${env.YOOKASSA_SECRET_KEY}`,
    ).toString("base64")}`,
  };
  if (input.body !== undefined) {
    headers["content-type"] = "application/json";
  }
  if (input.idempotenceKey) {
    headers["idempotence-key"] = input.idempotenceKey;
  }

  const response = await fetch(`${input.apiBaseUrl}${path}`, {
    method: input.method,
    headers,
    body: input.body === undefined ? undefined : JSON.stringify(input.body),
  });

  const body = await response.json().catch(() => null);
  if (!response.ok) {
    throw new Error(`yookassa_http_${response.status}`);
  }
  return body;
}
