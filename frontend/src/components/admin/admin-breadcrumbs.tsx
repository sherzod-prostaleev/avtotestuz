"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { ChevronRight } from "lucide-react";
import {
  activeGroupTitleKey,
  adminNav,
  isNavItemActive,
} from "@/components/admin/admin-nav-config";

/** Location trail derived from the nav IA — group, then leaf. */
export function AdminBreadcrumbs({
  locale,
  pathname,
}: {
  locale: string;
  pathname: string;
}) {
  const t = useTranslations("AdminNav");
  const tShell = useTranslations("AdminShell");
  const base = `/${locale}/admin`;
  const groups = adminNav(locale);
  // At the root the trail would read Home > Main > Overview — three crumbs for
  // one page. The home crumb already says it.
  const atRoot = pathname === base || pathname === `${base}/`;
  const groupKey = atRoot ? null : activeGroupTitleKey(groups, pathname);
  const group = groups.find((g) => g.titleKey === groupKey);
  const leaf = group?.items.find((item) => isNavItemActive(item.href, pathname, base));

  return (
    <nav aria-label={tShell("breadcrumbLabel")} className="min-w-0">
      <ol className="flex min-h-[44px] min-w-0 items-center gap-1.5 text-xs font-semibold text-muted-foreground">
        <li className="shrink-0">
          <Link
            href={base}
            aria-current={atRoot ? "page" : undefined}
            className="-mx-1.5 inline-flex min-h-[44px] items-center rounded px-1.5 hover:text-accent-ink focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {tShell("home")}
          </Link>
        </li>
        {group ? (
          <>
            <li aria-hidden className="flex shrink-0 items-center">
              <ChevronRight className="h-3.5 w-3.5" />
            </li>
            <li className="shrink-0">{t(group.titleKey)}</li>
          </>
        ) : null}
        {leaf ? (
          <>
            <li aria-hidden className="flex shrink-0 items-center">
              <ChevronRight className="h-3.5 w-3.5" />
            </li>
            <li className="min-w-0 truncate text-foreground" aria-current="page">
              {t(leaf.labelKey)}
            </li>
          </>
        ) : null}
      </ol>
    </nav>
  );
}
