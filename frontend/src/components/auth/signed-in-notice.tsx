"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { UserCheck, X } from "lucide-react";
import { takeSignedInAs } from "@/lib/signed-in-notice";

const VISIBLE_MS = 20_000;

/**
 * Once, right after a Telegram login: «+998 90 ••• •• 67 raqami bilan
 * kirdingiz», with a way out for someone who sees a number that is not
 * theirs. Fixed above the page so it never shifts a layout sized to the
 * viewport (the mobile "one screen" pages).
 */
export function SignedInNotice() {
  const t = useTranslations("SignedInNotice");
  const locale = useLocale();
  const [phone, setPhone] = useState<string | null>(null);
  const [leaving, setLeaving] = useState(false);

  useEffect(() => {
    const pending = takeSignedInAs();
    if (!pending) return;
    setPhone(pending);
    const timer = window.setTimeout(() => setPhone(null), VISIBLE_MS);
    return () => window.clearTimeout(timer);
  }, []);

  if (!phone) return null;

  async function signOut() {
    setLeaving(true);
    try {
      await fetch("/api/auth/logout", { method: "POST" });
    } catch {
      /* the login page still asks who they are */
    }
    window.location.assign(`/${locale}/login`);
  }

  return (
    <div
      role="status"
      className="fixed inset-x-3 top-[calc(0.75rem+env(safe-area-inset-top))] z-50 mx-auto flex max-w-md md:left-[calc(16rem+0.75rem)] items-start gap-3 rounded-2xl border border-success/40 bg-card p-3 shadow-lg"
    >
      <UserCheck aria-hidden="true" className="mt-0.5 h-5 w-5 shrink-0 text-success" />
      <div className="min-w-0 flex-1 space-y-1">
        {/* No-break spaces: the number is read as one thing, never split over two lines. */}
        <p className="text-sm font-extrabold leading-snug text-foreground">
          {t("message", { phone: phone.replaceAll(" ", "\u00a0") })}
        </p>
        <button
          type="button"
          onClick={() => void signOut()}
          disabled={leaving}
          className="min-h-11 text-left text-xs font-bold text-accent underline-offset-2 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-70 sm:min-h-0"
        >
          {t("notYou")}
        </button>
      </div>
      <button
        type="button"
        onClick={() => setPhone(null)}
        aria-label={t("dismiss")}
        className="-m-1 flex h-11 w-11 shrink-0 items-center justify-center rounded-xl text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <X aria-hidden="true" className="h-4 w-4" />
      </button>
    </div>
  );
}
