"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Check, Copy, ExternalLink, Loader2, RefreshCw, Send, Smartphone } from "lucide-react";
import { TelegramUnlink } from "./telegram-unlink";
import { useTelegramLink } from "./use-telegram-link";

export function TelegramLinkCard() {
  const t = useTranslations("TelegramLink");
  const tApp = useTranslations("TelegramApp");
  const link = useTelegramLink();
  const { status, loading, errorKey, mode, deepLink, expiresAt } = link;
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    if (!deepLink) return;
    try {
      await navigator.clipboard.writeText(deepLink);
      setCopied(true);
      setTimeout(() => setCopied(false), 2500);
    } catch {
      // clipboard may be denied
    }
  };

  const usernameLabel = status?.username ? `@${status.username.replace(/^@/, "")}` : null;
  const unlink = status?.linked ? (
    <TelegramUnlink
      confirming={link.confirmingUnlink}
      unlinking={link.unlinking}
      onAsk={link.askUnlink}
      onCancel={link.cancelUnlink}
      onConfirm={() => void link.unlink()}
    />
  ) : null;
  const errorBox = errorKey && (
    <div role="alert" className="mb-4 rounded-xl border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
      {t(errorKey)}
      {errorKey === "loadError" && (
        <Button type="button" variant="outline" size="sm" className="mt-3" onClick={() => void link.load()}>
          {t("retry")}
        </Button>
      )}
    </div>
  );

  // Inside the Mini App, an account linked to the Telegram user who opened it
  // with a verified phone needs no link actions — only the way to undo it.
  if (mode === "linked") {
    return (
      <Card className="border-success/40 bg-card p-5 sm:p-6">
        {/* Body-coloured label: the success green on white is 3.4:1, too
            faint for text; the tick alone carries the colour. */}
        <div role="status" className="flex items-center gap-3">
          <Check aria-hidden="true" className="h-5 w-5 shrink-0 text-success" />
          <div className="min-w-0">
            <p className="font-bold text-foreground">{tApp("linkedStatus")}</p>
            {usernameLabel && <p className="truncate text-xs text-muted-foreground">{usernameLabel}</p>}
          </div>
        </div>
        {errorKey && <div className="mt-4">{errorBox}</div>}
        <div className="mt-3">{unlink}</div>
      </Card>
    );
  }

  return (
    <Card className="border-accent/20 bg-card p-5 sm:p-6">
      <CardHeader className="mb-4 flex flex-row items-center justify-between p-0">
        <div className="flex items-center gap-2">
          <Send aria-hidden="true" className="h-5 w-5 text-accent" />
          <CardTitle className="text-base font-bold">{t("title")}</CardTitle>
        </div>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="min-h-11 gap-1.5"
          onClick={() => void link.load()}
          disabled={loading}
          aria-label={t("refresh")}
        >
          <RefreshCw aria-hidden="true" className={`h-4 w-4 ${loading ? "animate-spin motion-reduce:animate-none" : ""}`} />
          <span className="hidden sm:inline">{t("refresh")}</span>
        </Button>
      </CardHeader>

      {/* The website links through a 10-minute bot link; the Mini App links
          in place, so its hint says nothing about a link expiring. */}
      <p className="mb-4 text-xs text-muted-foreground">{link.inMiniApp ? t("subtitleInApp") : t("subtitle")}</p>

      {loading && !status && (
        <div role="status" className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 aria-hidden="true" className="h-4 w-4 animate-spin motion-reduce:animate-none" />
          {t("loading")}
        </div>
      )}

      {errorBox}

      {!loading && mode === "confirm" && (
        <div role="status" className="mb-4 rounded-xl border border-accent/40 bg-accent/10 p-3 text-sm">
          <p className="font-bold text-foreground">{t("confirmTitle")}</p>
          <p className="mt-0.5 text-xs text-muted-foreground">{tApp("phoneConfirmNote")}</p>
        </div>
      )}

      {!loading && mode === "other" && (
        <div role="status" className="mb-4 rounded-xl border border-accent/40 bg-accent/10 p-3 text-sm">
          <p className="font-bold text-foreground">{t("linkedOtherTitle")}</p>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {usernameLabel ? `${usernameLabel} · ` : ""}
            {t("linkedOtherHint")}
          </p>
        </div>
      )}

      {/* Website only: the Mini App's own states are above. */}
      {!loading && mode === null && status?.linked && (
        <div
          role="status"
          className="mb-4 flex items-start gap-3 rounded-xl border border-success/40 bg-success/10 p-3 text-sm"
        >
          {/* Only the tick is green: green text on the green tint is below
              4.5:1, so the words stay body text (as in the Mini App panel). */}
          <Check aria-hidden="true" className="mt-0.5 h-4 w-4 shrink-0 text-success" />
          <div>
            <p className="font-bold text-foreground">{t("linkedTitle")}</p>
            <p className="mt-0.5 text-xs text-muted-foreground">
              {usernameLabel ? t("linkedAs", { username: usernameLabel }) : t("linkedAnonymous")}
            </p>
          </div>
        </div>
      )}

      {!loading && status && !status.linked && !errorKey && (
        <p className="mb-4 text-sm text-muted-foreground">{t("notLinked")}</p>
      )}

      <div className="flex flex-col gap-2 sm:flex-row">
        {link.inMiniApp ? (
          <Button
            type="button"
            variant="game"
            size="sm"
            className="min-h-11 w-full sm:w-auto"
            disabled={link.linking || loading}
            onClick={() => void link.linkInApp()}
          >
            {link.linking ? (
              <>
                <Loader2 aria-hidden="true" className="mr-2 h-4 w-4 animate-spin motion-reduce:animate-none" />
                {t("linkingInApp")}
              </>
            ) : (
              <>
                <Smartphone aria-hidden="true" className="mr-2 h-4 w-4" />
                {t("linkInApp")}
              </>
            )}
          </Button>
        ) : (
          <Button
            type="button"
            variant="game"
            size="sm"
            className="min-h-11 w-full sm:w-auto"
            disabled={link.linking || loading}
            onClick={() => void link.startDeepLink()}
          >
            {link.linking ? (
              <>
                <Loader2 aria-hidden="true" className="mr-2 h-4 w-4 animate-spin motion-reduce:animate-none" />
                {t("linking")}
              </>
            ) : (
              <>
                <ExternalLink aria-hidden="true" className="mr-2 h-4 w-4" />
                {status?.linked ? t("relinkButton") : t("linkButton")}
              </>
            )}
          </Button>
        )}
      </div>

      {unlink && <div className="mt-3">{unlink}</div>}

      {deepLink && (
        <div className="mt-4 space-y-2 rounded-xl border border-border bg-background/60 p-3">
          <p className="text-xs font-semibold text-muted-foreground">{t("deepLinkHint")}</p>
          {expiresAt && (
            <p className="text-[11px] text-muted-foreground">
              {t("expiresAt", { time: new Date(expiresAt).toLocaleTimeString() })}
            </p>
          )}
          <div className="flex flex-col gap-2 sm:flex-row">
            <a
              href={deepLink}
              target="_blank"
              rel="noopener noreferrer"
              className="min-h-11 flex-1 truncate rounded-lg border border-border bg-card px-3 py-2 text-xs font-medium text-accent underline-offset-2 hover:underline"
            >
              {deepLink}
            </a>
            <Button type="button" variant="outline" size="sm" className="min-h-11" onClick={() => void handleCopy()}>
              {copied ? (
                <>
                  <Check aria-hidden="true" className="mr-2 h-4 w-4" /> {t("copied")}
                </>
              ) : (
                <>
                  <Copy aria-hidden="true" className="mr-2 h-4 w-4" /> {t("copyLink")}
                </>
              )}
            </Button>
          </div>
        </div>
      )}
    </Card>
  );
}
