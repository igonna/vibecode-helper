import { NextResponse, type NextRequest } from "next/server";
import { checkRateLimit, checkSameOrigin } from "@/lib/api/guards";

export function POST(request: NextRequest) {
  const blocked = checkRateLimit(request, "auth_logout") ?? checkSameOrigin(request);
  if (blocked) {
    return blocked;
  }

  const response = NextResponse.json({ ok: true });
  response.cookies.delete("vibeengine_session");
  return response;
}
