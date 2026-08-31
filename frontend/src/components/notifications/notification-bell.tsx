"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { Bell } from "lucide-react";
import { notificationUnreadCount } from "@/lib/badge-counts";
import { useSharedCount } from "@/lib/shared-count";
import { NotificationPanel } from "@/components/notifications/notification-panel";

type Props = {
  /** Mobile top-bar uses sheet; desktop sidebar uses popover. */
  variant: "mobile" | "desktop";
};

type AnchorRect = {
  top: number;
  bottom: number;
  left: number;
  right: number;
  width: number;
  height: number;
};

export function NotificationBell({ variant }: Props) {
  const t = useTranslations("Notifications");
  const pathname = usePathname();
  // Shared with the other bell: this component is mounted twice (mobile top
  // bar and desktop rail) and CSS hides one, so a private fetch per instance
  // doubled the platform's most-requested endpoint on every navigation.
  const unread = useSharedCount(notificationUnreadCount, pathname);
  const [open, setOpen] = useState(false);
  const [anchorRect, setAnchorRect] = useState<AnchorRect | null>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  const measure = useCallback(() => {
    const el = buttonRef.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    setAnchorRect({
      top: r.top,
      bottom: r.bottom,
      left: r.left,
      right: r.right,
      width: r.width,
      height: r.height,
    });
  }, []);

  useEffect(() => {
    setOpen(false);
  }, [pathname]);

  useEffect(() => {
    if (!open) return;
    measure();
    function onWin() {
      measure();
    }
    window.addEventListener("resize", onWin);
    window.addEventListener("scroll", onWin, true);
    return () => {
      window.removeEventListener("resize", onWin);
      window.removeEventListener("scroll", onWin, true);
    };
  }, [open, measure]);

  const label =
    unread > 0 ? t("bellWithCount", { count: unread > 99 ? 99 : unread }) : t("bell");

  return (
    <>
      <button
        ref={buttonRef}
        type="button"
        aria-label={label}
        aria-expanded={open}
        aria-haspopup="dialog"
        onClick={() => {
          measure();
          setOpen((v) => !v);
        }}
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
        // Publish into the shared store so the other bell updates too.
        onUnreadChange={(next) => notificationUnreadCount.set(next)}
        variant={variant === "mobile" ? "sheet" : "dropdown"}
        anchorRect={anchorRect}
      />
    </>
  );
}
