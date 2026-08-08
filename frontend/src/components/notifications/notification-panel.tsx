"use client";

import { useCallback, useEffect, useLayoutEffect, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { Bell, Loader2, X } from "lucide-react";
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

type AnchorRect = {
  top: number;
  bottom: number;
  left: number;
  right: number;
  width: number;
  height: number;
};

type Props = {
  open: boolean;
  onClose: () => void;
  onUnreadChange: (n: number) => void;
  variant: "dropdown" | "sheet";
  /** Bell button rect — used to park the desktop popover outside the sidebar. */
  anchorRect: AnchorRect | null;
};

const PANEL_Z = 80;

export function NotificationPanel({
  open,
  onClose,
  onUnreadChange,
  variant,
  anchorRect,
}: Props) {
  const t = useTranslations("Notifications");
  const locale = useLocale();
  const router = useRouter();
  const [mounted, setMounted] = useState(false);
  const [items, setItems] = useState<NotificationItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);
  const [busy, setBusy] = useState(false);
  const [desktopPos, setDesktopPos] = useState<{ top: number; left: number } | null>(null);

  useEffect(() => {
    setMounted(true);
  }, []);

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

  // Lock page scroll while the mobile sheet is open.
  useEffect(() => {
    if (!open || variant !== "sheet") return;
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = prev;
    };
  }, [open, variant]);

  useLayoutEffect(() => {
    if (!open || variant !== "dropdown" || !anchorRect) {
      setDesktopPos(null);
      return;
    }
    const width = Math.min(360, window.innerWidth - 24);
    const gap = 8;
    let left = anchorRect.right + gap;
    if (left + width > window.innerWidth - 12) {
      left = Math.max(12, anchorRect.left - width - gap);
    }
    let top = anchorRect.top;
    const maxH = Math.min(window.innerHeight * 0.75, 520);
    if (top + maxH > window.innerHeight - 12) {
      top = Math.max(12, window.innerHeight - maxH - 12);
    }
    setDesktopPos({ top, left });
  }, [open, variant, anchorRect]);

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

  if (!open || !mounted) return null;

  const listBody = (
    <>
      {loading ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-2 px-6 py-16 text-sm text-muted-foreground">
          <Loader2 className="h-5 w-5 animate-spin text-accent" aria-hidden />
          {t("loading")}
        </div>
      ) : null}
      {error ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 px-6 py-16 text-center">
          <p className="text-sm text-destructive">{t("error")}</p>
          <Button type="button" size="sm" variant="outline" onClick={() => void load()}>
            {t("retry")}
          </Button>
        </div>
      ) : null}
      {!loading && !error && items.length === 0 ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 px-6 py-16 text-center">
          <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
            <Bell className="h-5 w-5" aria-hidden />
          </span>
          <p className="max-w-[16rem] text-sm text-muted-foreground">{t("empty")}</p>
        </div>
      ) : null}
      {!loading && !error && items.length > 0 ? (
        <ul className="divide-y divide-border">
          {items.map((item) => {
            const unread = !item.read_at;
            return (
              <li key={item.id}>
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => void onItemClick(item)}
                  className={`flex w-full gap-3 px-4 py-3.5 text-left transition-colors hover:bg-accent/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring ${
                    unread ? "bg-accent/[0.07]" : ""
                  }`}
                >
                  <span
                    className={`mt-2 h-2 w-2 shrink-0 rounded-full ${
                      unread ? "bg-accent" : "bg-transparent"
                    }`}
                    aria-hidden
                  />
                  <span className="min-w-0 flex-1">
                    <span className={`block text-sm leading-snug ${unread ? "font-bold" : "font-medium"}`}>
                      {item.title}
                    </span>
                    <span className="mt-1 line-clamp-2 block text-xs leading-relaxed text-muted-foreground">
                      {item.body}
                    </span>
                    <span className="mt-1.5 block text-[11px] tabular-nums text-muted-foreground/80">
                      {formatWhen(item.created_at, locale)}
                    </span>
                  </span>
                  {item.image_url ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img
                      src={item.image_url}
                      alt=""
                      className="h-12 w-12 shrink-0 rounded-xl object-cover ring-1 ring-border"
                    />
                  ) : null}
                </button>
              </li>
            );
          })}
        </ul>
      ) : null}
    </>
  );

  const header = (
    <div className="flex shrink-0 items-center justify-between gap-2 border-b border-border px-4 py-3">
      <h2 className="font-display text-base font-bold tracking-tight">{t("title")}</h2>
      <div className="flex items-center gap-0.5">
        <Button
          type="button"
          size="sm"
          variant="ghost"
          disabled={busy || loading || items.length === 0}
          onClick={() => void onMarkAll()}
          className="h-9 px-2.5 text-xs font-semibold"
        >
          {t("markAllRead")}
        </Button>
        <button
          type="button"
          onClick={onClose}
          className="flex h-9 w-9 items-center justify-center rounded-xl text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={t("close")}
        >
          <X className="h-4 w-4" aria-hidden />
        </button>
      </div>
    </div>
  );

  const footer = (
    <div className="shrink-0 border-t border-border p-3">
      <Link
        href={`/${locale}/notifications`}
        onClick={onClose}
        className="flex h-11 items-center justify-center rounded-xl bg-accent/10 text-sm font-bold text-accent transition-colors hover:bg-accent/15 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        {t("viewAll")}
      </Link>
    </div>
  );

  let ui: ReactNode = null;

  if (variant === "sheet") {
    ui = (
      <div className="fixed inset-0" style={{ zIndex: PANEL_Z }} data-notification-overlay>
        <button
          type="button"
          aria-label={t("close")}
          className="absolute inset-0 bg-black/55 backdrop-blur-[2px]"
          onClick={onClose}
        />
        <div
          role="dialog"
          aria-modal="true"
          aria-label={t("title")}
          className="absolute inset-x-0 bottom-0 flex max-h-[min(92dvh,40rem)] flex-col overflow-hidden rounded-t-3xl border border-border bg-card shadow-[0_-12px_40px_-12px_rgba(0,0,0,0.45)]"
          style={{ paddingBottom: "max(0.5rem, env(safe-area-inset-bottom))" }}
        >
          <div className="flex justify-center pb-1 pt-3" aria-hidden>
            <span className="h-1.5 w-10 rounded-full bg-border" />
          </div>
          {header}
          <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain">{listBody}</div>
          {footer}
        </div>
      </div>
    );
  } else if (desktopPos) {
    ui = (
      <div className="fixed inset-0" style={{ zIndex: PANEL_Z }} data-notification-overlay>
        <button
          type="button"
          aria-label={t("close")}
          className="absolute inset-0 bg-transparent"
          onClick={onClose}
        />
        <div
          role="dialog"
          aria-modal="true"
          aria-label={t("title")}
          className="absolute flex max-h-[min(75vh,32rem)] w-[min(22.5rem,calc(100vw-1.5rem))] flex-col overflow-hidden rounded-2xl border border-border bg-card shadow-2xl ring-1 ring-black/5"
          style={{ top: desktopPos.top, left: desktopPos.left }}
        >
          {header}
          <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain">{listBody}</div>
          {footer}
        </div>
      </div>
    );
  }

  return createPortal(ui, document.body);
}
