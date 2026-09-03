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
    // Phone: a short thread sits on the composer instead of hanging from the
    // header. `mt-auto` on the first bubble rather than `justify-end` because an
    // auto margin collapses to 0 once the thread overflows, so the oldest
    // message stays reachable at the top of the scroll.
    <div className="flex flex-1 flex-col gap-2 overflow-y-auto px-3 py-3 max-md:py-3.5 max-md:[&>*:first-child]:mt-auto">
      {ordered.map((m) => {
        const mine = m.sender_kind === selfKind;
        const url = resolveAttachmentUrl(m);
        const isImage = Boolean(url) && (m.attachment_mime || "").startsWith("image/");
        return (
          <div
            key={m.id}
            className={`flex max-w-[85%] flex-col gap-1 rounded-2xl px-3 py-2 text-sm shadow-sm max-md:max-w-[calc(82%+1.5rem)] max-md:gap-[3px] max-md:rounded-[14px] max-md:py-[9px] max-md:text-[15px] max-md:leading-[20px] max-md:shadow-none ${
              mine
                ? "ml-auto bg-accent text-accent-foreground max-md:rounded-br-[4px]"
                : "mr-auto bg-muted text-foreground max-md:rounded-bl-[4px] max-md:border max-md:border-border max-md:bg-card"
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
              className={`text-[10px] tabular-nums opacity-70 max-md:text-[12px] max-md:leading-[16px] ${mine ? "text-right" : "text-left"}`}
              dateTime={m.created_at}
            >
              {/* The stamp is the whole date on a wide screen, where the thread
                  is one panel among many and the day matters; on a phone the
                  artboard shows the clock alone, so both are rendered and the
                  breakpoint picks one. `hourCycle` keeps it 24-hour whatever
                  the browser's own locale would default to. */}
              <span className="max-md:hidden">{new Date(m.created_at).toLocaleString()}</span>
              <span className="hidden max-md:inline">
                {new Date(m.created_at).toLocaleTimeString(undefined, {
                  hour: "2-digit",
                  minute: "2-digit",
                  hourCycle: "h23",
                })}
              </span>
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
