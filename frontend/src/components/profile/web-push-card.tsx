"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { apiDelete, apiGet, apiPost, ApiError } from "@/lib/api-client";
import {
  ensurePushServiceWorker,
  pushSupported,
  subscriptionToJSON,
  urlBase64ToUint8Array,
} from "@/lib/web-push";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Bell, BellOff, Loader2, RefreshCw, Send } from "lucide-react";

interface PushStatus {
  configured: boolean;
  subscribed: boolean;
  subscription_count: number;
  vapid_public_key?: string;
}

type ErrorKey =
  | "loadError"
  | "enableError"
  | "disableError"
  | "unconfigured"
  | "unsupported"
  | "denied"
  | "testError"
  | null;

export function WebPushCard() {
  const t = useTranslations("WebPush");
  const [status, setStatus] = useState<PushStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [localEndpoint, setLocalEndpoint] = useState<string | null>(null);
  const [errorKey, setErrorKey] = useState<ErrorKey>(null);
  const [testOk, setTestOk] = useState(false);

  const refreshLocalSub = useCallback(async () => {
    if (!pushSupported()) {
      setLocalEndpoint(null);
      return;
    }
    try {
      const reg = await navigator.serviceWorker.getRegistration("/");
      const sub = await reg?.pushManager.getSubscription();
      setLocalEndpoint(sub?.endpoint ?? null);
    } catch {
      setLocalEndpoint(null);
    }
  }, []);

  const loadStatus = useCallback(async () => {
    setLoading(true);
    setErrorKey(null);
    setTestOk(false);
    try {
      if (!pushSupported()) {
        setErrorKey("unsupported");
        setStatus(null);
        return;
      }
      const data = await apiGet<PushStatus>("me/push");
      setStatus(data);
      await refreshLocalSub();
    } catch {
      setErrorKey("loadError");
    } finally {
      setLoading(false);
    }
  }, [refreshLocalSub]);

  useEffect(() => {
    void loadStatus();
  }, [loadStatus]);

  const handleEnable = async () => {
    setBusy(true);
    setErrorKey(null);
    setTestOk(false);
    try {
      if (!pushSupported()) {
        setErrorKey("unsupported");
        return;
      }
      const current = status ?? (await apiGet<PushStatus>("me/push"));
      setStatus(current);
      if (!current.configured || !current.vapid_public_key) {
        setErrorKey("unconfigured");
        return;
      }
      const permission = await Notification.requestPermission();
      if (permission !== "granted") {
        setErrorKey("denied");
        return;
      }
      const reg = await ensurePushServiceWorker();
      let sub = await reg.pushManager.getSubscription();
      if (!sub) {
        const keyBytes = urlBase64ToUint8Array(current.vapid_public_key);
        sub = await reg.pushManager.subscribe({
          userVisibleOnly: true,
          // TS DOM lib + Uint8Array generic mismatch on ArrayBufferLike
          applicationServerKey: keyBytes.buffer.slice(
            keyBytes.byteOffset,
            keyBytes.byteOffset + keyBytes.byteLength
          ) as ArrayBuffer,
        });
      }
      const json = subscriptionToJSON(sub);
      await apiPost("me/push/subscribe", {
        endpoint: json.endpoint,
        keys: json.keys,
        user_agent: typeof navigator !== "undefined" ? navigator.userAgent : "",
      });
      setLocalEndpoint(json.endpoint);
      await loadStatus();
    } catch (err) {
      if (err instanceof ApiError && err.code === "web_push_unconfigured") {
        setErrorKey("unconfigured");
      } else {
        setErrorKey("enableError");
      }
    } finally {
      setBusy(false);
    }
  };

  const handleDisable = async () => {
    setBusy(true);
    setErrorKey(null);
    setTestOk(false);
    try {
      const reg = await navigator.serviceWorker.getRegistration("/");
      const sub = await reg?.pushManager.getSubscription();
      const endpoint = sub?.endpoint ?? localEndpoint;
      if (sub) {
        await sub.unsubscribe();
      }
      if (endpoint) {
        await apiDelete("me/push/subscribe", { endpoint });
      }
      setLocalEndpoint(null);
      await loadStatus();
    } catch {
      setErrorKey("disableError");
    } finally {
      setBusy(false);
    }
  };

  const handleTest = async () => {
    setBusy(true);
    setErrorKey(null);
    setTestOk(false);
    try {
      await apiPost("me/push/test");
      setTestOk(true);
    } catch {
      setErrorKey("testError");
    } finally {
      setBusy(false);
    }
  };

  const enabledHere = Boolean(localEndpoint);

  return (
    <Card className="border-accent/20 bg-card p-5 sm:p-6">
      <CardHeader className="mb-4 flex flex-row items-center justify-between p-0">
        <div className="flex items-center gap-2">
          <Bell aria-hidden="true" className="h-5 w-5 text-accent" />
          <CardTitle className="text-base font-bold">{t("title")}</CardTitle>
        </div>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="min-h-11 gap-1.5"
          onClick={() => void loadStatus()}
          disabled={loading || busy}
          aria-label={t("refresh")}
        >
          <RefreshCw aria-hidden="true" className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
          <span className="hidden sm:inline">{t("refresh")}</span>
        </Button>
      </CardHeader>

      <p className="mb-4 text-sm text-muted-foreground">{t("subtitle")}</p>

      {loading && (
        <div role="status" className="mb-3 flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 aria-hidden="true" className="h-4 w-4 animate-spin" />
          {t("loading")}
        </div>
      )}

      {errorKey && (
        <div role="alert" className="mb-3 rounded-xl border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
          {t(errorKey)}
        </div>
      )}

      {testOk && (
        <div role="status" className="mb-3 rounded-xl border border-success/50 bg-success/10 p-3 text-sm font-medium text-success">
          {t("testSent")}
        </div>
      )}

      {!loading && status && (
        <div className="mb-4 space-y-1 text-sm">
          <p className="font-medium">
            {enabledHere ? t("enabledHere") : status.subscribed ? t("enabledElsewhere") : t("notEnabled")}
          </p>
          {status.subscription_count > 0 && (
            <p className="text-xs text-muted-foreground">
              {t("deviceCount", { count: status.subscription_count })}
            </p>
          )}
        </div>
      )}

      <div className="flex flex-col gap-2 sm:flex-row">
        {!enabledHere ? (
          <Button
            type="button"
            variant="game"
            className="min-h-11 w-full sm:w-auto"
            disabled={loading || busy || errorKey === "unsupported"}
            onClick={() => void handleEnable()}
          >
            {busy ? <Loader2 aria-hidden="true" className="mr-2 h-4 w-4 animate-spin" /> : <Bell aria-hidden="true" className="mr-2 h-4 w-4" />}
            {t("enable")}
          </Button>
        ) : (
          <>
            <Button
              type="button"
              variant="outline"
              className="min-h-11 w-full sm:w-auto"
              disabled={loading || busy}
              onClick={() => void handleDisable()}
            >
              <BellOff aria-hidden="true" className="mr-2 h-4 w-4" />
              {t("disable")}
            </Button>
            <Button
              type="button"
              variant="game"
              className="min-h-11 w-full sm:w-auto"
              disabled={loading || busy}
              onClick={() => void handleTest()}
            >
              <Send aria-hidden="true" className="mr-2 h-4 w-4" />
              {t("test")}
            </Button>
          </>
        )}
      </div>
    </Card>
  );
}
