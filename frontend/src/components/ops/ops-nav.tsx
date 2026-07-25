import Link from "next/link";

type OpsNavProps = {
  locale: string;
  active: "providers" | "health";
  providersLabel: string;
  healthLabel: string;
};

export function OpsNav({ locale, active, providersLabel, healthLabel }: OpsNavProps) {
  const linkClass = (key: OpsNavProps["active"]) =>
    `rounded-lg px-3 py-2 text-xs font-bold transition-colors ${
      active === key
        ? "bg-accent/15 text-accent"
        : "text-muted-foreground hover:bg-card hover:text-foreground"
    }`;

  return (
    <nav aria-label="Ops" className="mb-6 flex flex-wrap gap-1 border-b border-border pb-3">
      <Link href={`/${locale}/ops/health`} className={linkClass("health")} aria-current={active === "health" ? "page" : undefined}>
        {healthLabel}
      </Link>
      <Link
        href={`/${locale}/ops/providers`}
        className={linkClass("providers")}
        aria-current={active === "providers" ? "page" : undefined}
      >
        {providersLabel}
      </Link>
    </nav>
  );
}
