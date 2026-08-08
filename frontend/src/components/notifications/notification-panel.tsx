"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { Loader2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  listNotifications,
  markAllNotificationsRead,
  markNotificationRead,
  type NotificationItem,
} from "@/lib/notifications-client";

function formatWhen(iso: string, locale: string): string {
  try {
    return new Intl.DateTimeFormat(locale, {
      dateStyle: "medium",
      timeStyle: "short",
    }).format(new Date(iso));
  } catch {
    return iso;
  }
}

type Props = {
  open: boolean;
  onClose: () => void;
  onUnreadChange: (n: number) => void;
  variant: "dropdown" | "sheet";
};

export function NotificationPanel({ open, onClose, onUnreadChange, variant }: Props) {
  const t = useTranslations("Notifications");
  const locale = useLocale();
  const router = useRouter();
  const [items, setItems] = useState<NotificationItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const rows = await listNotifications({ limit: 20 });
      setItems(rows);
      onUnreadChange(rows.filter((r) => !r.read_at).length);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [onUnreadChange]);

  useEffect(() => {
    if (!open) return;
    void load();
  }, [open, load]);

  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  async function onItemClick(item: NotificationItem) {
    setBusy(true);
    try {
      if (!item.read_at) {
        await markNotificationRead(item.id);
        setItems((prev) =>
          prev.map((r) =>
            r.id === item.id ? { ...r, read_at: new Date().toISOString() } : r,
          ),
        );
        onUnreadChange(Math.max(0, items.filter((r) => !r.read_at && r.id !== item.id).length));
      }
      if (item.url?.startsWith("/")) {
        onClose();
        router.push(item.url);
        return;
      }
    } catch {
      /* keep panel open */
    } finally {
      setBusy(false);
    }
  }

  async function onMarkAll() {
    setBusy(true);
    try {
      await markAllNotificationsRead();
      setItems((prev) =>
        prev.map((r) => ({ ...r, read_at: r.read_at || new Date().toISOString() })),
      );
      onUnreadChange(0);
    } catch {
      /* ignore */
    } finally {
      setBusy(false);
    }
  }

  if (!open) return null;

  const panel = (
    <div
      role="dialog"
      aria-label={t("title")}
      className={
        variant === "dropdown"
          ? "absolute right-0 top-full z-50 mt-2 w-[min(24rem,calc(100vw-1.5rem))] overflow-hidden rounded-2xl border border-border bg-card shadow-xl"
          : "fixed inset-x-0 bottom-0 top-[max(3.5rem,env(safe-area-inset-top))] z-50 flex flex-col bg-background md:hidden"
      }
    >
      <div className="flex items-center justify-between gap-2 border-b border-border px-3 py-2.5">
        <h2 className="text-sm font-bold">{t("title")}</h2>
        <div className="flex items-center gap-1">
          <Button
            type="button"
            size="sm"
            variant="ghost"
            disabled={busy || loading}
            onClick={() => void onMarkAll()}
            className="text-xs"
          >
            {t("markAllRead")}
          </Button>
          <button
            type="button"
            onClick={onClose}
            className="flex h-9 w-9 items-center justify-center rounded-lg text-muted-foreground hover:bg-background"
            aria-label={t("close")}
          >
            <X className="h-4 w-4" aria-hidden />
          </button>
        </div>
      </div>

      <div className={`overflow-y-auto ${variant === "dropdown" ? "max-h-[70vh]" : "flex-1"}`}>
        {loading ? (
          <div className="flex items-center justify-center gap-2 p-8 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
            {t("loading")}
          </div>
        ) : null}
        {error ? (
          <div className="space-y-2 p-4 text-center text-sm">
            <p className="text-destructive">{t("error")}</p>
            <Button type="button" size="sm" variant="outline" onClick={() => void load()}>
              {t("retry")}
            </Button>
          </div>
        ) : null}
        {!loading && !error && items.length === 0 ? (
          <p className="p-8 text-center text-sm text-muted-foreground">{t("empty")}</p>
        ) : null}
        <ul className="divide-y divide-border">
          {items.map((item) => {
            const unread = !item.read_at;
            return (
              <li key={item.id}>
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => void onItemClick(item)}
                  className={`flex w-full gap-3 px-3 py-3 text-left transition-colors hover:bg-accent/10 ${
                    unread ? "bg-accent/5" : ""
                  }`}
                >
                  <span
                    className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${
                      unread ? "bg-destructive" : "bg-transparent"
                    }`}
                    aria-hidden
                  />
                  <span className="min-w-0 flex-1">
                    <span className={`block text-sm ${unread ? "font-bold" : "font-medium"}`}>
                      {item.title}
                    </span>
                    <span className="mt-0.5 line-clamp-2 block text-xs text-muted-foreground">
                      {item.body}
                    </span>
                    <span className="mt-1 block text-[11px] text-muted-foreground">
                      {formatWhen(item.created_at, locale)}
                    </span>
                  </span>
                  {item.image_url ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img
                      src={item.image_url}
                      alt=""
                      className="h-12 w-12 shrink-0 rounded-lg object-cover"
                    />
                  ) : null}
                </button>
              </li>
            );
          })}
        </ul>
      </div>

      <div className="border-t border-border p-2">
        <Link
          href={`/${locale}/notifications`}
          onClick={onClose}
          className="block rounded-xl px-3 py-2.5 text-center text-sm font-semibold text-accent hover:bg-accent/10"
        >
          {t("viewAll")}
        </Link>
      </div>
    </div>
  );

  if (variant === "sheet") {
    return (
      <>
        <button
          type="button"
          aria-label={t("close")}
          className="fixed inset-0 z-40 bg-black/50 md:hidden"
          onClick={onClose}
        />
        {panel}
      </>
    );
  }

  return panel;
}
