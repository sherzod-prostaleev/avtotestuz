"use client";

import { useTranslations } from "next-intl";
import { Check, ExternalLink, Loader2, RefreshCw, Send, Smartphone } from "lucide-react";
import { TelegramUnlink } from "../telegram-unlink";
import { useTelegramLink } from "../use-telegram-link";
import { MobileScreen } from "./mobile-screen";

/**
 * Telegram linking on a phone. Its own fetch of `me/telegram` rather than a
 * prop from the list, so opening the panel always shows the current state —
 * the same hook the wide card uses, not a query hook.
 */
export function MobileTelegram({ onBack }: { onBack: () => void }) {
  const t = useTranslations("TelegramLink");
  const tApp = useTranslations("TelegramApp");
  const link = useTelegramLink();
  const { status, loading, mode, errorKey } = link;

  const linked = status?.linked === true;
  // Inside the Mini App and linked to the Telegram user who opened it, with a
  // verified phone: a status, nothing to do but (optionally) unlink.
  const statusOnly = mode === "linked";
  const ok = statusOnly || (mode === null && linked);

  const title = (() => {
    if (mode === "linked") return tApp("linkedStatus");
    if (mode === "confirm") return t("confirmTitle");
    if (mode === "other") return t("linkedOtherTitle");
    return linked ? t("linkedTitle") : t("notLinked");
  })();
  const detail = (() => {
    if (mode === "confirm") return tApp("phoneConfirmNote");
    if (linked && status?.username) return t("linkedAs", { username: status.username });
    return link.inMiniApp ? t("subtitleInApp") : t("subtitle");
  })();
  // The explainer under the actions. Gone once there is nothing to do, and
  // the Mini App's never mentions the website's 10-minute bot link.
  const hint = statusOnly ? null : mode === "other" ? t("linkedOtherHint") : link.inMiniApp ? t("subtitleInApp") : t("subtitle");

  return (
    <MobileScreen title={t("title")} onBack={onBack}>
      <div
        className={`surface-raised flex items-center gap-3 rounded-2xl border p-3 ${
          ok ? "border-success/40 bg-success/[0.06]" : "border-border bg-card"
        }`}
      >
        <span
          className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-full ${
            ok ? "bg-success/15 text-success" : "bg-accent/15 text-accent"
          }`}
        >
          <Send aria-hidden="true" className="h-5 w-5" />
        </span>
        <div role="status" className="min-w-0 flex-1">
          {loading && !status ? (
            <span aria-hidden="true" className="block h-5 w-40 max-w-full animate-pulse rounded bg-border/60 motion-reduce:animate-none" />
          ) : (
            <>
              <p className="truncate text-sm font-bold text-foreground">{title}</p>
              <p className="truncate text-xs text-muted-foreground">{detail}</p>
            </>
          )}
        </div>
        {ok && <Check aria-hidden="true" className="h-5 w-5 shrink-0 text-success" />}
      </div>

      {/* Mini App: one clear action, linking in place through Telegram's
          signed phone share. The website keeps refresh + the bot link. */}
      {link.inMiniApp && !statusOnly && status && (
        <button
          type="button"
          onClick={() => void link.linkInApp()}
          disabled={link.linking || loading}
          className="flex min-h-12 w-full items-center justify-center gap-2 rounded-2xl border-b-4 border-accent-shadow bg-accent px-4 text-sm font-extrabold text-accent-foreground transition-transform active:translate-y-0.5 disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {link.linking ? (
            <Loader2 aria-hidden="true" className="h-4 w-4 animate-spin motion-reduce:animate-none" />
          ) : (
            <Smartphone aria-hidden="true" className="h-4 w-4" />
          )}
          {link.linking ? t("linkingInApp") : t("linkInApp")}
        </button>
      )}

      {!link.inMiniApp && (
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => void link.load()}
            disabled={loading}
            className="flex min-h-11 flex-1 items-center justify-center gap-2 rounded-xl border border-border bg-card text-sm font-bold disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <RefreshCw aria-hidden="true" className={`h-4 w-4 ${loading ? "animate-spin motion-reduce:animate-none" : ""}`} />
            {t("refresh")}
          </button>
          <button
            type="button"
            onClick={() => void link.startDeepLink()}
            disabled={link.linking}
            className="flex min-h-11 flex-1 items-center justify-center gap-2 rounded-xl border border-border bg-card text-sm font-bold text-muted-foreground disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {linked ? t("relinkButton") : t("linkButton")}
          </button>
        </div>
      )}

      {errorKey && (
        <p role="alert" className="text-sm text-destructive">
          {t(errorKey)}
        </p>
      )}

      {/* Only when a link was actually started — the deep link is a one-shot
          token and inventing a button for it before then would go nowhere. */}
      {link.deepLink && (
        <a
          href={link.deepLink}
          target="_blank"
          rel="noopener noreferrer"
          className="flex min-h-11 items-center justify-center gap-2 rounded-xl border border-accent/40 bg-accent/10 text-sm font-bold text-accent"
        >
          <ExternalLink aria-hidden="true" className="h-4 w-4" />
          {t("deepLinkHint")}
        </a>
      )}

      {hint && (
        <div className="rounded-2xl border border-border bg-card p-3">
          <p className="text-[13px] leading-snug text-muted-foreground">{hint}</p>
        </div>
      )}

      {linked && (
        <TelegramUnlink
          confirming={link.confirmingUnlink}
          unlinking={link.unlinking}
          onAsk={link.askUnlink}
          onCancel={link.cancelUnlink}
          onConfirm={() => void link.unlink()}
        />
      )}
    </MobileScreen>
  );
}
