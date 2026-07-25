"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { X } from "lucide-react";
import { apiGet } from "@/lib/api-client";

type SupportBannerDTO = {
  enabled: boolean;
  message: string;
  href?: string;
  updated_at?: string;
};

const DISMISS_KEY = "drivergo:support-banner-dismissed";

export function SupportBanner() {
  const t = useTranslations("SupportBanner");
  const [banner, setBanner] = useState<SupportBannerDTO | null>(null);
  const [hidden, setHidden] = useState(true);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const data = await apiGet<SupportBannerDTO>("site/banner");
        if (cancelled || !data?.enabled || !data.message) {
          return;
        }
        try {
          const dismissed = sessionStorage.getItem(DISMISS_KEY);
          if (dismissed && dismissed === (data.updated_at || data.message)) {
            return;
          }
        } catch {
          /* ignore */
        }
        setBanner(data);
        setHidden(false);
      } catch {
        /* banner is optional chrome */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  if (hidden || !banner) return null;

  const href = banner.href?.startsWith("/") ? banner.href : undefined;

  function dismiss() {
    try {
      sessionStorage.setItem(DISMISS_KEY, banner.updated_at || banner.message || "");
    } catch {
      /* ignore */
    }
    setHidden(true);
  }

  return (
    <div
      role="status"
      className="flex items-start gap-3 border-b border-border bg-accent/10 px-4 py-3 text-sm"
    >
      <div className="min-w-0 flex-1">
        <p className="font-medium text-foreground">{banner.message}</p>
        {href ? (
          <Link href={href} className="mt-1 inline-block text-xs font-semibold text-accent">
            {t("cta")}
          </Link>
        ) : null}
      </div>
      <button
        type="button"
        onClick={dismiss}
        className="shrink-0 rounded-lg p-1 text-muted-foreground hover:bg-background hover:text-foreground"
        aria-label={t("dismiss")}
      >
        <X className="h-4 w-4" aria-hidden />
      </button>
    </div>
  );
}
