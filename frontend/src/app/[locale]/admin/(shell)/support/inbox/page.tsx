"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { UserRound } from "lucide-react";
import { PermissionGate } from "@/components/admin/permission-gate";
import { AdminPageHeader } from "@/components/admin/admin-page-header";
import { AdminErrorState } from "@/components/admin/admin-error-state";
import { ChatComposer } from "@/components/support-chat/chat-composer";
import {
  ChatMessageList,
  adminAttachmentUrl,
} from "@/components/support-chat/chat-message-list";
import { TemporaryPasswordButton } from "@/components/support-chat/temporary-password-button";
import { resolveArenaWsUrl } from "@/lib/arena-client";
import type { SupportConversation, SupportMessage } from "@/lib/support-chat-client";

type Learner = {
  id: string;
  phone: string;
  name: string;
  status: string;
  vip_active: boolean;
  vip_ends_at?: string;
  has_password: boolean;
  streak: number;
  locale_pref: string;
  created_at: string;
};

type Detail = {
  conversation: SupportConversation;
  learner: Learner;
  items: SupportMessage[];
};

export default function AdminSupportInboxPage() {
  const t = useTranslations("AdminSupport");
  const [q, setQ] = useState("");
  const [status, setStatus] = useState("");
  const [list, setList] = useState<SupportConversation[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [detail, setDetail] = useState<Detail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [mobilePane, setMobilePane] = useState<"list" | "chat" | "user">("list");
  const [userOpen, setUserOpen] = useState(true);

  const loadList = useCallback(async () => {
    const params = new URLSearchParams();
    if (q.trim()) params.set("q", q.trim());
    if (status) params.set("status", status);
    const res = await fetch(`/api/admin/support/conversations?${params}`, { cache: "no-store" });
    if (!res.ok) throw new Error("list failed");
    const json = await res.json();
    setList(json.data?.items ?? json.items ?? []);
  }, [q, status]);

  const loadDetail = useCallback(async (id: string) => {
    const res = await fetch(`/api/admin/support/conversations/${id}`, { cache: "no-store" });
    if (!res.ok) throw new Error("detail failed");
    const json = await res.json();
    const data = json.data ?? json;
    setDetail({
      conversation: data.conversation,
      learner: data.learner,
      items: data.items ?? [],
    });
  }, []);

  useEffect(() => {
    loadList().catch((e) => setError(String(e)));
  }, [loadList]);

  useEffect(() => {
    if (!selectedId) return;
    loadDetail(selectedId).catch((e) => setError(String(e)));
  }, [selectedId, loadDetail]);

  // Realtime fanout for inbox + open thread (auto-reconnect).
  useEffect(() => {
    let ws: WebSocket | null = null;
    let cancelled = false;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let attempt = 0;

    const handleEnvelope = (env: { t: string; d: Record<string, unknown> }) => {
      if (env.t === "message.new" || env.t === "conversation.updated") {
        void loadList();
        const conv = (env.d.conversation || {}) as SupportConversation;
        if (selectedId && conv.id === selectedId) {
          void loadDetail(selectedId);
        } else if (env.t === "message.new" && selectedId) {
          const msg = env.d.message as SupportMessage | undefined;
          if (msg?.conversation_id === selectedId) {
            setDetail((prev) =>
              prev
                ? {
                    ...prev,
                    items: prev.items.some((m) => m.id === msg.id)
                      ? prev.items
                      : [msg, ...prev.items],
                    conversation: (env.d.conversation as SupportConversation) || prev.conversation,
                  }
                : prev,
            );
          }
        }
      }
    };

    const connect = async () => {
      try {
        const res = await fetch("/api/admin/support/ws-ticket", { method: "POST" });
        if (!res.ok || cancelled) return;
        const json = await res.json();
        const data = json.data ?? json;
        const url = new URL(resolveArenaWsUrl(data.ws_url));
        url.searchParams.set("ticket", data.ticket);
        if (cancelled) return;
        ws = new WebSocket(url.toString());
        ws.onmessage = (ev) => {
          try {
            handleEnvelope(JSON.parse(String(ev.data)) as { t: string; d: Record<string, unknown> });
          } catch {
            /* ignore */
          }
        };
        ws.onopen = () => {
          attempt = 0;
          if (selectedId) {
            ws?.send(JSON.stringify({ t: "subscribe", d: { conversation_id: selectedId } }));
          }
        };
        ws.onclose = () => {
          if (cancelled) return;
          const delay = Math.min(15_000, 800 * 2 ** Math.min(attempt, 4));
          attempt += 1;
          reconnectTimer = setTimeout(() => {
            void connect();
          }, delay);
        };
      } catch {
        if (!cancelled) {
          reconnectTimer = setTimeout(() => {
            void connect();
          }, 2000);
        }
      }
    };

    void connect();
    return () => {
      cancelled = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      ws?.close();
    };
  }, [selectedId, loadList, loadDetail]);

  async function send(text: string, file: File | null) {
    if (!selectedId) return;
    let attachment:
      | {
          attachment_key: string;
          attachment_name: string;
          attachment_mime: string;
          attachment_size: number;
        }
      | undefined;
    if (file) {
      const form = new FormData();
      form.append("file", file);
      const upRes = await fetch(`/api/admin/support/conversations/${selectedId}/attachments`, {
        method: "POST",
        body: form,
      });
      if (!upRes.ok) throw new Error("upload failed");
      const upJson = await upRes.json();
      const up = upJson.data ?? upJson;
      attachment = {
        attachment_key: up.key,
        attachment_name: up.name,
        attachment_mime: up.mime,
        attachment_size: up.size,
      };
    }
    const res = await fetch(`/api/admin/support/conversations/${selectedId}/messages`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ body: text, ...attachment }),
    });
    if (!res.ok) throw new Error("send failed");
    const json = await res.json();
    const data = json.data ?? json;
    setDetail((prev) =>
      prev
        ? {
            ...prev,
            conversation: data.conversation ?? prev.conversation,
            items: [data.message, ...prev.items.filter((m) => m.id !== data.message.id)],
          }
        : prev,
    );
    void loadList();
  }

  const patchStatus = useCallback(
    async (next: "open" | "closed") => {
      if (!selectedId) return;
      const res = await fetch(`/api/admin/support/conversations/${selectedId}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ status: next }),
      });
      if (!res.ok) return;
      const json = await res.json();
      const conv = json.data ?? json;
      setDetail((prev) => (prev ? { ...prev, conversation: conv } : prev));
      void loadList();
    },
    [selectedId, loadList],
  );

  const userPanel = useMemo(() => {
    if (!detail) return null;
    const u = detail.learner;
    return (
      <div className="flex h-full flex-col gap-3 overflow-y-auto p-4">
        <h2 className="font-display text-base font-bold">{t("userDetails")}</h2>
        <div>
          <div className="text-xs text-muted-foreground">{t("phone")}</div>
          <div className="font-medium">{u.phone}</div>
        </div>
        <div>
          <div className="text-xs text-muted-foreground">{t("name")}</div>
          <div className="font-medium">{u.name || "—"}</div>
        </div>
        <div>
          <div className="text-xs text-muted-foreground">{t("status")}</div>
          <div className="font-medium">{u.status}</div>
        </div>
        <div>
          <div className="text-xs text-muted-foreground">{t("vip")}</div>
          <div className="font-medium">{u.vip_active ? "VIP" : "—"}</div>
        </div>
        <div>
          <div className="text-xs text-muted-foreground">{t("streak")}</div>
          <div className="font-medium">{u.streak}</div>
        </div>
        <div>
          <div className="mb-1 text-xs text-muted-foreground">{t("passwordHelp")}</div>
          <TemporaryPasswordButton
            userId={u.id}
            issueLabel={t("issueTempPassword")}
            copyLabel={t("copy")}
            copiedLabel={t("copied")}
            issuedHint={t("tempPasswordHint")}
            errorLabel={t("tempPasswordError")}
          />
        </div>
        <button
          type="button"
          className="mt-auto rounded-lg border border-border px-3 py-2 text-sm font-semibold"
          onClick={() =>
            void patchStatus(detail.conversation.status === "closed" ? "open" : "closed")
          }
        >
          {detail.conversation.status === "closed" ? t("reopen") : t("close")}
        </button>
      </div>
    );
  }, [detail, t, patchStatus]);

  return (
    <PermissionGate permission="support.inbox">
      <div className="flex h-[calc(100dvh-5rem)] flex-col gap-3">
        <AdminPageHeader title={t("title")} />
        {error ? <AdminErrorState message={error} /> : null}

        <div className="grid min-h-0 flex-1 overflow-hidden rounded-xl border border-border bg-card md:grid-cols-[18rem_1fr_16rem]">
          {/* List */}
          <aside
            className={`flex min-h-0 flex-col border-r border-border ${
              mobilePane === "list" ? "flex" : "hidden md:flex"
            }`}
          >
            <div className="space-y-2 border-b border-border p-3">
              <input
                value={q}
                onChange={(e) => setQ(e.target.value)}
                placeholder={t("search")}
                className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm"
              />
              <select
                value={status}
                onChange={(e) => setStatus(e.target.value)}
                className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm"
              >
                <option value="">{t("statusAll")}</option>
                <option value="open">{t("statusOpen")}</option>
                <option value="closed">{t("statusClosed")}</option>
              </select>
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto">
              {list.length === 0 ? (
                <div className="p-4 text-sm text-muted-foreground">{t("emptyList")}</div>
              ) : (
                list.map((c) => (
                  <button
                    key={c.id}
                    type="button"
                    onClick={() => {
                      setSelectedId(c.id);
                      setMobilePane("chat");
                    }}
                    className={`flex w-full flex-col gap-0.5 border-b border-border px-3 py-3 text-left hover:bg-muted/50 ${
                      selectedId === c.id ? "bg-muted/70" : ""
                    }`}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className="truncate text-sm font-semibold">
                        {c.profile_name || c.profile_phone || c.id.slice(0, 8)}
                      </span>
                      {c.unread_admin > 0 ? (
                        <span className="rounded-full bg-destructive px-1.5 text-[10px] font-bold text-destructive-foreground">
                          {c.unread_admin}
                        </span>
                      ) : null}
                    </div>
                    <span className="truncate text-xs text-muted-foreground">{c.preview || "—"}</span>
                  </button>
                ))
              )}
            </div>
          </aside>

          {/* Chat */}
          <section
            className={`flex min-h-0 flex-col ${
              mobilePane === "chat" ? "flex" : "hidden md:flex"
            }`}
          >
            {!detail ? (
              <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
                {t("selectConversation")}
              </div>
            ) : (
              <>
                <div className="flex items-center justify-between border-b border-border px-3 py-2">
                  <button
                    type="button"
                    className="text-sm text-muted-foreground md:hidden"
                    onClick={() => setMobilePane("list")}
                  >
                    ←
                  </button>
                  <div className="min-w-0 flex-1 truncate px-2 text-sm font-semibold">
                    {detail.learner.name || detail.learner.phone}
                  </div>
                  <button
                    type="button"
                    className="rounded-lg border border-border p-2 md:hidden"
                    onClick={() => setMobilePane("user")}
                    aria-label={t("userDetails")}
                  >
                    <UserRound className="h-4 w-4" />
                  </button>
                  <button
                    type="button"
                    className="hidden rounded-lg border border-border p-2 md:inline-flex"
                    onClick={() => setUserOpen((v) => !v)}
                    aria-label={t("userDetails")}
                  >
                    <UserRound className="h-4 w-4" />
                  </button>
                </div>
                <ChatMessageList
                  messages={detail.items}
                  selfKind="admin"
                  emptyLabel={t("emptyChat")}
                  resolveAttachmentUrl={adminAttachmentUrl}
                />
                <ChatComposer
                  placeholder={t("placeholder")}
                  sendLabel={t("send")}
                  attachLabel={t("attach")}
                  onSend={send}
                />
              </>
            )}
          </section>

          {/* User sidebar — desktop column / mobile pane */}
          <aside
            className={`min-h-0 border-l border-border ${
              mobilePane === "user" ? "flex" : "hidden"
            } ${userOpen ? "md:flex" : "md:hidden"} flex-col`}
          >
            <div className="flex items-center justify-between border-b border-border px-3 py-2 md:hidden">
              <button type="button" onClick={() => setMobilePane("chat")}>
                ←
              </button>
              <span className="text-sm font-semibold">{t("userDetails")}</span>
              <span />
            </div>
            {userPanel}
          </aside>
        </div>
      </div>
    </PermissionGate>
  );
}
