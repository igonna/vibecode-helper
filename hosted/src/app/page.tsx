import Link from "next/link";
import { Bell, Database, LockKeyhole, Terminal } from "lucide-react";
import { PublicShell } from "@/components/public-shell";

const points = [
  {
    icon: Terminal,
    title: "Local engine",
    body: "Hooks write locally first, so prompt tracking is not blocked by cloud access.",
  },
  {
    icon: Bell,
    title: "Live attention",
    body: "Dashboard status and browser sound show when a prompt finishes or needs attention.",
  },
  {
    icon: Database,
    title: "Small metadata",
    body: "Prompt timing and status are stored without prompt text, responses, or command logs.",
  },
  {
    icon: LockKeyhole,
    title: "Local approval",
    body: "Executable command changes require local review before they run.",
  },
];

export default function HomePage() {
  return (
    <PublicShell>
      <section className="mx-auto grid min-h-[calc(100vh-9rem)] w-full max-w-6xl content-center gap-10 px-5 py-12">
        <div className="max-w-3xl">
          <p className="text-sm font-medium uppercase tracking-[0.2em] text-emerald-300/80">
            Private beta
          </p>
          <h1 className="mt-4 text-5xl font-semibold tracking-normal text-white sm:text-6xl">
            Vibeengine
          </h1>
          <p className="mt-5 max-w-2xl text-lg leading-8 text-white/68">
            A local-first engine and hosted dashboard for prompt timing, project
            status, and safe command automation.
          </p>
          <div className="mt-8 flex flex-wrap gap-3">
            <Link
              className="rounded-md bg-white px-4 py-3 text-sm font-medium text-black hover:bg-white/90"
              href="/login"
            >
              Join private beta
            </Link>
            <Link
              className="rounded-md border border-white/12 px-4 py-3 text-sm font-medium text-white/85 hover:bg-white/8"
              href="/pricing"
            >
              View pricing
            </Link>
          </div>
        </div>
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {points.map((point) => {
            const Icon = point.icon;
            return (
              <div
                className="rounded-lg border border-white/10 bg-white/[0.045] p-4"
                key={point.title}
              >
                <Icon className="text-emerald-300" size={20} />
                <h2 className="mt-4 text-base font-medium">{point.title}</h2>
                <p className="mt-2 text-sm leading-6 text-white/58">
                  {point.body}
                </p>
              </div>
            );
          })}
        </div>
      </section>
    </PublicShell>
  );
}
