import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ArenaSocket, resolveArenaWsUrl } from "@/lib/arena-client";

vi.mock("@/lib/api-client", () => ({
  apiPost: vi.fn(),
}));

import { apiPost } from "@/lib/api-client";

type Listener = ((ev?: unknown) => void) | null;

class FakeWebSocket {
  static OPEN = 1;
  static CONNECTING = 0;
  static CLOSING = 2;
  static CLOSED = 3;
  static instances: FakeWebSocket[] = [];

  readyState = FakeWebSocket.CONNECTING;
  onopen: Listener = null;
  onerror: Listener = null;
  onclose: Listener = null;
  onmessage: Listener = null;
  sent: string[] = [];
  url: string;

  constructor(url: string) {
    this.url = url;
    FakeWebSocket.instances.push(this);
  }

  send(data: string) {
    this.sent.push(data);
  }

  close() {
    this.readyState = FakeWebSocket.CLOSED;
    this.onclose?.({ code: 1000, reason: "client_close" });
  }

  openNow() {
    this.readyState = FakeWebSocket.OPEN;
    this.onopen?.({});
  }

  failNow() {
    this.onerror?.({});
    this.readyState = FakeWebSocket.CLOSED;
    this.onclose?.({ code: 1006, reason: "error" });
  }
}

async function waitForSocket(): Promise<FakeWebSocket> {
  for (let i = 0; i < 20; i++) {
    if (FakeWebSocket.instances.length > 0) {
      return FakeWebSocket.instances[0];
    }
    await Promise.resolve();
  }
  throw new Error("WebSocket was not created");
}

describe("resolveArenaWsUrl", () => {
  it("rewrites compose-internal api host to the page origin", () => {
    vi.stubGlobal("window", {
      location: { protocol: "https:", host: "drivergo.uz" },
    });
    expect(resolveArenaWsUrl("ws://api:8080/api/v1/arena/ws")).toBe(
      "wss://drivergo.uz/api/v1/arena/ws"
    );
  });

  it("leaves public and localhost hosts unchanged", () => {
    expect(resolveArenaWsUrl("wss://drivergo.uz/api/v1/arena/ws")).toBe(
      "wss://drivergo.uz/api/v1/arena/ws"
    );
    expect(resolveArenaWsUrl("ws://localhost:8080/api/v1/arena/ws")).toBe(
      "ws://localhost:8080/api/v1/arena/ws"
    );
  });
});

describe("ArenaSocket", () => {
  beforeEach(() => {
    FakeWebSocket.instances = [];
    vi.mocked(apiPost).mockReset();
    vi.mocked(apiPost).mockResolvedValue({
      ticket: "test-ticket",
      expires_in_sec: 30,
      ws_url: "ws://localhost:8090/api/v1/arena/ws",
    });
    vi.stubGlobal("WebSocket", FakeWebSocket);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("waits for OPEN before connect() resolves so send works immediately after", async () => {
    const sock = new ArenaSocket();
    const onOpen = vi.fn();

    const connecting = sock.connect({ onMessage: vi.fn(), onOpen });
    const ws = await waitForSocket();

    expect(sock.isOpen()).toBe(false);
    expect(sock.send("queue.join", { locale: "uz-Latn" })).toBe(false);

    ws.openNow();
    await connecting;

    expect(onOpen).toHaveBeenCalledOnce();
    expect(sock.isOpen()).toBe(true);
    expect(sock.send("queue.join", { locale: "uz-Latn" })).toBe(true);
    expect(ws.sent).toHaveLength(1);
    expect(JSON.parse(ws.sent[0]).t).toBe("queue.join");
    expect(ws.url).toContain("ticket=test-ticket");
  });

  it("reuses an open socket instead of minting another ticket", async () => {
    const sock = new ArenaSocket();

    const first = sock.connect({ onMessage: vi.fn() });
    const ws = await waitForSocket();
    ws.openNow();
    await first;

    await sock.connect({ onMessage: vi.fn() });

    expect(apiPost).toHaveBeenCalledTimes(1);
    expect(FakeWebSocket.instances).toHaveLength(1);
  });

  it("rejects when the handshake errors before open", async () => {
    const sock = new ArenaSocket();
    const onError = vi.fn();

    const connecting = sock.connect({ onMessage: vi.fn(), onError });
    const ws = await waitForSocket();
    ws.failNow();

    await expect(connecting).rejects.toThrow(/arena_ws/);
    expect(onError).toHaveBeenCalledOnce();
  });
});
