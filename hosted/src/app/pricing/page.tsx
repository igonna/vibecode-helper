import Link from "next/link";
import { Check } from "lucide-react";
import { PublicShell } from "@/components/public-shell";

const features = [
  "14-day trial",
  "Personal account",
  "Unlimited projects within abuse limits",
  "Unlimited local installs within abuse limits",
  "Metadata-only prompt stats",
  "Codex and Claude Code adapters first",
];

export default function PricingPage() {
  return (
    <PublicShell>
      <section className="mx-auto w-full max-w-4xl px-5 py-16">
        <div className="rounded-lg border border-white/10 bg-white/[0.045] p-7">
          <p className="text-sm font-medium text-emerald-300">One plan</p>
          <h1 className="mt-3 text-4xl font-semibold">399 RUB / month</h1>
          <p className="mt-4 max-w-2xl text-sm leading-6 text-white/65">
            Private beta starts with a no-card trial. Billing activates when the
            paid beta opens.
          </p>
          <div className="mt-7 grid gap-3 sm:grid-cols-2">
            {features.map((feature) => (
              <div className="flex items-center gap-3 text-sm" key={feature}>
                <span className="flex size-6 items-center justify-center rounded-md bg-emerald-300/12 text-emerald-300">
                  <Check size={15} />
                </span>
                {feature}
              </div>
            ))}
          </div>
          <Link
            className="mt-8 inline-flex rounded-md bg-white px-4 py-3 text-sm font-medium text-black hover:bg-white/90"
            href="/login"
          >
            Start trial
          </Link>
        </div>
      </section>
    </PublicShell>
  );
}
