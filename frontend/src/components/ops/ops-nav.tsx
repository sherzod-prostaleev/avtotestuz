import Link from "next/link";

export type OpsNavKey = "health" | "providers" | "contacts" | "users" | "payments" | "audit" | "limits";

type OpsNavProps = {
  locale: string;
  active: OpsNavKey;
  labels: Partial<Record<OpsNavKey, string>>;
  adminLabel?: string;
};

const ORDER: OpsNavKey[] = ["health", "providers", "contacts", "users", "payments", "audit", "limits"];

const HREF: Record<OpsNavKey, string> = {
  health: "health",
  providers: "providers",
  contacts: "contacts",
  users: "users",
  payments: "payments",
  audit: "audit",
  limits: "limits",
};

export function OpsNav({ locale, active, labels, adminLabel }: OpsNavProps) {
  const linkClass = (key: OpsNavKey) =>
    `rounded-lg px-3 py-2 text-xs font-bold transition-colors ${
      active === key
        ? "bg-accent/15 text-accent"
        : "text-muted-foreground hover:bg-card hover:text-foreground"
    }`;

  return (
    <nav aria-label="Ops" className="mb-6 flex flex-wrap gap-1 border-b border-border pb-3">
      {ORDER.map((key) => {
        const label = labels[key];
        if (!label) return null;
        return (
          <Link
            key={key}
            href={`/${locale}/ops/${HREF[key]}`}
            className={linkClass(key)}
            aria-current={active === key ? "page" : undefined}
          >
            {label}
          </Link>
        );
      })}
      {adminLabel ? (
        <Link
          href={`/${locale}/admin`}
          className="ml-auto rounded-lg px-3 py-2 text-xs font-bold text-accent hover:bg-card"
        >
          {adminLabel}
        </Link>
      ) : null}
    </nav>
  );
}
