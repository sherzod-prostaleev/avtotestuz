"use client";

import { useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { ChevronDown } from "lucide-react";
import { useAdminMeOptional } from "@/components/admin/admin-me-context";
import {
  activeGroupTitleKey,
  adminNav,
  isNavItemActive,
  type AdminNavGroup,
} from "@/components/admin/admin-nav-config";
import {
  ADMIN_ROUTE_PERMISSIONS,
  hasPermission,
  routePermissionKey,
} from "@/lib/admin-permissions";

export { adminNav } from "@/components/admin/admin-nav-config";
export type { AdminNavGroup, AdminNavItem } from "@/components/admin/admin-nav-config";

type AdminSidebarProps = {
  locale: string;
  activePath: string;
  mobileOpen?: boolean;
  onNavigate?: () => void;
};

export function AdminSidebar({
  locale,
  activePath,
  mobileOpen,
  onNavigate,
}: AdminSidebarProps) {
  const t = useTranslations("AdminNav");
  const tShell = useTranslations("AdminShell");
  const me = useAdminMeOptional();
  const groups = adminNav(locale);
  const activeGroup = activeGroupTitleKey(groups, activePath);
  const [openGroups, setOpenGroups] = useState<Record<string, boolean>>({});
  const base = `/${locale}/admin`;

  function isOpen(group: AdminNavGroup): boolean {
    return openGroups[group.titleKey] ?? group.titleKey === activeGroup;
  }

  function toggle(titleKey: string, current: boolean) {
    setOpenGroups((prev) => ({ ...prev, [titleKey]: !current }));
  }

  return (
    <aside
      className={`fixed inset-y-0 left-0 z-40 flex w-[272px] flex-col border-r border-border bg-card transition-transform lg:static lg:translate-x-0 ${
        mobileOpen ? "translate-x-0" : "-translate-x-full"
      }`}
    >
      <div className="border-b border-border px-4 py-4">
        <p className="font-display text-xl font-black tracking-tight">Driver Go</p>
        <p className="mt-0.5 text-[10px] font-extrabold uppercase tracking-[0.2em] text-accent">
          {t("badge")}
        </p>
        {me?.email ? (
          <div className="mt-3 rounded-xl bg-muted px-2 py-1.5">
            <p className="truncate text-[11px] font-bold text-foreground">
              {me.display_name || me.email}
            </p>
            {/* Which hat the operator is wearing decides what the panel will
                let them do, so it stays visible rather than living in /me. */}
            <p className="truncate text-[10px] text-muted-foreground">
              {me.roles.length ? me.roles.join(", ") : tShell("staff")}
            </p>
          </div>
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

          const open = isOpen(group);
          const Icon = group.icon;
          const panelId = `admin-nav-${group.titleKey}`;

          return (
            <div key={group.titleKey} className="mb-1">
              <button
                type="button"
                onClick={() => toggle(group.titleKey, open)}
                aria-expanded={open}
                aria-controls={panelId}
                aria-label={`${t(group.titleKey)} — ${open ? t("collapseGroup") : t("expandGroup")}`}
                className="flex min-h-[44px] w-full items-center gap-2.5 rounded-xl px-2.5 py-2 text-left text-[13px] font-bold text-foreground/90 transition-colors hover:bg-accent/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <Icon className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden />
                <span className="flex-1 truncate">{t(group.titleKey)}</span>
                <ChevronDown
                  aria-hidden
                  className={`h-4 w-4 shrink-0 text-muted-foreground transition-transform ${
                    open ? "rotate-180" : ""
                  }`}
                />
              </button>

              {open ? (
                <ul id={panelId} className="mb-2 mt-0.5 space-y-0.5 pl-6">
                  {visibleItems.map((item) => {
                    const active = isNavItemActive(item.href, activePath, base);
                    return (
                      <li key={item.href}>
                        <Link
                          href={item.href}
                          onClick={onNavigate}
                          aria-current={active ? "page" : undefined}
                          className={`flex min-h-[40px] items-center justify-between gap-2 rounded-xl px-2.5 py-1.5 text-[13px] font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                            active
                              ? "bg-accent text-accent-foreground"
                              : "text-foreground/80 hover:bg-accent/10 hover:text-accent"
                          }`}
                        >
                          <span className="truncate">{t(item.labelKey)}</span>
                          {item.stub ? (
                            <span
                              className={`shrink-0 text-[9px] font-bold uppercase ${
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
              ) : null}
            </div>
          );
        })}
      </nav>
    </aside>
  );
}
