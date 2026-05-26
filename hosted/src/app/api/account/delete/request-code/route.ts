import { and, eq, isNull } from "drizzle-orm";
import type { NextRequest } from "next/server";
import { createDb } from "@/db/client";
import { emailLoginCodes } from "@/db/schema";
import {
  checkRateLimit,
  checkRequestSize,
  checkSameOrigin,
  requireVerifiedBrowserSession,
} from "@/lib/api/guards";
import {
  accountDeleteCodePurpose,
  generateLoginCode,
  hashSecret,
} from "@/lib/auth";
import { sendLoginCode } from "@/lib/email";
import { incrementMetric } from "@/lib/metrics";

export async function POST(request: NextRequest) {
  const guard =
    checkRequestSize(request) ??
    checkRateLimit(request, "auth_request_code") ??
    checkSameOrigin(request);
  if (guard) {
    return guard;
  }

  const { response, auth } = await requireVerifiedBrowserSession(request);
  if (response) {
    return response;
  }

  const code = generateLoginCode();
  const { db, client } = createDb();
  try {
    await db
      .update(emailLoginCodes)
      .set({ consumedAt: new Date() })
      .where(
        and(
          eq(emailLoginCodes.email, auth.email),
          eq(emailLoginCodes.purpose, accountDeleteCodePurpose),
          isNull(emailLoginCodes.consumedAt),
        ),
      );

    await db.insert(emailLoginCodes).values({
      email: auth.email,
      codeHash: hashSecret(code),
      purpose: accountDeleteCodePurpose,
      expiresAt: new Date(Date.now() + 10 * 60 * 1000),
    });

    await sendLoginCode({ email: auth.email, code });
    incrementMetric("account_delete_code_requested_total");

    return Response.json({
      ok: true,
      devCode: process.env.NODE_ENV === "production" ? undefined : code,
    });
  } finally {
    await client.end();
  }
}
