import { NextResponse, type NextRequest } from "next/server";
import { getBrowserAuth } from "@/lib/auth";
import { getServerEnv } from "@/lib/env";
import { incrementMetric } from "@/lib/metrics";

const sessionCookie = "vibeengine_session";
const maxJsonBodyBytes = 1_000_000;
const rateLimitWindowMs = 60_000;

const rateLimits = new Map<string, { count: number; resetAt: number }>();

const bucketLimits: Record<string, number> = {
  auth_request_code: 5,
  auth_verify_code: 20,
  yookassa_webhook: 120,
  engine: 60,
  browser: 120,
};

type ErrorBody = {
  error: {
    code: string;
    message: string;
  };
};

export function jsonError(
  status: number,
  code: string,
  message: string,
): NextResponse<ErrorBody> {
  return NextResponse.json({ error: { code, message } }, { status });
}

export function notImplemented(feature: string) {
  return jsonError(
    501,
    "not_implemented",
    `${feature} is not implemented yet.`,
  );
}

export function checkRequestSize(request: NextRequest) {
  const contentLength = request.headers.get("content-length");
  if (!contentLength) {
    return null;
  }

  const size = Number(contentLength);
  if (Number.isFinite(size) && size > maxJsonBodyBytes) {
    incrementMetric("request_payload_too_large_total");
    return jsonError(413, "payload_too_large", "Request body is too large.");
  }

  return null;
}

export function checkRateLimit(_request: NextRequest, _bucket: string) {
  const limit = bucketLimits[_bucket] ?? 60;
  const ip =
    _request.headers.get("x-forwarded-for")?.split(",")[0]?.trim() ??
    _request.headers.get("x-real-ip") ??
    "local";
  const key = `${_bucket}:${ip}`;
  const now = Date.now();
  const current = rateLimits.get(key);

  if (!current || current.resetAt <= now) {
    rateLimits.set(key, { count: 1, resetAt: now + rateLimitWindowMs });
    return null;
  }

  if (current.count >= limit) {
    incrementMetric("request_rate_limited_total");
    const retryAfter = Math.max(1, Math.ceil((current.resetAt - now) / 1000));
    const response = jsonError(
      429,
      "too_many_requests",
      "Too many requests. Try again later.",
    );
    response.headers.set("Retry-After", String(retryAfter));
    return response;
  }

  current.count += 1;
  return null;
}

export function checkSameOrigin(request: NextRequest) {
  if (request.method === "GET" || request.method === "HEAD") {
    return null;
  }

  const origin = request.headers.get("origin");
  if (origin && !allowedOrigins(request).has(normalizeOrigin(origin))) {
    incrementMetric("request_csrf_blocked_total");
    return jsonError(403, "cross_origin_blocked", "Cross-origin request blocked.");
  }

  return null;
}

function allowedOrigins(request: NextRequest) {
  const env = getServerEnv();
  const origins = new Set<string>();

  addOrigin(origins, request.nextUrl.origin);
  addOrigin(origins, env.HOSTED_PUBLIC_URL);

  const host = request.headers.get("host");
  if (host) {
    addOrigin(origins, `${request.nextUrl.protocol}//${host}`);
    const forwardedProto = request.headers.get("x-forwarded-proto");
    if (forwardedProto) {
      addOrigin(origins, `${forwardedProto.split(",")[0]?.trim() ?? "https"}://${host}`);
    }
  }

  if (env.NODE_ENV !== "production") {
    for (const existingOrigin of [...origins]) {
      const parsed = safeURL(existingOrigin);
      if (!parsed || !isLoopbackHost(parsed.hostname)) {
        continue;
      }
      const port = parsed.port ? `:${parsed.port}` : "";
      addOrigin(origins, `${parsed.protocol}//localhost${port}`);
      addOrigin(origins, `${parsed.protocol}//127.0.0.1${port}`);
      addOrigin(origins, `${parsed.protocol}//0.0.0.0${port}`);
    }
  }

  return origins;
}

function addOrigin(origins: Set<string>, value: string) {
  const normalized = normalizeOrigin(value);
  if (normalized) {
    origins.add(normalized);
  }
}

function normalizeOrigin(value: string) {
  const parsed = safeURL(value);
  return parsed?.origin ?? "";
}

function safeURL(value: string) {
  try {
    return new URL(value);
  } catch {
    return null;
  }
}

function isLoopbackHost(hostname: string) {
  return hostname === "localhost" || hostname === "127.0.0.1" || hostname === "0.0.0.0";
}

export function requireBrowserSession(request: NextRequest) {
  if (request.headers.get("authorization")) {
    return jsonError(
      401,
      "browser_session_required",
      "Use a browser session cookie for this route.",
    );
  }

  if (!request.cookies.has(sessionCookie)) {
    return jsonError(401, "not_authenticated", "Login required.");
  }

  return null;
}

export async function requireVerifiedBrowserSession(request: NextRequest) {
  const basicGuard = requireBrowserSession(request);
  if (basicGuard) {
    return { response: basicGuard, auth: null };
  }

  const auth = await getBrowserAuth(request);
  if (!auth) {
    return {
      response: jsonError(401, "not_authenticated", "Login required."),
      auth: null,
    };
  }

  return { response: null, auth };
}

export function requireEngineBearer(request: NextRequest) {
  const authorization = request.headers.get("authorization");
  if (!authorization?.startsWith("Bearer ")) {
    return jsonError(
      401,
      "engine_bearer_required",
      "Use an engine bearer token for this route.",
    );
  }

  return null;
}

export function browserPlaceholder(request: NextRequest, feature: string) {
  return (
    checkRequestSize(request) ??
    checkRateLimit(request, "browser") ??
    checkSameOrigin(request) ??
    requireBrowserSession(request) ??
    notImplemented(feature)
  );
}

export function enginePlaceholder(request: NextRequest, feature: string) {
  return (
    checkRequestSize(request) ??
    checkRateLimit(request, "engine") ??
    requireEngineBearer(request) ??
    notImplemented(feature)
  );
}
