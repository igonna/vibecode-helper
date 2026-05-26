import type { NextRequest } from "next/server";
import { getServerEnv } from "@/lib/env";
import { metricsText } from "@/lib/metrics";

export function GET(request: NextRequest) {
  const env = getServerEnv();

  if (env.HOSTED_METRICS_TOKEN) {
    const expected = `Bearer ${env.HOSTED_METRICS_TOKEN}`;
    if (request.headers.get("authorization") !== expected) {
      return new Response("not found", { status: 404 });
    }
  } else if (env.NODE_ENV === "production") {
    return new Response("not found", { status: 404 });
  }

  return new Response(`${metricsText()}\n`, {
    headers: {
      "content-type": "text/plain; version=0.0.4; charset=utf-8",
      "cache-control": "no-store",
    },
  });
}
