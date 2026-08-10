"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { ChatComposer } from "@/components/support-chat/chat-composer";
import {
  ChatMessageList,
  learnerAttachmentUrl,
} from "@/components/support-chat/chat-message-list";
import {
  SupportSocket,
  getMyConversation,
  listMyMessages,
  markMySupportRead,
  postMyMessage,
  uploadMyAttachment,
  type SupportConversation,
  type SupportMessage,
} from "@/lib/support-chat-client";

export default function SupportChatPage() {
  const t = useTranslations("SupportChat");
  const [conversation, setConversation] = useState<SupportConversation | null>(null);
  const [messages, setMessages] = useState<SupportMessage[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const mergeMessage = useCallback((msg: SupportMessage) => {
    setMessages((prev) => {
      if (prev.some((m) => m.id === msg.id)) return prev;
      return [msg, ...prev];
    });
  }, []);

  useEffect(() => {
    let cancelled = false;
    const sock = new SupportSocket();

    (async () => {
      try {
        const [conv, page] = await Promise.all([getMyConversation(), listMyMessages()]);
        if (cancelled) return;
        setConversation(page.conversation ?? conv);
        setMessages(page.items ?? []);
        await markMySupportRead();
        // REST history is enough to use chat; WS is best-effort realtime.
        try {
          await sock.connect({
            onMessage: (env) => {
              if (env.t === "message.new") {
                const d = env.d as {
                  message?: SupportMessage;
                  conversation?: SupportConversation;
                };
                if (d.message) mergeMessage(d.message);
                if (d.conversation) setConversation(d.conversation);
                void markMySupportRead();
              }
              if (env.t === "conversation.updated" && env.d.conversation) {
                setConversation(env.d.conversation as SupportConversation);
              }
            },
          });
          if (page.conversation?.id) {
            sock.send("subscribe", { conversation_id: page.conversation.id });
          }
        } catch {
          /* reconnect loop inside SupportSocket; keep REST UI usable */
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : t("loadError"));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();

    return () => {
      cancelled = true;
      sock.close();
    };
  }, [mergeMessage, t]);

  async function handleSend(text: string, file: File | null) {
    let attachment:
      | {
          attachment_key: string;
          attachment_name: string;
          attachment_mime: string;
          attachment_size: number;
        }
      | undefined;
    if (file) {
      const up = await uploadMyAttachment(file);
      attachment = {
        attachment_key: up.key,
        attachment_name: up.name,
        attachment_mime: up.mime,
        attachment_size: up.size,
      };
    }
    const res = await postMyMessage({ body: text, ...attachment });
    mergeMessage(res.message);
    setConversation(res.conversation);
  }

  return (
    <div className="flex h-[calc(100dvh-3.5rem-4.25rem)] flex-col md:h-[calc(100dvh)] md:min-h-[32rem]">
      <header className="flex items-center justify-between border-b border-border px-4 py-3">
        <div>
          <h1 className="font-display text-lg font-bold">{t("title")}</h1>
          <p className="text-xs text-muted-foreground">{t("subtitle")}</p>
        </div>
        {conversation ? (
          <span
            className={`rounded-full px-2 py-0.5 text-[11px] font-semibold uppercase ${
              conversation.status === "closed"
                ? "bg-muted text-muted-foreground"
                : "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300"
            }`}
          >
            {conversation.status === "closed" ? t("statusClosed") : t("statusOpen")}
          </span>
        ) : null}
      </header>

      {loading ? (
        <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
          {t("loading")}
        </div>
      ) : error ? (
        <div className="flex flex-1 items-center justify-center px-4 text-center text-sm text-destructive">
          {error}
        </div>
      ) : (
        <>
          <ChatMessageList
            messages={messages}
            selfKind="user"
            emptyLabel={t("empty")}
            resolveAttachmentUrl={learnerAttachmentUrl}
          />
          <ChatComposer
            disabled={false}
            placeholder={t("placeholder")}
            sendLabel={t("send")}
            attachLabel={t("attach")}
            onSend={handleSend}
          />
        </>
      )}
    </div>
  );
}
