"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { apiGet, apiPost, ApiError } from "@/lib/api-client";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Check, Copy, ExternalLink, Loader2, RefreshCw, Send } from "lucide-react";

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

export function TelegramLinkCard() {
  const t = useTranslations("TelegramLink");
  const [status, setStatus] = useState<TelegramStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [linking, setLinking] = useState(false);
  const [deepLink, setDeepLink] = useState<string | null>(null);
  const [expiresAt, setExpiresAt] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [errorKey, setErrorKey] = useState<"loadError" | "linkError" | "unconfigured" | null>(null);

  const loadStatus = useCallback(async () => {
    setLoading(true);
    setErrorKey(null);
    try {
      const data = await apiGet<TelegramStatus>("me/telegram");
      setStatus(data);
    } catch {
      setErrorKey("loadError");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadStatus();
  }, [loadStatus]);

  const handleLink = async () => {
    setLinking(true);
    setErrorKey(null);
    setCopied(false);
    try {
      const result = await apiPost<LinkTokenResult>("me/telegram/link-token");
      setDeepLink(result.deep_link);
      setExpiresAt(result.expires_at);
      window.open(result.deep_link, "_blank", "noopener,noreferrer");
    } catch (err) {
      if (err instanceof ApiError && err.code === "telegram_bot_unconfigured") {
        setErrorKey("unconfigured");
      } else {
        setErrorKey("linkError");
      }
    } finally {
      setLinking(false);
    }
  };

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
          onClick={() => void loadStatus()}
          disabled={loading}
          aria-label={t("refresh")}
        >
          <RefreshCw aria-hidden="true" className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
          <span className="hidden sm:inline">{t("refresh")}</span>
        </Button>
      </CardHeader>

      <p className="mb-4 text-xs text-muted-foreground">{t("subtitle")}</p>

      {loading && !status && (
        <div role="status" className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 aria-hidden="true" className="h-4 w-4 animate-spin" />
          {t("loading")}
        </div>
      )}

      {errorKey && (
        <div role="alert" className="mb-4 rounded-xl border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
          {t(errorKey)}
          {errorKey === "loadError" && (
            <Button type="button" variant="outline" size="sm" className="mt-3" onClick={() => void loadStatus()}>
              {t("retry")}
            </Button>
          )}
        </div>
      )}

      {!loading && status?.linked && (
        <div
          role="status"
          className="mb-4 flex items-start gap-3 rounded-xl border border-success/40 bg-success/10 p-3 text-sm text-success"
        >
          <Check aria-hidden="true" className="mt-0.5 h-4 w-4 shrink-0" />
          <div>
            <p className="font-bold">{t("linkedTitle")}</p>
            <p className="mt-0.5 text-xs text-success/90">
              {usernameLabel ? t("linkedAs", { username: usernameLabel }) : t("linkedAnonymous")}
            </p>
          </div>
        </div>
      )}

      {!loading && status && !status.linked && !errorKey && (
        <p className="mb-4 text-sm text-muted-foreground">{t("notLinked")}</p>
      )}

      <div className="flex flex-col gap-2 sm:flex-row">
        <Button
          type="button"
          variant="game"
          size="sm"
          className="min-h-11 w-full sm:w-auto"
          disabled={linking || loading}
          onClick={() => void handleLink()}
        >
          {linking ? (
            <>
              <Loader2 aria-hidden="true" className="mr-2 h-4 w-4 animate-spin" />
              {t("linking")}
            </>
          ) : (
            <>
              <ExternalLink aria-hidden="true" className="mr-2 h-4 w-4" />
              {status?.linked ? t("relinkButton") : t("linkButton")}
            </>
          )}
        </Button>
      </div>

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
