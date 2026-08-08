import { apiGet, apiPost } from "@/lib/api-client";

export type NotificationItem = {
  id: string;
  title: string;
  body: string;
  image_url?: string;
  url?: string;
  read_at?: string | null;
  created_at: string;
  campaign_id?: string;
};

export async function getNotificationUnreadCount(): Promise<number> {
  const data = await apiGet<{ unread: number }>("me/notifications/unread-count");
  return data.unread ?? 0;
}

export async function listNotifications(opts?: {
  before?: string;
  limit?: number;
}): Promise<NotificationItem[]> {
  const params = new URLSearchParams();
  if (opts?.before) params.set("before", opts.before);
  if (opts?.limit) params.set("limit", String(opts.limit));
  const q = params.toString();
  const data = await apiGet<{ items: NotificationItem[] }>(
    `me/notifications${q ? `?${q}` : ""}`,
  );
  return data.items ?? [];
}

export async function markNotificationRead(id: string): Promise<NotificationItem> {
  return apiPost<NotificationItem>(`me/notifications/${id}/read`);
}

export async function markAllNotificationsRead(): Promise<number> {
  const data = await apiPost<{ marked: number }>("me/notifications/read-all");
  return data.marked ?? 0;
}
