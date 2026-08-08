"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { Bell } from "lucide-react";
import { getNotificationUnreadCount } from "@/lib/notifications-client";
import { NotificationPanel } from "@/components/notifications/notification-panel";

type Props = {
  /** Mobile top-bar uses sheet; desktop sidebar uses dropdown. */
  variant: "mobile" | "desktop";
};

export function NotificationBell({ variant }: Props) {
  const t = useTranslations("Notifications");
  const pathname = usePathname();
  const [unread, setUnread] = useState(0);
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  const refresh = useCallback(async () => {
    try {
      const n = await getNotificationUnreadCount();
      setUnread(n);
    } catch {
      /* badge is best-effort */
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh, pathname]);

  useEffect(() => {
    const id = window.setInterval(() => void refresh(), 60_000);
    return () => window.clearInterval(id);
  }, [refresh]);

  useEffect(() => {
    if (!open || variant !== "desktop") return;
    function onDoc(e: MouseEvent) {
      if (!rootRef.current?.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open, variant]);

  const label =
    unread > 0 ? t("bellWithCount", { count: unread > 99 ? 99 : unread }) : t("bell");

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        aria-label={label}
        aria-expanded={open}
        aria-haspopup="dialog"
        onClick={() => setOpen((v) => !v)}
        className="relative flex h-10 w-10 items-center justify-center rounded-xl border border-border bg-card text-foreground transition-colors hover:bg-accent/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <Bell aria-hidden className="h-4 w-4" />
        {unread > 0 ? (
          <span
            className="absolute -right-1 -top-1 flex h-5 min-w-5 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-bold text-destructive-foreground tabular-nums"
            aria-live="polite"
          >
            {unread > 99 ? "99+" : unread}
          </span>
        ) : null}
      </button>
      <NotificationPanel
        open={open}
        onClose={() => setOpen(false)}
        onUnreadChange={setUnread}
        variant={variant === "mobile" ? "sheet" : "dropdown"}
      />
    </div>
  );
}
