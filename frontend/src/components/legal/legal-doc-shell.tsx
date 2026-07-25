import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { BrandLogo } from "@/components/brand/brand-logo";
import { ThemeToggle } from "@/components/theme-toggle";

type Props = {
  brandName: string;
  title: string;
  updatedLabel: string;
  backHref: string;
  backLabel: string;
  children: React.ReactNode;
};

export function LegalDocShell({
  brandName,
  title,
  updatedLabel,
  backHref,
  backLabel,
  children,
}: Props) {
  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <header className="sticky top-0 z-40 w-full border-b border-border/70 bg-background/90 backdrop-blur-md">
        <div className="mx-auto flex h-16 max-w-3xl items-center justify-between px-4">
          <Link
            href={backHref}
            className="flex items-center gap-2.5 font-display text-xl font-black tracking-tight text-foreground"
          >
            <BrandLogo size={36} className="h-9 w-9 rounded-2xl object-cover" />
            <span>{brandName}</span>
          </Link>
          <ThemeToggle />
        </div>
      </header>

      <main className="mx-auto w-full max-w-3xl flex-1 px-4 py-10">
        <Link
          href={backHref}
          className="mb-6 inline-flex min-h-11 items-center gap-2 text-sm font-semibold text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft aria-hidden="true" className="h-4 w-4" />
          {backLabel}
        </Link>
        <h1 className="font-display text-3xl font-black tracking-tight text-foreground sm:text-4xl">
          {title}
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">{updatedLabel}</p>
        <div className="mt-8 space-y-8 text-sm leading-relaxed text-foreground/90 sm:text-[15px]">
          {children}
        </div>
      </main>
    </div>
  );
}

export function LegalSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-2">
      <h2 className="font-display text-lg font-bold text-foreground">{title}</h2>
      <div className="space-y-2 text-muted-foreground [&_strong]:font-semibold [&_strong]:text-foreground">
        {children}
      </div>
    </section>
  );
}
