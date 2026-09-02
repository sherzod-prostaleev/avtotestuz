"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import {
  AlertTriangle,
  ArrowLeft,
  Award,
  BarChart3,
  Bell,
  Bookmark,
  BookOpen,
  Crown,
  LayoutDashboard,
  LifeBuoy,
  Loader2,
  Signpost,
  Swords,
  Target,
  Trophy,
  User,
  type LucideIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  listNotifications,
  markAllNotificationsRead,
  markNotificationRead,
  type NotificationItem,
} from "@/lib/notifications-client";


/**
 * The API returns no `type` or `kind` for a notification — only an optional
 * `url`. Until it does (the artboard's colours are semantic: a repeat prompt is
 * red, a streak nudge orange, a referral payout green), the icon and its tone
 * are read off the destination route, reusing the exact tokens the bottom tab
 * bar and the phone menu already give that route so the two screens agree.
 *
 * Written out in full because Tailwind only emits a class it can read verbatim
 * in the source — the same reason `sidebar.tsx` spells its `tone` field out.
 */
const NOTIFICATION_ICON_BY_SEGMENT: Record<string, { icon: LucideIcon; tone: string }> = {
  dashboard: { icon: LayoutDashboard, tone: "text-accent" },
  tickets: { icon: BookOpen, tone: "text-success" },
  practice: { icon: Target, tone: "text-streak" },
  arena: { icon: Swords, tone: "text-streak" },
  exam: { icon: Award, tone: "text-gold" },
  signs: { icon: Signpost, tone: "text-accent" },
  premium: { icon: Crown, tone: "text-gold" },
  mistakes: { icon: AlertTriangle, tone: "text-danger" },
  saved: { icon: Bookmark, tone: "text-gold" },
  leaderboard: { icon: Trophy, tone: "text-gold" },
  stats: { icon: BarChart3, tone: "text-success" },
  profile: { icon: User, tone: "text-muted-foreground" },
  support: { icon: LifeBuoy, tone: "text-success" },
};

const DEFAULT_NOTIFICATION_ICON = { icon: Bell, tone: "text-muted-foreground" };

/**
 * "10 daq" / "2 soat" / "kecha" / "2 kun". A phone row is read at a glance, so
 * relative time replaces the wide layout's full date. Computed by hand rather
 * than through `Intl.RelativeTimeFormat` for the same reason the leaderboard
 * groups its digits by hand: `uz-Latn` and `uz-Cyrl` are not tags every
 * browser's ICU data resolves.
 */
function formatRelative(iso: string, t: (key: string, values?: Record<string, string | number>) => string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const minutes = Math.floor((Date.now() - then) / 60_000);
  if (minutes < 1) return t("timeJustNow");
  if (minutes < 60) return t("timeMinutesAgo", { count: minutes });
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return t("timeHoursAgo", { count: hours });
  const days = Math.floor(hours / 24);
  if (days < 2) return t("timeYesterday");
  return t("timeDaysAgo", { count: days });
}

function iconForNotification(item: NotificationItem) {
  // `url` is "/{locale}/{segment}/...", so the second part names the route.
  const segment = item.url?.split("/").filter(Boolean)[1];
  return (segment && NOTIFICATION_ICON_BY_SEGMENT[segment]) || DEFAULT_NOTIFICATION_ICON;
}

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
    // Phone padding comes down to the design's 12px: at `px-4` the title and
    // "Barchasini o'qilgan" are three pixels too wide for the row and the
    // heading loses its last syllable to the ellipsis.
    <main className="mx-auto max-w-2xl space-y-4 px-4 py-5 max-md:space-y-3 max-md:px-3 max-md:py-3">
      <div className="flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <Link
            href={`/${locale}/dashboard`}
            className="flex h-10 w-10 items-center justify-center rounded-xl border border-border max-md:h-auto max-md:w-auto max-md:border-0"
            aria-label={t("back")}
          >
            <ArrowLeft className="h-4 w-4 max-md:h-[22px] max-md:w-[22px] max-md:text-muted-foreground" aria-hidden />
          </Link>
          <h1 className="truncate text-lg font-black max-md:text-xl">{t("title")}</h1>
        </div>
        {/* On the phone this is plain accent text, not a bordered button — the
            design gives the row's weight to the title. */}
        <Button
          type="button"
          size="sm"
          variant="outline"
          className="max-md:h-auto max-md:min-h-0 max-md:whitespace-nowrap max-md:border-0 max-md:bg-transparent max-md:p-0 max-md:text-[13px] max-md:font-bold max-md:text-accent max-md:shadow-none max-md:hover:bg-transparent"
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

      <ul className="divide-y divide-border overflow-hidden rounded-2xl border border-border bg-card max-md:hidden">
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

      {/* Phone: icon-led rows. The row shape genuinely differs — an icon in
          front instead of a picture behind, relative time instead of a full
          date — so this is a second body rather than a reflow. */}
      <ul className="divide-y divide-border overflow-hidden rounded-2xl border border-border bg-card md:hidden">
        {items.map((item) => {
          const unread = !item.read_at;
          const { icon: Icon, tone } = iconForNotification(item);
          return (
            <li key={item.id}>
              <button
                type="button"
                disabled={busy}
                onClick={() => void onItemClick(item)}
                className={`flex w-full items-start gap-2.5 px-3.5 py-[11px] text-left hover:bg-accent/10 ${
                  unread ? "bg-accent/5" : ""
                }`}
              >
                <span
                  className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${
                    unread ? "bg-accent" : "bg-transparent"
                  }`}
                  aria-hidden
                />
                <Icon aria-hidden className={`mt-px h-[19px] w-[19px] shrink-0 ${tone}`} />
                <span className="min-w-0 flex-1">
                  <span className={`block text-[15px] leading-tight ${unread ? "font-bold" : "font-semibold"}`}>
                    {item.title}
                  </span>
                  <span className="mt-px block truncate text-[13px] leading-tight text-muted-foreground">
                    {item.body}
                  </span>
                </span>
                <span className="shrink-0 whitespace-nowrap text-xs text-muted-foreground/80">
                  {formatRelative(item.created_at, t)}
                </span>
              </button>
            </li>
          );
        })}
      </ul>

      {items.length >= 30 ? (
        <div className="flex justify-center max-md:block">
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="max-md:min-h-11 max-md:w-full max-md:rounded-xl max-md:text-sm max-md:font-bold"
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
