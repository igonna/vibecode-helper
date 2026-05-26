import { PublicShell } from "@/components/public-shell";

type InfoPageProps = {
  title: string;
  children: React.ReactNode;
};

export function InfoPage({ title, children }: InfoPageProps) {
  return (
    <PublicShell>
      <section className="mx-auto w-full max-w-3xl px-5 py-16">
        <div className="rounded-lg border border-white/10 bg-white/[0.04] p-7 shadow-2xl shadow-black/20">
          <h1 className="text-3xl font-semibold tracking-normal">{title}</h1>
          <div className="mt-6 space-y-4 text-sm leading-6 text-white/70">
            {children}
          </div>
        </div>
      </section>
    </PublicShell>
  );
}
