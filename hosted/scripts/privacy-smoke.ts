import assert from "node:assert/strict";
import { containsForbiddenEventPayload } from "../src/lib/event-privacy";

const forbiddenSamples: unknown[] = [
  { prompt: "write code" },
  { response: "done" },
  { transcript_path: "/tmp/transcript.json" },
  { nested: { command: "rm -rf /" } },
  { nested: { env: { SECRET_TOKEN: "secret" } } },
  { files: ["src/app/page.tsx"] },
  { cwd: "/repo" },
  { authorization: "Bearer secret" },
  { cookie: "session=secret" },
];

const allowedSamples: unknown[] = [
  null,
  undefined,
  {},
  { source: "codex", event: "prompt_finished" },
  { tool_adapter: "claude_code", native_event_name: "Stop" },
  { duration_ms: 1234, status: "success" },
  ["permission_requested", { result: "approved" }],
];

for (const sample of forbiddenSamples) {
  assert.equal(
    containsForbiddenEventPayload(sample),
    true,
    `expected forbidden payload to be rejected: ${JSON.stringify(sample)}`,
  );
}

for (const sample of allowedSamples) {
  assert.equal(
    containsForbiddenEventPayload(sample),
    false,
    `expected safe metadata payload to be allowed: ${JSON.stringify(sample)}`,
  );
}

console.log("privacy smoke passed");
