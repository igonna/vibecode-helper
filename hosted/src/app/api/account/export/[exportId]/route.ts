import type { NextRequest } from "next/server";
import { and, eq, isNull } from "drizzle-orm";
import { createDb } from "@/db/client";
import { accounts } from "@/db/schema";
import {
  checkRateLimit,
  checkRequestSize,
  jsonError,
  requireVerifiedBrowserSession,
} from "@/lib/api/guards";
import { readSafeExportZip } from "@/lib/safe-export";

type ExportContext = {
  params: Promise<{ exportId: string }>;
};

export async function GET(request: NextRequest, context: ExportContext) {
  const guard = checkRequestSize(request) ?? checkRateLimit(request, "browser");
  if (guard) {
    return guard;
  }
  const { response, auth } = await requireVerifiedBrowserSession(request);
  if (response) {
    return response;
  }

  const { exportId } = await context.params;
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

    const body = await readSafeExportZip(exportId, account.id);
  if (!body) {
    return jsonError(404, "export_not_found", "Export is expired or missing.");
  }

    return new Response(body, {
      headers: {
        "content-type": "application/zip",
        "content-disposition": `attachment; filename="vibeengine-export-${new Date()
          .toISOString()
          .slice(0, 10)}.zip"`,
        "cache-control": "no-store",
      },
    });
  } finally {
    await client.end();
  }
}
