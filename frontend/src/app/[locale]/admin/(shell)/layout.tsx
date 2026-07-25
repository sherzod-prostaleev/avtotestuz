"use client";

import { useEffect, useState } from "react";
import { useLocale } from "next-intl";
import { usePathname, useRouter } from "next/navigation";
import { AdminSidebar } from "@/components/admin/admin-sidebar";
import { Button } from "@/components/ui/button";

type AdminMe = {
  id: string;
  email: string;
  display_name: string;
  roles: string[];
  permissions: string[];
};

export default function AdminShellLayout({ children }: { children: React.ReactNode }) {
  const locale = useLocale();
  const pathname = usePathname();
  const router = useRouter();
  const [me, setMe] = useState<AdminMe | null>(null);
  const [loading, setLoading] = useState(true);

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

  async function logout() {
    await fetch("/api/admin/auth/logout", { method: "POST" });
    router.replace(`/${locale}/admin/login`);
  }

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background text-sm text-muted-foreground">
        Admin yuklanmoqda…
      </div>
    );
  }

  if (!me) return null;

  return (
    <div className="flex min-h-screen bg-background text-foreground">
      <AdminSidebar locale={locale} activePath={pathname} email={me.email} />
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex items-center justify-between border-b border-border px-5 py-3">
          <p className="text-xs font-bold uppercase tracking-wider text-muted-foreground">
            {me.roles.join(", ") || "staff"}
          </p>
          <Button type="button" size="sm" variant="outline" onClick={() => void logout()}>
            Chiqish
          </Button>
        </header>
        <div className="flex-1 overflow-auto p-5">{children}</div>
      </div>
    </div>
  );
}
