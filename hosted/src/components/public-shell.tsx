import Link from "next/link";
import { Activity, ArrowRight, ShieldCheck } from "lucide-react";

type PublicShellProps = {
  children: React.ReactNode;
};

export function PublicShell({ children }: PublicShellProps) {
  return (
    <main className="min-h-screen">
      <header className="mx-auto flex w-full max-w-6xl items-center justify-between px-5 py-5">
        <Link className="flex items-center gap-2 text-sm font-semibold" href="/">
          <span className="flex size-8 items-center justify-center rounded-md border border-white/10 bg-white/8">
            <Activity size={17} />
          </span>
          Vibeengine
        </Link>
        <nav className="flex items-center gap-2 text-sm text-white/72">
          <Link className="rounded-md px-3 py-2 hover:bg-white/8" href="/pricing">
            Pricing
          </Link>
          <Link className="rounded-md px-3 py-2 hover:bg-white/8" href="/login">
            Login
          </Link>
          <Link
            className="inline-flex items-center gap-2 rounded-md bg-white px-3 py-2 font-medium text-black hover:bg-white/90"
            href="/projects"
          >
            Projects
            <ArrowRight size={15} />
          </Link>
        </nav>
      </header>
      {children}
      <footer className="mx-auto flex w-full max-w-6xl flex-col gap-3 px-5 py-10 text-sm text-white/50 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2">
          <ShieldCheck size={16} />
          Metadata-only by default
        </div>
        <div className="flex gap-4">
          <Link href="/terms">Terms</Link>
          <Link href="/privacy">Privacy</Link>
          <Link href="/refund">Refund</Link>
          <Link href="/contact">Contact</Link>
        </div>
      </footer>
    </main>
  );
}
