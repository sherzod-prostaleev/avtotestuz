"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAdminMeOptional } from "@/components/admin/admin-me-context";
import {
  adminMobilePrimary,
  isNavItemActive,
} from "@/components/admin/admin-nav-config";
import {
  ADMIN_ROUTE_PERMISSIONS,
  hasPermission,
  routePermissionKey,
} from "@/lib/admin-permissions";

/** Thumb-zone navigation for the operations an admin runs from a phone. */
export function AdminMobileBar({
  locale,
  activePath,
}: {
  locale: string;
  activePath: string;
}) {
  const t = useTranslations("AdminNav");
  const tShell = useTranslations("AdminShell");
  const me = useAdminMeOptional();
  const base = `/${locale}/admin`;
  // Same gate as the sidebar, and for the same reason: on a phone this bar is
  // the whole navigation, so an item it shows is a capability claim.
  const items = adminMobilePrimary(locale).filter((item) => {
    const key = routePermissionKey(item.href, locale);
    if (!key) return true;
    const need = ADMIN_ROUTE_PERMISSIONS[key];
    if (!need) return true;
    return hasPermission(me?.permissions, need);
  });

  // z-20 keeps the bar above page content but below the drawer scrim (z-30).
  // At an equal z-index the bar wins on document order, paints over the scrim,
  // and stays tappable while the drawer is open — the "tap outside to close"
  // contract would silently not hold in the bottom strip of the screen.
  return (
    <nav
      aria-label={tShell("primaryNav")}
      className="fixed inset-x-0 bottom-0 z-20 border-t border-border bg-card lg:hidden"
      style={{ paddingBottom: "env(safe-area-inset-bottom)" }}
    >
      <ul className="flex items-stretch justify-between">
        {items.map((item) => {
          const active = isNavItemActive(item.href, activePath, base);
          const Icon = item.icon;
          return (
            <li key={item.href} className="flex-1">
              <Link
                href={item.href}
                aria-current={active ? "page" : undefined}
                className={`flex min-h-[56px] flex-col items-center justify-center gap-1 px-1 py-2 text-[10px] font-bold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring ${
                  active ? "text-accent" : "text-muted-foreground"
                }`}
              >
                <Icon className="h-5 w-5" aria-hidden />
                <span className="w-full truncate text-center">{t(item.labelKey)}</span>
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
