import Link from "next/link";

export type AdminNavItem = {
  href: string;
  label: string;
  stub?: boolean;
};

export type AdminNavGroup = {
  title: string;
  items: AdminNavItem[];
};

/** Sidebar IA from M3 SoT — stub pages link until modules ship. */
export function adminNav(locale: string): AdminNavGroup[] {
  const base = `/${locale}/admin`;
  return [
    {
      title: "Asosiy",
      items: [{ href: base, label: "Overview" }],
    },
    {
      title: "Monitoring",
      items: [
        { href: `${base}/monitoring/health`, label: "System health", stub: true },
        { href: `${base}/monitoring/perf`, label: "API & DB", stub: true },
        { href: `${base}/monitoring/logs`, label: "Live logs", stub: true },
        { href: `${base}/monitoring/jobs`, label: "Jobs", stub: true },
        { href: `${base}/monitoring/alerts`, label: "Alerts", stub: true },
      ],
    },
    {
      title: "Users",
      items: [
        { href: `${base}/users`, label: "Directory" },
      ],
    },
    {
      title: "Content",
      items: [
        { href: `${base}/content/questions`, label: "Questions" },
        { href: `${base}/content/explanations`, label: "Explanations" },
        { href: `${base}/content/tickets`, label: "Tickets", stub: true },
        { href: `${base}/content/signs`, label: "Signs", stub: true },
      ],
    },
    {
      title: "Payments",
      items: [
        { href: `${base}/payments/transactions`, label: "Transactions" },
        { href: `${base}/payments/refunds`, label: "Refunds" },
        { href: `${base}/payments/providers`, label: "Providers" },
      ],
    },
    {
      title: "CMS",
      items: [
        { href: `${base}/cms/chrome`, label: "Header / footer" },
        { href: `${base}/cms/home`, label: "Homepage", stub: true },
        { href: `${base}/cms/legal`, label: "Legal", stub: true },
      ],
    },
    {
      title: "Settings",
      items: [
        { href: `${base}/settings/flags`, label: "Feature flags", stub: true },
        { href: `${base}/settings/config`, label: "Runtime config", stub: true },
      ],
    },
    {
      title: "Security",
      items: [
        { href: `${base}/security/rbac`, label: "Admins & RBAC", stub: true },
        { href: `${base}/security/audit`, label: "Audit log", stub: true },
      ],
    },
  ];
}

type AdminSidebarProps = {
  locale: string;
  activePath: string;
  email?: string;
};

export function AdminSidebar({ locale, activePath, email }: AdminSidebarProps) {
  const groups = adminNav(locale);
  return (
    <aside className="flex w-60 shrink-0 flex-col border-r border-border bg-card">
      <div className="border-b border-border px-4 py-4">
        <p className="font-display text-lg font-black text-foreground">Driver Go</p>
        <p className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">Admin</p>
        {email ? <p className="mt-2 truncate text-xs text-muted-foreground">{email}</p> : null}
      </div>
      <nav className="flex-1 overflow-y-auto px-2 py-3" aria-label="Admin">
        {groups.map((group) => (
          <div key={group.title} className="mb-4">
            <p className="mb-1 px-2 text-[10px] font-extrabold uppercase tracking-wider text-muted-foreground">
              {group.title}
            </p>
            <ul className="space-y-0.5">
              {group.items.map((item) => {
                const active = activePath === item.href || activePath.startsWith(item.href + "/");
                return (
                  <li key={item.href}>
                    <Link
                      href={item.href}
                      className={`flex items-center justify-between rounded-lg px-2 py-1.5 text-sm font-semibold transition-colors ${
                        active
                          ? "bg-accent/15 text-accent"
                          : "text-foreground hover:bg-background hover:text-accent"
                      }`}
                      aria-current={active ? "page" : undefined}
                    >
                      <span>{item.label}</span>
                      {item.stub ? (
                        <span className="text-[10px] font-bold uppercase text-muted-foreground">soon</span>
                      ) : null}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </nav>
      <div className="border-t border-border p-3">
        <Link
          href={`/${locale}/ops/health`}
          className="block rounded-lg px-2 py-1.5 text-xs font-semibold text-muted-foreground hover:text-accent"
        >
          Legacy ops (deprecated)
        </Link>
      </div>
    </aside>
  );
}
