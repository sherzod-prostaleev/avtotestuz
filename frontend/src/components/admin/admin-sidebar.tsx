"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAdminMeOptional } from "@/components/admin/admin-me-context";
import {
  ADMIN_ROUTE_PERMISSIONS,
  hasPermission,
  routePermissionKey,
} from "@/lib/admin-permissions";

export type AdminNavItem = {
  href: string;
  labelKey: string;
  stub?: boolean;
};

export type AdminNavGroup = {
  titleKey: string;
  items: AdminNavItem[];
};

/** Sidebar IA from M3 SoT — stub pages use ComingSoon. Labels via AdminNav i18n. */
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
        { href: `${base}/cms/legal`, labelKey: "legal" },
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
        { href: `${base}/security/totp`, labelKey: "totp" },
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
  mobileOpen?: boolean;
  onNavigate?: () => void;
};

export function AdminSidebar({ locale, activePath, mobileOpen, onNavigate }: AdminSidebarProps) {
  const t = useTranslations("AdminNav");
  const me = useAdminMeOptional();
  const groups = adminNav(locale);

  return (
    <aside
      className={`fixed inset-y-0 left-0 z-40 flex w-[272px] flex-col border-r border-border/80 bg-[hsl(220_28%_7%)] text-foreground transition-transform lg:static lg:translate-x-0 ${
        mobileOpen ? "translate-x-0" : "-translate-x-full"
      }`}
    >
      <div className="relative overflow-hidden border-b border-border/70 px-4 py-5">
        <div
          aria-hidden
          className="pointer-events-none absolute -right-6 -top-8 h-24 w-24 rounded-full bg-accent/20 blur-2xl"
        />
        <p className="relative font-display text-xl font-black tracking-tight">Driver Go</p>
        <p className="relative mt-0.5 text-[10px] font-extrabold uppercase tracking-[0.2em] text-accent">
          {t("badge")}
        </p>
        {me?.email ? (
          <p className="relative mt-3 truncate rounded-lg bg-background/40 px-2 py-1 text-[11px] text-muted-foreground">
            {me.email}
          </p>
        ) : null}
      </div>
      <nav className="flex-1 overflow-y-auto px-2 py-3" aria-label={t("badge")}>
        {groups.map((group) => {
          const visibleItems = group.items.filter((item) => {
            const key = routePermissionKey(item.href, locale);
            if (!key) return true;
            const need = ADMIN_ROUTE_PERMISSIONS[key];
            if (!need) return true;
            return hasPermission(me?.permissions, need);
          });
          if (visibleItems.length === 0) return null;
          return (
            <div key={group.titleKey} className="mb-3.5">
              <p className="mb-1 px-2 text-[10px] font-extrabold uppercase tracking-[0.16em] text-muted-foreground/80">
                {t(group.titleKey)}
              </p>
              <ul className="space-y-0.5">
                {visibleItems.map((item) => {
                  const active =
                    item.href === `/${locale}/admin`
                      ? activePath === item.href
                      : activePath === item.href || activePath.startsWith(item.href + "/");
                  return (
                    <li key={item.href}>
                      <Link
                        href={item.href}
                        onClick={onNavigate}
                        className={`flex items-center justify-between rounded-lg px-2.5 py-1.5 text-[13px] font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                          active
                            ? "bg-accent text-accent-foreground shadow-[0_2px_0_0_hsl(var(--accent-shadow))]"
                            : "text-foreground/85 hover:bg-white/[0.04] hover:text-accent"
                        }`}
                        aria-current={active ? "page" : undefined}
                      >
                        <span>{t(item.labelKey)}</span>
                        {item.stub ? (
                          <span
                            className={`text-[9px] font-bold uppercase ${
                              active ? "text-accent-foreground/70" : "text-muted-foreground"
                            }`}
                          >
                            {t("soon")}
                          </span>
                        ) : null}
                      </Link>
                    </li>
                  );
                })}
              </ul>
            </div>
          );
        })}
      </nav>
    </aside>
  );
}
