const forbiddenPayloadKeys = [
  "prompt",
  "response",
  "content",
  "transcript_path",
  "cwd",
  "command",
  "env",
  "file",
  "path",
  "token",
  "authorization",
  "cookie",
];

export function containsForbiddenEventPayload(value: unknown): boolean {
  if (value === null || value === undefined) {
    return false;
  }
  if (typeof value === "string") {
    return false;
  }
  if (typeof value !== "object") {
    return false;
  }
  if (Array.isArray(value)) {
    return value.some(containsForbiddenEventPayload);
  }
  return Object.entries(value as Record<string, unknown>).some(([key, nested]) => {
    const lowerKey = key.toLowerCase();
    return (
      forbiddenPayloadKeys.some((forbidden) => lowerKey.includes(forbidden)) ||
      containsForbiddenEventPayload(nested)
    );
  });
}
