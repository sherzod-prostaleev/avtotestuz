import { apiPost } from "@/lib/api-client";
import { encodeClient, parseEnvelope, type ArenaEnvelope } from "@/lib/arena-protocol";

export type ArenaTicket = {
  ticket: string;
  expires_in_sec: number;
  ws_url: string;
};

export async function mintArenaTicket(): Promise<ArenaTicket> {
  return apiPost<ArenaTicket>("arena/ws-ticket");
}

export type ArenaSocketHandlers = {
  onMessage: (env: ArenaEnvelope) => void;
  onClose?: (code: number, reason: string) => void;
  onOpen?: () => void;
  onError?: () => void;
};

/**
 * Arena WebSocket client. `connect()` resolves only after the socket is OPEN
 * so callers can safely `send()` immediately after awaiting it. Reuses an
 * already-open socket (updates handlers) instead of tearing it down on every
 * CTA click — the previous implementation raced `send` against `onopen` and
 * silently dropped queue/invite messages.
 */
export class ArenaSocket {
  private ws: WebSocket | null = null;
  private handlers: ArenaSocketHandlers | null = null;
  private connectSeq = 0;
  private pending: Promise<void> | null = null;

  isOpen(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  async connect(handlers: ArenaSocketHandlers): Promise<void> {
    this.handlers = handlers;

    if (this.ws?.readyState === WebSocket.OPEN) {
      return;
    }
    if (this.pending) {
      await this.pending;
      if (this.ws?.readyState === WebSocket.OPEN) {
        return;
      }
    }

    const seq = ++this.connectSeq;
    this.detachSocket();

    this.pending = this.openNew(seq);
    try {
      await this.pending;
    } finally {
      if (this.connectSeq === seq) {
        this.pending = null;
      }
    }
  }

  send(t: string, d: Record<string, unknown> = {}): boolean {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      return false;
    }
    this.ws.send(encodeClient(t, d));
    return true;
  }

  close() {
    this.connectSeq += 1;
    this.pending = null;
    this.detachSocket();
  }

  private async openNew(seq: number): Promise<void> {
    const ticket = await mintArenaTicket();
    if (seq !== this.connectSeq) {
      throw new Error("arena_ws_superseded");
    }

    const url = new URL(ticket.ws_url);
    url.searchParams.set("ticket", ticket.ticket);

    await new Promise<void>((resolve, reject) => {
      const ws = new WebSocket(url.toString());
      this.ws = ws;

      ws.onmessage = (ev) => {
        if (seq !== this.connectSeq || !this.handlers) return;
        try {
          this.handlers.onMessage(parseEnvelope(String(ev.data)));
        } catch {
          /* ignore malformed */
        }
      };

      ws.onopen = () => {
        if (seq !== this.connectSeq) {
          try {
            ws.close(1000, "superseded");
          } catch {
            /* ignore */
          }
          reject(new Error("arena_ws_superseded"));
          return;
        }
        this.handlers?.onOpen?.();
        resolve();
      };

      ws.onerror = () => {
        if (seq !== this.connectSeq) {
          reject(new Error("arena_ws_superseded"));
          return;
        }
        this.handlers?.onError?.();
        reject(new Error("arena_ws_error"));
      };

      ws.onclose = (ev) => {
        if (seq !== this.connectSeq) return;
        this.ws = null;
        this.handlers?.onClose?.(ev.code, ev.reason);
        // onclose after a settled open is fine (Promise already resolved).
        reject(new Error("arena_ws_closed"));
      };
    });
  }

  private detachSocket() {
    if (!this.ws) return;
    const ws = this.ws;
    this.ws = null;
    ws.onopen = null;
    ws.onmessage = null;
    ws.onerror = null;
    ws.onclose = null;
    try {
      ws.close(1000, "client_close");
    } catch {
      /* already closed */
    }
  }
}
