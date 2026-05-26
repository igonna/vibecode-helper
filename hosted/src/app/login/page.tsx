"use client";

import { Mail } from "lucide-react";
import { useState } from "react";
import { PublicShell } from "@/components/public-shell";

export default function LoginPage() {
  const [email, setEmail] = useState("");
  const [inviteCode, setInviteCode] = useState("");
  const [code, setCode] = useState("");
  const [devCode, setDevCode] = useState("");
  const [step, setStep] = useState<"request" | "verify">("request");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function requestCode() {
    setIsSubmitting(true);
    setError("");
    setMessage("");
    setDevCode("");
    try {
      const response = await fetch("/api/auth/request-code", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          email,
          inviteCode: inviteCode.trim() || undefined,
        }),
      });
      const body = (await response.json()) as {
        devCode?: string;
        error?: { message?: string };
      };
      if (!response.ok) {
        throw new Error(body.error?.message ?? "Could not send login code.");
      }
      setDevCode(body.devCode ?? "");
      setStep("verify");
      setMessage("Code sent.");
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : "Could not send login code.",
      );
    } finally {
      setIsSubmitting(false);
    }
  }

  async function verifyCode() {
    setIsSubmitting(true);
    setError("");
    setMessage("");
    try {
      const response = await fetch("/api/auth/verify-code", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          email,
          code,
          inviteCode: inviteCode.trim() || undefined,
        }),
      });
      const body = (await response.json()) as {
        error?: { message?: string };
      };
      if (!response.ok) {
        throw new Error(body.error?.message ?? "Could not verify login code.");
      }
      window.location.href = "/projects";
    } catch (verifyError) {
      setError(
        verifyError instanceof Error
          ? verifyError.message
          : "Could not verify login code.",
      );
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <PublicShell>
      <section className="mx-auto flex min-h-[calc(100vh-12rem)] w-full max-w-md items-center px-5 py-12">
        <div className="w-full rounded-lg border border-white/10 bg-white/[0.045] p-6">
          <div className="flex size-10 items-center justify-center rounded-md bg-white/8">
            <Mail size={19} />
          </div>
          <h1 className="mt-5 text-2xl font-semibold">Login</h1>
          <form
            className="mt-6 grid gap-3"
            onSubmit={(event) => {
              event.preventDefault();
              if (step === "request") {
                void requestCode();
              } else {
                void verifyCode();
              }
            }}
          >
            <input
              className="h-11 rounded-md border border-white/10 bg-black/35 px-3 text-sm outline-none ring-emerald-300/40 placeholder:text-white/35 focus:ring-2"
              name="email"
              onChange={(event) => setEmail(event.target.value)}
              placeholder="Email"
              required
              type="email"
              value={email}
            />
            <input
              className="h-11 rounded-md border border-white/10 bg-black/35 px-3 text-sm outline-none ring-emerald-300/40 placeholder:text-white/35 focus:ring-2"
              name="inviteCode"
              onChange={(event) => setInviteCode(event.target.value)}
              placeholder="Invite code"
              type="text"
              value={inviteCode}
            />
            {step === "verify" ? (
              <input
                autoComplete="one-time-code"
                className="h-11 rounded-md border border-white/10 bg-black/35 px-3 text-sm outline-none ring-emerald-300/40 placeholder:text-white/35 focus:ring-2"
                name="code"
                onChange={(event) => setCode(event.target.value)}
                placeholder="Login code"
                required
                type="text"
                value={code}
              />
            ) : null}
            {devCode ? (
              <div className="rounded-md border border-emerald-300/20 bg-emerald-300/10 px-3 py-2 text-sm text-emerald-100">
                Dev code: {devCode}
              </div>
            ) : null}
            {message ? (
              <div className="rounded-md border border-white/10 bg-white/8 px-3 py-2 text-sm text-white/70">
                {message}
              </div>
            ) : null}
            {error ? (
              <div className="rounded-md border border-red-300/25 bg-red-300/10 px-3 py-2 text-sm text-red-100">
                {error}
              </div>
            ) : null}
            <button
              className="h-11 rounded-md bg-white text-sm font-medium text-black hover:bg-white/90 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={isSubmitting}
              type="submit"
            >
              {isSubmitting
                ? "Please wait"
                : step === "request"
                  ? "Send code"
                  : "Login"}
            </button>
          </form>
        </div>
      </section>
    </PublicShell>
  );
}
