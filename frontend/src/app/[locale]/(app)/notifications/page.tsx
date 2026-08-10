"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { ArrowLeft, Loader2 } from "lucide-react";
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

export default function NotificationsPage() {
  const t = useTranslations("Notifications");
  const locale = useLocale();
  const router = useRouter();
  const [items, setItems] = useState<NotificationItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState(false);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async (before?: string) => {
    if (before) setLoadingMore(true);
    else setLoading(true);
    setError(false);
    try {
      const rows = await listNotifications({ before, limit: 30 });
      setItems((prev) => (before ? [...prev, ...rows] : rows));
    } catch {
      setError(true);
    } finally {
      setLoading(false);
      setLoadingMore(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

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
      }
      if (item.url?.startsWith("/")) {
        router.push(item.url);
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="mx-auto max-w-2xl space-y-4 px-4 py-5">
      <div className="flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <Link
            href={`/${locale}/dashboard`}
            className="flex h-10 w-10 items-center justify-center rounded-xl border border-border"
            aria-label={t("back")}
          >
            <ArrowLeft className="h-4 w-4" aria-hidden />
          </Link>
          <h1 className="truncate text-lg font-black">{t("title")}</h1>
        </div>
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={busy || loading}
          onClick={() => {
            setBusy(true);
            void markAllNotificationsRead()
              .then(() =>
                setItems((prev) =>
                  prev.map((r) => ({
                    ...r,
                    read_at: r.read_at || new Date().toISOString(),
                  })),
                ),
              )
              .finally(() => setBusy(false));
          }}
        >
          {t("markAllRead")}
        </Button>
      </div>

      {loading ? (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
          {t("loading")}
        </div>
      ) : null}
      {error ? <p className="text-sm text-destructive">{t("error")}</p> : null}
      {!loading && !error && items.length === 0 ? (
        <p className="rounded-2xl border border-dashed border-border p-8 text-center text-sm text-muted-foreground">
          {t("empty")}
        </p>
      ) : null}

      <ul className="divide-y divide-border overflow-hidden rounded-2xl border border-border bg-card">
        {items.map((item) => {
          const unread = !item.read_at;
          return (
            <li key={item.id}>
              <button
                type="button"
                disabled={busy}
                onClick={() => void onItemClick(item)}
                className={`flex w-full gap-3 px-4 py-3.5 text-left hover:bg-accent/10 ${
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
                  <span className="mt-0.5 block whitespace-pre-wrap break-words text-sm leading-relaxed text-muted-foreground">
                    {item.body}
                  </span>
                  <span className="mt-1 block text-xs text-muted-foreground">
                    {formatWhen(item.created_at, locale)}
                  </span>
                </span>
                {item.image_url ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img
                    src={item.image_url}
                    alt=""
                    className="h-14 w-14 shrink-0 rounded-xl object-cover"
                  />
                ) : null}
              </button>
            </li>
          );
        })}
      </ul>

      {items.length >= 30 ? (
        <div className="flex justify-center">
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={loadingMore}
            onClick={() => {
              const last = items[items.length - 1];
              if (last) void load(last.created_at);
            }}
          >
            {loadingMore ? <Loader2 className="mr-2 h-4 w-4 animate-spin" aria-hidden /> : null}
            {t("loadMore")}
          </Button>
        </div>
      ) : null}
    </main>
  );
}
