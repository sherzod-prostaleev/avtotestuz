"use client";

import { useTranslations } from "next-intl";
import { Send } from "lucide-react";
import { openExternalUrl } from "@/lib/telegram/links";

/** Telegram's own share sheet for a link (works on the web and in the app). */
export function telegramShareUrl(url: string, text: string): string {
  return `https://t.me/share/url?url=${encodeURIComponent(url)}&text=${encodeURIComponent(text)}`;
}

/**
 * The invite link itself (now t.me/<bot>?startapp=ref_<CODE>, which opens the
 * Mini App with the code) and a one-tap share into a Telegram chat — where
 * the link is most useful, since it opens the app right there.
 */
export function ReferralShareLink({ url }: { url: string }) {
  const t = useTranslations("Referral");
  if (!url) return null;
  return (
    <div className="space-y-2">
      <div>
        <span className="text-xs font-bold uppercase tracking-wider text-muted-foreground">{t("yourLink")}</span>
        <p className="mt-0.5 select-all break-all rounded-lg border border-border bg-background/60 px-2.5 py-1.5 font-mono text-xs text-foreground">
          {url}
        </p>
      </div>
      <button
        type="button"
        onClick={() => openExternalUrl(telegramShareUrl(url, t("shareText")))}
        className="flex min-h-11 w-full items-center justify-center gap-2 rounded-xl bg-[#1f75bc] px-3 text-sm font-extrabold text-white hover:bg-[#1a66a5] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
      >
        <Send aria-hidden="true" className="h-4 w-4" />
        {t("shareTelegram")}
      </button>
    </div>
  );
}
