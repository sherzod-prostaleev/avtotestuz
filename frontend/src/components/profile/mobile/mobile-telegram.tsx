"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Check, ExternalLink, RefreshCw, Send } from "lucide-react";
import { apiGet, apiPost } from "@/lib/api-client";
import { MobileScreen } from "./mobile-screen";

interface TelegramStatus {
  linked: boolean;
  username?: string;
  linked_at?: string;
}

interface LinkTokenResult {
  token: string;
  deep_link: string;
  expires_at: string;
}

/**
 * Telegram linking on a phone. Its own fetch of `me/telegram` rather than a
 * prop from the list, so opening the panel always shows the current state —
 * the same plain `apiGet` the wide card uses, not a query hook.
 */
export function MobileTelegram({ onBack }: { onBack: () => void }) {
  const t = useTranslations("TelegramLink");
  const [status, setStatus] = useState<TelegramStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [deepLink, setDeepLink] = useState<string | null>(null);
  const [errorKey, setErrorKey] = useState<"loadError" | "linkError" | "unconfigured" | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setErrorKey(null);
    try {
      setStatus(await apiGet<TelegramStatus>("me/telegram"));
    } catch {
      setErrorKey("loadError");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function startLink() {
    setBusy(true);
    setErrorKey(null);
    try {
      const result = await apiPost<LinkTokenResult>("me/telegram/link-token");
      setDeepLink(result.deep_link);
      window.open(result.deep_link, "_blank", "noopener");
    } catch {
      setErrorKey("linkError");
    } finally {
      setBusy(false);
    }
  }

  const linked = status?.linked === true;

  return (
    <MobileScreen title={t("title")} onBack={onBack}>
      <div
        className={`surface-raised flex items-center gap-3 rounded-2xl border p-3 ${
          linked ? "border-success/40 bg-success/[0.06]" : "border-border bg-card"
        }`}
      >
        <span
          className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-full ${
            linked ? "bg-success/15 text-success" : "bg-accent/15 text-accent"
          }`}
        >
          <Send aria-hidden="true" className="h-5 w-5" />
        </span>
        <div className="min-w-0 flex-1">
          {loading ? (
            <span aria-hidden="true" className="block h-5 w-40 max-w-full animate-pulse rounded bg-border/60" />
          ) : (
            <>
              <p className="truncate text-sm font-bold">
                {linked ? t("linkedTitle") : t("notLinked")}
              </p>
              <p className="truncate text-xs text-muted-foreground">
                {linked && status?.username
                  ? t("linkedAs", { username: status.username })
                  : t("subtitle")}
              </p>
            </>
          )}
        </div>
        {linked && <Check aria-hidden="true" className="h-5 w-5 shrink-0 text-success" />}
      </div>

      <div className="flex gap-2">
        <button
          type="button"
          onClick={() => void load()}
          disabled={loading}
          className="flex min-h-11 flex-1 items-center justify-center gap-2 rounded-xl border border-border bg-card text-sm font-bold disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <RefreshCw aria-hidden="true" className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
          {t("refresh")}
        </button>
        <button
          type="button"
          onClick={() => void startLink()}
          disabled={busy}
          className="flex min-h-11 flex-1 items-center justify-center gap-2 rounded-xl border border-border bg-card text-sm font-bold text-muted-foreground disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {linked ? t("relinkButton") : t("linkButton")}
        </button>
      </div>

      {errorKey && (
        <p role="alert" className="text-sm text-destructive">
          {t(errorKey)}
        </p>
      )}

      {/* Only when a link was actually started — the deep link is a one-shot
          token and inventing a button for it before then would go nowhere. */}
      {deepLink && (
        <a
          href={deepLink}
          target="_blank"
          rel="noopener noreferrer"
          className="flex min-h-11 items-center justify-center gap-2 rounded-xl border border-accent/40 bg-accent/10 text-sm font-bold text-accent"
        >
          <ExternalLink aria-hidden="true" className="h-4 w-4" />
          {t("deepLinkHint")}
        </a>
      )}

      <div className="rounded-2xl border border-border bg-card p-3">
        <p className="text-[13px] leading-snug text-muted-foreground">{t("subtitle")}</p>
      </div>
    </MobileScreen>
  );
}
