"use client";

import { createSharedCount } from "@/lib/shared-count";
import { getNotificationUnreadCount } from "@/lib/notifications-client";
import { getMySupportUnread } from "@/lib/support-chat-client";

/**
 * The two chrome badges, each fetched once no matter how many components show
 * it. Both are read by the sidebar, which mounts the notification bell twice
 * (mobile top bar + desktop rail).
 *
 * The TTL is what stops a page change from refetching: navigating between
 * dashboard, tickets and practice inside half a minute now costs nothing.
 */

/** Kept on a timer — a notification can arrive while the learner sits still. */
export const notificationUnreadCount = createSharedCount(getNotificationUnreadCount, {
  ttlMs: 30_000,
  pollMs: 60_000,
});

/** Navigation-driven only, as before: support replies are not time-critical. */
export const supportUnreadCount = createSharedCount(
  async () => (await getMySupportUnread()).unread ?? 0,
  { ttlMs: 30_000 },
);
