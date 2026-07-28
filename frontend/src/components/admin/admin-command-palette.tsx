"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { adminNav } from "@/components/admin/admin-sidebar";
import { useAdminMeOptional } from "@/components/admin/admin-me-context";
import {
  ADMIN_ROUTE_PERMISSIONS,
  hasPermission,
  routePermissionKey,
} from "@/lib/admin-permissions";

type AdminCommandPaletteProps = {
  locale: string;
};

/** ⌘K / Ctrl+K command palette for admin routes. */
export function AdminCommandPalette({ locale }: AdminCommandPaletteProps) {
  const t = useTranslations("AdminNav");
  const tShell = useTranslations("AdminShell");
  const router = useRouter();
  const me = useAdminMeOptional();
  const [open, setOpen] = useState(false);
  const [q, setQ] = useState("");

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((v) => !v);
      }
      if (e.key === "Escape") setOpen(false);
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const items = useMemo(() => {
    const all = adminNav(locale).flatMap((g) =>
      g.items.map((item) => ({
        href: item.href,
        label: t(item.labelKey),
        stub: item.stub,
      })),
    );
    const perms = me?.permissions;
    return all.filter((item) => {
      const key = routePermissionKey(item.href, locale);
      if (!key) return true;
      const need = ADMIN_ROUTE_PERMISSIONS[key];
      if (!need) return true;
      return hasPermission(perms, need);
    });
  }, [locale, me?.permissions, t]);

  const filtered = items.filter((i) => i.label.toLowerCase().includes(q.trim().toLowerCase()));

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-[60] flex items-start justify-center bg-foreground/50 px-4 pt-[12vh] backdrop-blur-[2px]"
      role="dialog"
      aria-modal="true"
      aria-label={tShell("commandPalette")}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) setOpen(false);
      }}
    >
      <div className="w-full max-w-lg overflow-hidden rounded-2xl border border-border bg-card shadow-2xl">
        <input
          autoFocus
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder={tShell("commandPlaceholder")}
          className="h-12 w-full border-b border-border/70 bg-transparent px-4 text-sm focus:outline-none"
        />
        <ul className="max-h-72 overflow-y-auto p-1.5">
          {filtered.length === 0 ? (
            <li className="px-3 py-4 text-sm text-muted-foreground">{tShell("commandEmpty")}</li>
          ) : (
            filtered.map((item) => (
              <li key={item.href}>
                <button
                  type="button"
                  className="flex w-full items-center justify-between rounded-xl px-3 py-2.5 text-left text-sm font-semibold hover:bg-accent/15 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  onClick={() => {
                    setOpen(false);
                    setQ("");
                    router.push(item.href);
                  }}
                >
                  <span>{item.label}</span>
                  {item.stub ? (
                    <span className="text-[10px] font-bold uppercase text-muted-foreground">
                      {t("soon")}
                    </span>
                  ) : null}
                </button>
              </li>
            ))
          )}
        </ul>
      </div>
    </div>
  );
}
