"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";

export type AdminNavItem = {
  href: string;
  labelKey: string;
  stub?: boolean;
};

export type AdminNavGroup = {
  titleKey: string;
  items: AdminNavItem[];
};

/** Sidebar IA from M3 SoT — stub pages link until modules ship. Labels via AdminNav i18n. */
export function adminNav(locale: string): AdminNavGroup[] {
  const base = `/${locale}/admin`;
  return [
    {
      titleKey: "groupMain",
      items: [{ href: base, labelKey: "overview" }],
    },
    {
      titleKey: "groupMonitoring",
      items: [
        { href: `${base}/monitoring/health`, labelKey: "systemHealth" },
        { href: `${base}/monitoring/perf`, labelKey: "apiDb" },
        { href: `${base}/monitoring/logs`, labelKey: "liveLogs" },
        { href: `${base}/monitoring/jobs`, labelKey: "jobs" },
        { href: `${base}/monitoring/alerts`, labelKey: "alerts" },
      ],
    },
    {
      titleKey: "groupAnalytics",
      items: [{ href: `${base}/analytics/overview`, labelKey: "overview" }],
    },
    {
      titleKey: "groupInvestors",
      items: [{ href: `${base}/investors`, labelKey: "overview" }],
    },
    {
      titleKey: "groupB2B",
      items: [{ href: `${base}/b2b/orgs`, labelKey: "organizations" }],
    },
    {
      titleKey: "groupUsers",
      items: [{ href: `${base}/users`, labelKey: "directory" }],
    },
    {
      titleKey: "groupContent",
      items: [
        { href: `${base}/content/questions`, labelKey: "questions" },
        { href: `${base}/content/explanations`, labelKey: "explanations" },
        { href: `${base}/content/tickets`, labelKey: "tickets", stub: true },
        { href: `${base}/content/signs`, labelKey: "signs", stub: true },
      ],
    },
    {
      titleKey: "groupPayments",
      items: [
        { href: `${base}/payments/transactions`, labelKey: "transactions" },
        { href: `${base}/payments/refunds`, labelKey: "refunds" },
        { href: `${base}/payments/providers`, labelKey: "providers" },
        { href: `${base}/payments/recon`, labelKey: "recon" },
      ],
    },
    {
      titleKey: "groupCMS",
      items: [
        { href: `${base}/cms/chrome`, labelKey: "headerFooter" },
        { href: `${base}/cms/home`, labelKey: "homepage" },
        { href: `${base}/cms/legal`, labelKey: "legal", stub: true },
      ],
    },
    {
      titleKey: "groupSettings",
      items: [
        { href: `${base}/settings/flags`, labelKey: "featureFlags" },
        { href: `${base}/settings/limits`, labelKey: "limits" },
        { href: `${base}/settings/config`, labelKey: "runtimeConfig", stub: true },
      ],
    },
    {
      titleKey: "groupSecurity",
      items: [
        { href: `${base}/security/rbac`, labelKey: "adminsRbac", stub: true },
        { href: `${base}/security/audit`, labelKey: "auditLog" },
      ],
    },
    {
      titleKey: "groupSupport",
      items: [
        { href: `${base}/support/inbox`, labelKey: "inbox" },
        { href: `${base}/support/broadcasts`, labelKey: "broadcasts" },
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
  const t = useTranslations("AdminNav");
  const groups = adminNav(locale);
  return (
    <aside className="flex w-60 shrink-0 flex-col border-r border-border bg-card">
      <div className="border-b border-border px-4 py-4">
        <p className="font-display text-lg font-black text-foreground">Driver Go</p>
        <p className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
          {t("badge")}
        </p>
        {email ? <p className="mt-2 truncate text-xs text-muted-foreground">{email}</p> : null}
      </div>
      <nav className="flex-1 overflow-y-auto px-2 py-3" aria-label={t("badge")}>
        {groups.map((group) => (
          <div key={group.titleKey} className="mb-4">
            <p className="mb-1 px-2 text-[10px] font-extrabold uppercase tracking-wider text-muted-foreground">
              {t(group.titleKey)}
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
                      <span>{t(item.labelKey)}</span>
                      {item.stub ? (
                        <span className="text-[10px] font-bold uppercase text-muted-foreground">
                          {t("soon")}
                        </span>
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
          {t("legacyOps")}
        </Link>
      </div>
    </aside>
  );
}
