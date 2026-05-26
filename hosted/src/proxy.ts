import { NextResponse, type NextRequest } from "next/server";

const SESSION_COOKIE = "vibeengine_session";

export function proxy(request: NextRequest) {
  const response = handleRoute(request);
  response.headers.set("x-content-type-options", "nosniff");
  response.headers.set("x-frame-options", "DENY");
  response.headers.set("referrer-policy", "same-origin");
  response.headers.set(
    "permissions-policy",
    "camera=(), microphone=(), geolocation=()",
  );
  return response;
}

function handleRoute(request: NextRequest) {
  const hasSession = request.cookies.has(SESSION_COOKIE);

  if (request.nextUrl.pathname.startsWith("/projects") && !hasSession) {
    const loginUrl = new URL("/login", request.url);
    loginUrl.searchParams.set("next", request.nextUrl.pathname);
    return NextResponse.redirect(loginUrl);
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
