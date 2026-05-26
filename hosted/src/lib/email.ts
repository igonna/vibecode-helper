import nodemailer from "nodemailer";
import { getServerEnv } from "@/lib/env";
import { incrementMetric } from "@/lib/metrics";

type SendLoginCodeInput = {
  email: string;
  code: string;
};

export async function sendLoginCode({ email, code }: SendLoginCodeInput) {
  const env = getServerEnv();
  const provider = env.EMAIL_PROVIDER;

  if (provider === "console") {
    console.log(`[email:console] login code for ${email}: ${code}`);
    incrementMetric("email_send_total");
    return;
  }

  const config = smtpProviderConfig(provider);
  const transporter = nodemailer.createTransport({
    host: config.host,
    port: config.port,
    secure: config.port === 465,
    auth: {
      user: config.user,
      pass: config.password,
    },
  });

  try {
    await transporter.sendMail({
      from: env.EMAIL_FROM ?? config.user,
      to: email,
      subject: "Vibeengine login code",
      text: `Your Vibeengine login code is ${code}. It expires in 10 minutes.`,
    });
    incrementMetric("email_send_total");
  } catch (error) {
    incrementMetric("email_send_failed_total");
    throw error;
  }
}

function smtpProviderConfig(provider: "rusender" | "unisender_go") {
  const env = getServerEnv();

  if (provider === "rusender") {
    if (!env.RUSENDER_SMTP_USER || !env.RUSENDER_SMTP_PASSWORD) {
      throw new Error("RuSender SMTP credentials are not configured");
    }
    return {
      host: env.RUSENDER_SMTP_HOST,
      port: env.RUSENDER_SMTP_PORT,
      user: env.RUSENDER_SMTP_USER,
      password: env.RUSENDER_SMTP_PASSWORD,
    };
  }

  if (
    !env.UNISENDER_GO_SMTP_HOST ||
    !env.UNISENDER_GO_SMTP_USER ||
    !env.UNISENDER_GO_SMTP_PASSWORD
  ) {
    throw new Error("Unisender Go SMTP credentials are not configured");
  }
  return {
    host: env.UNISENDER_GO_SMTP_HOST,
    port: env.UNISENDER_GO_SMTP_PORT,
    user: env.UNISENDER_GO_SMTP_USER,
    password: env.UNISENDER_GO_SMTP_PASSWORD,
  };
}
