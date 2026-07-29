"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { usePathname, useRouter } from "next/navigation";
import Link from "next/link";
import { Menu } from "lucide-react";
import { AdminSidebar } from "@/components/admin/admin-sidebar";
import { AdminBreadcrumbs } from "@/components/admin/admin-breadcrumbs";
import { AdminMobileBar } from "@/components/admin/admin-mobile-bar";
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
      <div className="flex min-h-screen items-center justify-center bg-background text-sm text-muted-foreground">
        {t("loading")}
      </div>
    );
  }

  if (!me) return null;

  return (
    <AdminMeProvider me={me}>
      <div className="flex min-h-screen bg-background text-foreground">
        {navOpen ? (
          <button
            type="button"
            className="fixed inset-0 z-30 bg-foreground/40 lg:hidden"
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
          <header className="sticky top-0 z-20 flex items-center justify-between gap-3 border-b border-border bg-background px-4 py-1.5 sm:px-6">
            <div className="flex min-w-0 flex-1 items-center gap-3">
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="min-h-[44px] min-w-[44px] lg:hidden"
                aria-label={t("openNav")}
                onClick={() => setNavOpen(true)}
              >
                <Menu className="h-4 w-4" />
              </Button>
              <AdminBreadcrumbs locale={locale} pathname={pathname} />
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <p className="hidden text-[11px] text-muted-foreground xl:block">
                {t("commandHint")}
              </p>
              {me.totp_setup_required ? (
                <Link
                  href={`/${locale}/admin/security/totp`}
                  className="rounded-xl border border-accent/50 bg-accent/10 px-2 py-1 text-[11px] font-bold text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >
                  {t("totpSetupBanner")}
                </Link>
              ) : null}
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="min-h-[44px]"
                onClick={() => void logout()}
              >
                {t("logout")}
              </Button>
            </div>
          </header>
          {/* A div, not a <main>: all 41 admin pages render their own <main>, and
              two landmarks in one document is invalid and confuses screen readers. */}
          <div className="admin-shell-pad flex-1 pb-[calc(56px+env(safe-area-inset-bottom))] lg:pb-6">
            {children}
          </div>
        </div>
        <AdminMobileBar locale={locale} activePath={pathname} />
        <AdminCommandPalette locale={locale} />
      </div>
    </AdminMeProvider>
  );
}
