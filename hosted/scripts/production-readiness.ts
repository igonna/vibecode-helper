import { readFileSync } from "node:fs";
import { request as httpsRequest } from "node:https";
import { lookup } from "node:dns/promises";
import postgres from "postgres";

type Check = {
  name: string;
  ok: boolean;
  detail: string;
};

const envFile = readArgValue("--env-file");
const runNetworkChecks = process.argv.includes("--network");

if (envFile) {
  loadEnvFile(envFile);
}

function loadEnvFile(path: string) {
  const raw = readFileSync(path, "utf8");
  for (const line of raw.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) {
      continue;
    }
    const separator = trimmed.indexOf("=");
    if (separator === -1) {
      continue;
    }
    const key = trimmed.slice(0, separator).trim();
    const value = trimmed
      .slice(separator + 1)
      .trim()
      .replace(/^"(.*)"$/, "$1")
      .replace(/^'(.*)'$/, "$1");
    if (!(key in process.env) || process.env[key]?.trim() === "") {
      process.env[key] = value;
    }
  }
}

function readArgValue(name: string) {
  const inline = process.argv.find((arg) => arg.startsWith(`${name}=`));
  if (inline) {
    return inline.slice(name.length + 1);
  }
  const index = process.argv.indexOf(name);
  if (index !== -1) {
    return process.argv[index + 1];
  }
  return undefined;
}

function value(key: string) {
  return process.env[key]?.trim() ?? "";
}

function present(key: string) {
  return value(key).length > 0;
}

function placeholder(key: string) {
  const current = value(key).toLowerCase();
  return (
    !current ||
    current.includes("change_me") ||
    current.includes("example.com") ||
    current === "dev_engine_token_secret_change_before_production"
  );
}

function check(name: string, ok: boolean, detail: string): Check {
  return { name, ok, detail };
}

async function main() {
  const checks: Check[] = [];
  const publicUrl = value("HOSTED_PUBLIC_URL");
  const databaseUrl = value("HOSTED_DATABASE_URL");
  const emailProvider = value("EMAIL_PROVIDER") || "rusender";

  checks.push(
    check(
      "public URL uses production HTTPS",
      publicUrl.startsWith("https://") &&
        !publicUrl.includes("localhost") &&
        !publicUrl.includes("example.com"),
      "HOSTED_PUBLIC_URL must be the real HTTPS domain.",
    ),
  );
  checks.push(
    check(
      "database URL configured",
      present("HOSTED_DATABASE_URL") && !placeholder("HOSTED_DATABASE_URL"),
      "HOSTED_DATABASE_URL must point at production Postgres.",
    ),
  );
  checks.push(
    check(
      "Postgres password changed",
      present("POSTGRES_PASSWORD") && !placeholder("POSTGRES_PASSWORD"),
      "POSTGRES_PASSWORD must not use the example/default password.",
    ),
  );
  checks.push(
    check(
      "engine token secret configured",
      value("HOSTED_ENGINE_TOKEN_SECRET").length >= 32 &&
        !placeholder("HOSTED_ENGINE_TOKEN_SECRET"),
      "HOSTED_ENGINE_TOKEN_SECRET must be a unique random secret.",
    ),
  );
  checks.push(
    check(
      "metrics token configured",
      value("HOSTED_METRICS_TOKEN").length >= 24 &&
        !placeholder("HOSTED_METRICS_TOKEN"),
      "HOSTED_METRICS_TOKEN must protect production metrics.",
    ),
  );
  checks.push(
    check(
      "email sender configured",
      /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value("EMAIL_FROM")) &&
        !placeholder("EMAIL_FROM"),
      "EMAIL_FROM must be a real sender address on the sending domain.",
    ),
  );
  checks.push(
    check(
      "email provider configured",
      emailProvider === "rusender" || emailProvider === "unisender_go",
      "EMAIL_PROVIDER must be rusender or unisender_go in production.",
    ),
  );
  if (emailProvider === "rusender") {
    checks.push(
      check(
        "RuSender SMTP credentials configured",
        present("RUSENDER_SMTP_USER") && present("RUSENDER_SMTP_PASSWORD"),
        "RUSENDER_SMTP_USER and RUSENDER_SMTP_PASSWORD are required.",
      ),
    );
  }
  if (emailProvider === "unisender_go") {
    checks.push(
      check(
        "Unisender Go SMTP credentials configured",
        present("UNISENDER_GO_SMTP_HOST") &&
          present("UNISENDER_GO_SMTP_USER") &&
          present("UNISENDER_GO_SMTP_PASSWORD"),
        "UNISENDER_GO_SMTP_HOST/USER/PASSWORD are required.",
      ),
    );
  }
  checks.push(
    check(
      "YooKassa credentials configured",
      present("YOOKASSA_SHOP_ID") && present("YOOKASSA_SECRET_KEY"),
      "YOOKASSA_SHOP_ID and YOOKASSA_SECRET_KEY are required for paid use.",
    ),
  );
  checks.push(
    check(
      "YooKassa API URL configured",
      value("YOOKASSA_API_URL").startsWith("https://"),
      "YOOKASSA_API_URL must be HTTPS.",
    ),
  );

  if (runNetworkChecks && publicUrl) {
    checks.push(await checkPublicDns(publicUrl));
    checks.push(await checkPublicTls(publicUrl));
  }
  if (runNetworkChecks && databaseUrl) {
    checks.push(await checkPostgres(databaseUrl));
  }

  for (const item of checks) {
    const prefix = item.ok ? "ok" : "fail";
    console.log(`${prefix} - ${item.name}: ${item.detail}`);
  }

  const failures = checks.filter((item) => !item.ok);
  if (failures.length > 0) {
    console.error(`production readiness failed: ${failures.length} check(s) failed`);
    process.exit(1);
  }
  console.log("production readiness passed");
}

async function checkPublicDns(rawUrl: string): Promise<Check> {
  try {
    const host = new URL(rawUrl).hostname;
    const addresses = await lookup(host, { all: true });
    return check(
      "public DNS resolves",
      addresses.length > 0,
      `resolved ${host} to ${addresses.map((item) => item.address).join(", ")}`,
    );
  } catch (error) {
    return check("public DNS resolves", false, formatError(error));
  }
}

async function checkPublicTls(rawUrl: string): Promise<Check> {
  return new Promise((resolve) => {
    const url = new URL(rawUrl);
    const request = httpsRequest(
      {
        hostname: url.hostname,
        path: "/api/health",
        method: "GET",
        timeout: 10_000,
      },
      (response) => {
        response.resume();
        resolve(
          check(
            "public TLS health endpoint",
            Boolean(response.statusCode && response.statusCode < 500),
            `GET /api/health returned HTTP ${response.statusCode}`,
          ),
        );
      },
    );
    request.on("timeout", () => {
      request.destroy(new Error("timeout"));
    });
    request.on("error", (error) => {
      resolve(check("public TLS health endpoint", false, formatError(error)));
    });
    request.end();
  });
}

async function checkPostgres(databaseUrl: string): Promise<Check> {
  const sql = postgres(databaseUrl, { max: 1, connect_timeout: 10 });
  try {
    const rows = await sql<{ ok: number }[]>`select 1 as ok`;
    return check(
      "production Postgres connection",
      rows[0]?.ok === 1,
      "select 1 succeeded",
    );
  } catch (error) {
    return check("production Postgres connection", false, formatError(error));
  } finally {
    await sql.end().catch(() => undefined);
  }
}

function formatError(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
