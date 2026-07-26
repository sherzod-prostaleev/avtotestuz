"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { usePathname, useRouter } from "next/navigation";
import Link from "next/link";
import { Menu, X } from "lucide-react";
import { AdminSidebar } from "@/components/admin/admin-sidebar";
import { AdminMeProvider, type AdminMe } from "@/components/admin/admin-me-context";
import { AdminCommandPalette } from "@/components/admin/admin-command-palette";
import { Button } from "@/components/ui/button";

export default function AdminShellLayout({ children }: { children: React.ReactNode }) {
  const locale = useLocale();
  const t = useTranslations("AdminShell");
  const pathname = usePathname();
  const router = useRouter();
  const [me, setMe] = useState<AdminMe | null>(null);
  const [loading, setLoading] = useState(true);
  const [navOpen, setNavOpen] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const res = await fetch("/api/admin/me", { cache: "no-store" });
        if (!res.ok) {
          if (!cancelled) router.replace(`/${locale}/admin/login`);
          return;
        }
        const json = await res.json();
        if (!cancelled) setMe(json.data as AdminMe);
      } catch {
        if (!cancelled) router.replace(`/${locale}/admin/login`);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [locale, router]);

  useEffect(() => {
    setNavOpen(false);
  }, [pathname]);

  async function logout() {
    await fetch("/api/admin/auth/logout", { method: "POST" });
    router.replace(`/${locale}/admin/login`);
  }

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-[hsl(220_28%_6%)] text-sm text-muted-foreground">
        {t("loading")}
      </div>
    );
  }

  if (!me) return null;

  return (
    <AdminMeProvider me={me}>
      <div className="flex min-h-screen bg-[hsl(220_28%_6%)] text-foreground">
        {navOpen ? (
          <button
            type="button"
            className="fixed inset-0 z-30 bg-black/50 lg:hidden"
            aria-label={t("closeNav")}
            onClick={() => setNavOpen(false)}
          />
        ) : null}
        <AdminSidebar
          locale={locale}
          activePath={pathname}
          mobileOpen={navOpen}
          onNavigate={() => setNavOpen(false)}
        />
        <div className="flex min-w-0 flex-1 flex-col">
          <header className="flex items-center justify-between gap-3 border-b border-border/70 bg-background/40 px-4 py-3 backdrop-blur sm:px-5">
            <div className="flex min-w-0 items-center gap-3">
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="lg:hidden"
                aria-label={t("openNav")}
                onClick={() => setNavOpen(true)}
              >
                {navOpen ? <X className="h-4 w-4" /> : <Menu className="h-4 w-4" />}
              </Button>
              <div className="min-w-0">
                <p className="text-[10px] font-extrabold uppercase tracking-[0.18em] text-accent">
                  {t("badge")}
                </p>
                <p className="truncate text-xs font-bold text-muted-foreground">
                  {me.display_name || me.email} · {me.roles.join(", ") || t("staff")}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <p className="hidden text-[11px] text-muted-foreground sm:block">{t("commandHint")}</p>
              {me.totp_setup_required ? (
                <Link
                  href={`/${locale}/admin/security/totp`}
                  className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-2 py-1 text-[11px] font-bold text-amber-200"
                >
                  {t("totpSetupBanner")}
                </Link>
              ) : null}
              <Button type="button" size="sm" variant="outline" onClick={() => void logout()}>
                {t("logout")}
              </Button>
            </div>
          </header>
          <div className="flex-1 overflow-auto p-4 sm:p-6">{children}</div>
        </div>
        <AdminCommandPalette locale={locale} />
      </div>
    </AdminMeProvider>
  );
}
