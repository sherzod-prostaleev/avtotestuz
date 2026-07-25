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

export class ArenaSocket {
  private ws: WebSocket | null = null;

  async connect(handlers: ArenaSocketHandlers): Promise<void> {
    this.close();
    const ticket = await mintArenaTicket();
    const url = new URL(ticket.ws_url);
    url.searchParams.set("ticket", ticket.ticket);
    const ws = new WebSocket(url.toString());
    this.ws = ws;
    ws.onopen = () => handlers.onOpen?.();
    ws.onerror = () => handlers.onError?.();
    ws.onclose = (ev) => handlers.onClose?.(ev.code, ev.reason);
    ws.onmessage = (ev) => {
      try {
        handlers.onMessage(parseEnvelope(String(ev.data)));
      } catch {
        /* ignore malformed */
      }
    };
  }

  send(t: string, d: Record<string, unknown> = {}) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
    this.ws.send(encodeClient(t, d));
  }

  close() {
    if (this.ws) {
      this.ws.onclose = null;
      this.ws.close(1000, "client_close");
      this.ws = null;
    }
  }
}
