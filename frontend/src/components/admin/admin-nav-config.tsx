import {
  Activity,
  BarChart3,
  Briefcase,
  Building2,
  CreditCard,
  FileText,
  LayoutDashboard,
  LifeBuoy,
  type LucideIcon,
  Settings,
  Shield,
  Users,
} from "lucide-react";

export type AdminNavItem = { href: string; labelKey: string; stub?: boolean };

export type AdminNavGroup = {
  titleKey: string;
  icon: LucideIcon;
  items: AdminNavItem[];
};

/** Sidebar IA from the M3 SoT (docs/superpowers/specs/2026-07-26-m3-super-admin-control-center.md §2). */
export function adminNav(locale: string): AdminNavGroup[] {
  const base = `/${locale}/admin`;
  return [
    {
      titleKey: "groupMain",
      icon: LayoutDashboard,
      items: [{ href: base, labelKey: "overview" }],
    },
    {
      titleKey: "groupMonitoring",
      icon: Activity,
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
      icon: BarChart3,
      items: [
        { href: `${base}/analytics/overview`, labelKey: "overview" },
        { href: `${base}/analytics/funnels`, labelKey: "funnels", stub: true },
        { href: `${base}/analytics/exports`, labelKey: "exports", stub: true },
      ],
    },
    {
      titleKey: "groupInvestors",
      icon: Briefcase,
      items: [{ href: `${base}/investors`, labelKey: "overview" }],
    },
    {
      titleKey: "groupB2B",
      icon: Building2,
      items: [{ href: `${base}/b2b/orgs`, labelKey: "organizations" }],
    },
    {
      titleKey: "groupUsers",
      icon: Users,
      items: [{ href: `${base}/users`, labelKey: "directory" }],
    },
    {
      titleKey: "groupContent",
      icon: FileText,
      items: [
        { href: `${base}/content/questions`, labelKey: "questions" },
        { href: `${base}/content/explanations`, labelKey: "explanations" },
        { href: `${base}/content/tickets`, labelKey: "tickets" },
        { href: `${base}/content/signs`, labelKey: "signs" },
      ],
    },
    {
      titleKey: "groupPayments",
      icon: CreditCard,
      items: [
        { href: `${base}/payments/transactions`, labelKey: "transactions" },
        { href: `${base}/payments/referral-payouts`, labelKey: "referralPayouts" },
        { href: `${base}/payments/manual`, labelKey: "manualPay" },
        { href: `${base}/payments/refunds`, labelKey: "refunds" },
        { href: `${base}/payments/webhooks`, labelKey: "webhooks", stub: true },
        { href: `${base}/payments/providers`, labelKey: "providers" },
        { href: `${base}/payments/catalog`, labelKey: "catalog", stub: true },
        { href: `${base}/payments/recon`, labelKey: "recon" },
      ],
    },
    {
      titleKey: "groupCMS",
      icon: FileText,
      items: [
        { href: `${base}/cms/home`, labelKey: "homepage" },
        { href: `${base}/cms/chrome`, labelKey: "headerFooter" },
        { href: `${base}/cms/brand`, labelKey: "brand", stub: true },
        { href: `${base}/cms/surfaces`, labelKey: "surfaces", stub: true },
        { href: `${base}/cms/legal`, labelKey: "legal" },
      ],
    },
    {
      titleKey: "groupSettings",
      icon: Settings,
      items: [
        { href: `${base}/settings/flags`, labelKey: "featureFlags" },
        { href: `${base}/settings/limits`, labelKey: "limits" },
        { href: `${base}/settings/config`, labelKey: "runtimeConfig", stub: true },
      ],
    },
    {
      titleKey: "groupSecurity",
      icon: Shield,
      items: [
        { href: `${base}/security/totp`, labelKey: "totp" },
        { href: `${base}/security/rbac`, labelKey: "adminsRbac" },
        { href: `${base}/security/ip`, labelKey: "ipAllowlist", stub: true },
        { href: `${base}/security/audit`, labelKey: "auditLog" },
      ],
    },
    {
      titleKey: "groupSupport",
      icon: LifeBuoy,
      items: [
        { href: `${base}/support/inbox`, labelKey: "inbox" },
        { href: `${base}/support/broadcasts`, labelKey: "broadcasts" },
      ],
    },
  ];
}

/** Phone bottom bar: the destinations an operator needs while away from a desk. */
export function adminMobilePrimary(locale: string) {
  const base = `/${locale}/admin`;
  return [
    { href: base, labelKey: "overview", icon: LayoutDashboard },
    { href: `${base}/payments/manual`, labelKey: "manualPay", icon: CreditCard },
    { href: `${base}/users`, labelKey: "directory", icon: Users },
    { href: `${base}/monitoring/health`, labelKey: "systemHealth", icon: Activity },
    { href: `${base}/support/inbox`, labelKey: "inbox", icon: LifeBuoy },
  ];
}

function isActive(href: string, pathname: string, base: string): boolean {
  if (href === base) return pathname === base;
  return pathname === href || pathname.startsWith(href + "/");
}

/** Which group contains the current route, or null. */
export function activeGroupTitleKey(
  groups: AdminNavGroup[],
  pathname: string,
): string | null {
  const base = groups[0]?.items[0]?.href ?? "";
  for (const group of groups) {
    if (group.items.some((item) => isActive(item.href, pathname, base))) {
      return group.titleKey;
    }
  }
  return null;
}

export { isActive as isNavItemActive };
