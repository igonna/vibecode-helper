import { createHmac, timingSafeEqual } from "node:crypto";
import { getServerEnv } from "@/lib/env";

type EngineAccessPayload = {
  accountId: string;
  userId: string;
  epoch: number;
  exp: number;
};

export function createEngineAccessToken(payload: Omit<EngineAccessPayload, "exp">) {
  const fullPayload: EngineAccessPayload = {
    ...payload,
    exp: Math.floor(Date.now() / 1000) + 15 * 60,
  };
  const encodedPayload = Buffer.from(JSON.stringify(fullPayload)).toString(
    "base64url",
  );
  const signature = signPayload(encodedPayload);
  return {
    token: `${encodedPayload}.${signature}`,
    expiresAt: new Date(fullPayload.exp * 1000),
  };
}

export function verifyEngineAccessToken(token: string) {
  const [encodedPayload, signature] = token.split(".");
  if (!encodedPayload || !signature) {
    return null;
  }

  const expected = signPayload(encodedPayload);
  const actualBuffer = Buffer.from(signature);
  const expectedBuffer = Buffer.from(expected);
  if (
    actualBuffer.length !== expectedBuffer.length ||
    !timingSafeEqual(actualBuffer, expectedBuffer)
  ) {
    return null;
  }

  const payload = JSON.parse(
    Buffer.from(encodedPayload, "base64url").toString("utf8"),
  ) as EngineAccessPayload;
  if (payload.exp <= Math.floor(Date.now() / 1000)) {
    return null;
  }
  return payload;
}

function signPayload(encodedPayload: string) {
  return createHmac("sha256", getServerEnv().HOSTED_ENGINE_TOKEN_SECRET)
    .update(encodedPayload)
    .digest("base64url");
}
