import { z } from "zod";

const serverEnvSchema = z.object({
  HOSTED_DATABASE_URL: z.string().url().optional(),
  HOSTED_PORT: z.coerce.number().int().positive().default(8788),
  HOSTED_PUBLIC_URL: z.string().url().default("http://localhost:8788"),
  HOSTED_ENGINE_TOKEN_SECRET: z
    .string()
    .min(32)
    .default("dev_engine_token_secret_change_before_production"),
  HOSTED_METRICS_TOKEN: z
    .preprocess((value) => (value === "" ? undefined : value), z.string().min(24).optional()),
  EMAIL_PROVIDER: z
    .enum(["console", "rusender", "unisender_go"])
    .default("console"),
  EMAIL_FROM: z.string().email().optional(),
  RUSENDER_SMTP_HOST: z.string().default("smtp.rusender.ru"),
  RUSENDER_SMTP_PORT: z.coerce.number().int().positive().default(465),
  RUSENDER_SMTP_USER: z.string().optional(),
  RUSENDER_SMTP_PASSWORD: z.string().optional(),
  UNISENDER_GO_SMTP_HOST: z.string().optional(),
  UNISENDER_GO_SMTP_PORT: z.coerce.number().int().positive().default(465),
  UNISENDER_GO_SMTP_USER: z.string().optional(),
  UNISENDER_GO_SMTP_PASSWORD: z.string().optional(),
  YOOKASSA_API_URL: z.string().url().default("https://api.yookassa.ru"),
  YOOKASSA_SHOP_ID: z.string().optional(),
  YOOKASSA_SECRET_KEY: z
    .preprocess((value) => (value === "" ? undefined : value), z.string().optional()),
  NODE_ENV: z
    .enum(["development", "test", "production"])
    .default("development"),
});

export function getServerEnv() {
  return serverEnvSchema.parse(process.env);
}
