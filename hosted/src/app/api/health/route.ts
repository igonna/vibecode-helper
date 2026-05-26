import { NextResponse } from "next/server";
import { getServerEnv } from "@/lib/env";

export function GET() {
  const env = getServerEnv();

  return NextResponse.json({
    ok: true,
    service: "hosted",
    port: env.HOSTED_PORT,
  });
}
