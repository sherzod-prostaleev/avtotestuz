"use client";

import { useEffect, useRef } from "react";
import type { SupportMessage } from "@/lib/support-chat-client";

type Props = {
  messages: SupportMessage[];
  selfKind: "user" | "admin";
  emptyLabel: string;
  /** Resolves authenticated download URL for an attachment-bearing message. */
  resolveAttachmentUrl: (m: SupportMessage) => string;
};

export function ChatMessageList({
  messages,
  selfKind,
  emptyLabel,
  resolveAttachmentUrl,
}: Props) {
  const bottomRef = useRef<HTMLDivElement>(null);
  // Chronological for display (API returns newest-first).
  const ordered = [...messages].sort(
    (a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime(),
  );
  const lastMessageId = ordered[ordered.length - 1]?.id;

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [ordered.length, lastMessageId]);

  if (ordered.length === 0) {
    return (
      <div className="flex flex-1 items-center justify-center px-4 text-center text-sm text-muted-foreground">
        {emptyLabel}
      </div>
    );
  }

  return (
    <div className="flex flex-1 flex-col gap-2 overflow-y-auto px-3 py-3">
      {ordered.map((m) => {
        const mine = m.sender_kind === selfKind;
        const url = resolveAttachmentUrl(m);
        const isImage = Boolean(url) && (m.attachment_mime || "").startsWith("image/");
        return (
          <div
            key={m.id}
            className={`flex max-w-[85%] flex-col gap-1 rounded-2xl px-3 py-2 text-sm shadow-sm ${
              mine
                ? "ml-auto bg-accent text-accent-foreground"
                : "mr-auto bg-muted text-foreground"
            }`}
          >
            {m.body ? <p className="whitespace-pre-wrap break-words">{m.body}</p> : null}
            {isImage ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={url}
                alt={m.attachment_name || "image"}
                className="max-h-56 max-w-full rounded-lg object-contain"
              />
            ) : null}
            {url && !isImage ? (
              <a
                href={url}
                target="_blank"
                rel="noreferrer"
                className={`underline ${mine ? "text-primary-foreground/90" : "text-primary"}`}
              >
                {m.attachment_name || "file"}
                {m.attachment_size ? ` (${formatSize(m.attachment_size)})` : ""}
              </a>
            ) : null}
            <time
              className={`text-[10px] tabular-nums opacity-70 ${mine ? "text-right" : "text-left"}`}
              dateTime={m.created_at}
            >
              {new Date(m.created_at).toLocaleString()}
            </time>
          </div>
        );
      })}
      <div ref={bottomRef} />
    </div>
  );
}

function formatSize(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

/** Learner BFF path for a private support attachment. */
export function learnerAttachmentUrl(m: SupportMessage): string {
  if (!m.id) return "";
  if (!m.attachment_mime && !m.attachment_name && !m.attachment_url && !m.attachment_size) {
    return "";
  }
  if (m.attachment_url?.startsWith("/api/")) return m.attachment_url;
  if (m.attachment_url?.startsWith("/me/")) return `/api/proxy${m.attachment_url}`;
  return `/api/proxy/me/support/messages/${m.id}/attachment`;
}

/** Admin BFF path for a private support attachment. */
export function adminAttachmentUrl(m: SupportMessage): string {
  if (!m.id) return "";
  if (!m.attachment_mime && !m.attachment_name && !m.attachment_url && !m.attachment_size) {
    return "";
  }
  if (m.attachment_url?.startsWith("/api/")) return m.attachment_url;
  if (m.attachment_url?.startsWith("/support/")) return `/api/admin${m.attachment_url}`;
  return `/api/admin/support/messages/${m.id}/attachment`;
}
