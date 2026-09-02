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
    // The shell's own reserve under the content (`pb-[4.25rem]`) is 20px taller
    // than the tab bar it stands in for, and on a chat screen that reserve is
    // dead space between the composer and the bar. Below `md` the column is
    // measured against the bar itself — top bar 3.5rem, bar 3rem + its border
    // and a hair of clearance — so the composer lands just above it the way the
    // artboard draws it. `md:h-[calc(100dvh)]` still wins from 768px up.
    <div className="flex h-[calc(100dvh-3.5rem-4.25rem)] flex-col max-md:h-[calc(100dvh-3.5rem-3.25rem-env(safe-area-inset-bottom))] md:h-[calc(100dvh)] md:min-h-[32rem]">
      <header className="flex items-center justify-between border-b border-border px-4 py-3 max-md:border-b-0 max-md:px-3 max-md:pb-0 max-md:pt-3">
        <div>
          <h1 className="font-display text-lg font-bold max-md:text-[24px] max-md:font-extrabold max-md:leading-[1.15]">
            {t("title")}
          </h1>
          <p className="text-xs text-muted-foreground max-md:text-[15px]">{t("subtitle")}</p>
        </div>
        {conversation ? (
          // Phone: the artboard's 28px pill — bordered, tinted, with a status
          // dot. `!` on the two colours because `dark:` carries one more class
          // of specificity than a `max-md:` media rule can answer.
          <span
            className={`rounded-full px-2 py-0.5 text-[11px] font-semibold uppercase max-md:inline-flex max-md:h-[28px] max-md:items-center max-md:gap-1.5 max-md:border max-md:px-2.5 max-md:py-0 max-md:text-[13px] max-md:font-bold max-md:normal-case ${
              conversation.status === "closed"
                ? "bg-muted text-muted-foreground max-md:border-border"
                : "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300 max-md:!border-success/40 max-md:!bg-success/10 max-md:!text-success"
            }`}
          >
            <span
              className={`hidden h-[7px] w-[7px] flex-none rounded-full max-md:block ${
                conversation.status === "closed" ? "bg-muted-foreground" : "bg-success"
              }`}
              aria-hidden
            />
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
