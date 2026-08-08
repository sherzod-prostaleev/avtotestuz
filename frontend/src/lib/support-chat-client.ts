import { apiGet, apiPost, apiPostForm } from "@/lib/api-client";
import { resolveArenaWsUrl } from "@/lib/arena-client";

export type SupportConversation = {
  id: string;
  profile_id: string;
  status: "open" | "closed" | string;
  unread_admin: number;
  unread_user: number;
  last_message_at?: string | null;
  created_at: string;
  updated_at: string;
  profile_name?: string;
  profile_phone?: string;
  preview?: string;
};

export type SupportMessage = {
  id: string;
  conversation_id: string;
  sender_kind: "user" | "admin" | string;
  sender_profile_id?: string | null;
  sender_admin_id?: string | null;
  body: string;
  attachment_key?: string;
  attachment_name?: string;
  attachment_mime?: string;
  attachment_size?: number;
  attachment_url?: string;
  created_at: string;
};

export type SupportUpload = {
  key: string;
  name: string;
  mime: string;
  size: number;
  url: string;
};

export type SupportTicket = {
  ticket: string;
  expires_in_sec: number;
  ws_url: string;
};

export async function getMyConversation() {
  return apiGet<SupportConversation>("me/support/conversation");
}

export async function listMyMessages(before?: string) {
  const q = before ? `?before=${encodeURIComponent(before)}` : "";
  return apiGet<{ conversation: SupportConversation; items: SupportMessage[] }>(
    `me/support/messages${q}`,
  );
}

export async function postMyMessage(body: {
  body?: string;
  attachment_key?: string;
  attachment_name?: string;
  attachment_mime?: string;
  attachment_size?: number;
}) {
  return apiPost<{ message: SupportMessage; conversation: SupportConversation }>(
    "me/support/messages",
    body,
  );
}

export async function uploadMyAttachment(file: File) {
  const form = new FormData();
  form.append("file", file);
  return apiPostForm<SupportUpload>("me/support/attachments", form);
}

export async function markMySupportRead() {
  return apiPost<SupportConversation>("me/support/read");
}

export async function getMySupportUnread() {
  return apiGet<{ unread: number }>("me/support/unread");
}

export async function mintSupportTicket() {
  return apiPost<SupportTicket>("me/support/ws-ticket");
}

export type SupportSocketHandlers = {
  onMessage: (env: { t: string; d: Record<string, unknown> }) => void;
  onClose?: (code: number, reason: string) => void;
  onOpen?: () => void;
};

const RECONNECT_BASE_MS = 800;
const RECONNECT_MAX_MS = 15_000;

export class SupportSocket {
  private ws: WebSocket | null = null;
  private handlers: SupportSocketHandlers | null = null;
  private connectSeq = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectAttempt = 0;
  private intentionalClose = false;

  async connect(handlers: SupportSocketHandlers): Promise<void> {
    this.handlers = handlers;
    this.intentionalClose = false;
    this.reconnectAttempt = 0;
    await this.openSocket();
  }

  private async openSocket(): Promise<void> {
    const seq = ++this.connectSeq;
    this.closeSocketOnly();

    const ticket = await mintSupportTicket();
    if (seq !== this.connectSeq) return;

    const url = new URL(resolveArenaWsUrl(ticket.ws_url));
    url.searchParams.set("ticket", ticket.ticket);

    await new Promise<void>((resolve, reject) => {
      const ws = new WebSocket(url.toString());
      this.ws = ws;
      let settled = false;
      ws.onopen = () => {
        if (seq !== this.connectSeq) {
          ws.close();
          return;
        }
        this.reconnectAttempt = 0;
        this.handlers?.onOpen?.();
        settled = true;
        resolve();
      };
      ws.onerror = () => {
        if (!settled) {
          settled = true;
          reject(new Error("support_ws_error"));
        }
      };
      ws.onclose = (ev) => {
        if (seq !== this.connectSeq) return;
        this.handlers?.onClose?.(ev.code, ev.reason);
        if (!this.intentionalClose) {
          this.scheduleReconnect();
        }
        if (!settled) {
          settled = true;
          reject(new Error("support_ws_closed"));
        }
      };
      ws.onmessage = (ev) => {
        if (seq !== this.connectSeq || !this.handlers) return;
        try {
          const env = JSON.parse(String(ev.data)) as { t: string; d: Record<string, unknown> };
          this.handlers.onMessage(env);
        } catch {
          /* ignore malformed */
        }
      };
    });
  }

  private scheduleReconnect() {
    if (this.intentionalClose || !this.handlers) return;
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    const delay = Math.min(
      RECONNECT_MAX_MS,
      RECONNECT_BASE_MS * 2 ** Math.min(this.reconnectAttempt, 4),
    );
    this.reconnectAttempt += 1;
    this.reconnectTimer = setTimeout(() => {
      void this.openSocket().catch(() => {
        this.scheduleReconnect();
      });
    }, delay);
  }

  send(t: string, d: Record<string, unknown> = {}) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return false;
    this.ws.send(JSON.stringify({ t, d }));
    return true;
  }

  close() {
    this.intentionalClose = true;
    this.connectSeq += 1;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.closeSocketOnly();
  }

  private closeSocketOnly() {
    if (this.ws) {
      try {
        this.ws.close();
      } catch {
        /* ignore */
      }
      this.ws = null;
    }
  }
}
