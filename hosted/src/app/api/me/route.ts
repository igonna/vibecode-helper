import type { NextRequest } from "next/server";
import { requireVerifiedBrowserSession } from "@/lib/api/guards";

export async function GET(request: NextRequest) {
  const { response, auth } = await requireVerifiedBrowserSession(request);
  if (response) {
    return response;
  }

  return Response.json({
    user: {
      id: auth?.userId,
      email: auth?.email,
    },
  });
}
