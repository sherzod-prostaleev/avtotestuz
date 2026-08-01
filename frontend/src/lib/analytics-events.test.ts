import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createAnalyticsEventQueue,
  type AnalyticsBatchPayload,
  type SafeAnalyticsProps,
} from "./analytics-events";

async function settlePromises(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
}

describe("analytics event queue", () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("uses the exact POST /events payload and strips non-allowlisted or likely-PII props", async () => {
    const send = vi.fn<(payload: AnalyticsBatchPayload) => Promise<void>>().mockResolvedValue(undefined);
    const queue = createAnalyticsEventQueue({
      send,
      now: () => new Date("2026-07-22T10:11:12.000Z"),
      flushAt: 2,
      lifecycle: false,
    });

    queue.track("view_question", {
      question_id: "question-1",
      mode: "practice",
      source: "person@example.com",
      session_id: "+998901234567",
      phone: "+998901234567",
      email: "person@example.com",
      nested: { secret: true },
    } as unknown as SafeAnalyticsProps);
    queue.track("answer", { question_id: "question-1", correct: true });
    await settlePromises();

    expect(send).toHaveBeenCalledTimes(1);
    expect(send).toHaveBeenCalledWith({
      idempotency_key: expect.any(String),
      events: [
        {
          name: "view_question",
          props: { question_id: "question-1", mode: "practice" },
          ts: "2026-07-22T10:11:12.000Z",
        },
        {
          name: "answer",
          props: { question_id: "question-1", correct: true },
          ts: "2026-07-22T10:11:12.000Z",
        },
      ],
    });
    expect(queue.size).toBe(0);
    queue.dispose();
  });

  it("ignores invalid event names and bounds memory while retaining the newest pending events", async () => {
    const send = vi.fn<(payload: AnalyticsBatchPayload) => Promise<void>>().mockResolvedValue(undefined);
    const queue = createAnalyticsEventQueue({
      send,
      maxQueueSize: 3,
      flushAt: 10,
      flushIntervalMs: 60_000,
      lifecycle: false,
    });

    queue.track("contains raw spaces");
    queue.track("event_1");
    queue.track("event_2");
    queue.track("event_3");
    queue.track("event_4");

    expect(queue.size).toBe(3);
    await queue.flush();
    expect(send.mock.calls[0][0].events.map((event) => event.name)).toEqual(["event_2", "event_3", "event_4"]);
    queue.dispose();
  });

  it("never sends an empty batch and splits queued events into backend-safe batches of at most 100", async () => {
    const send = vi.fn<(payload: AnalyticsBatchPayload) => Promise<void>>().mockResolvedValue(undefined);
    const queue = createAnalyticsEventQueue({
      send,
      maxQueueSize: 205,
      flushAt: 1_000,
      flushIntervalMs: 60_000,
      lifecycle: false,
    });

    await queue.flush();
    expect(send).not.toHaveBeenCalled();

    for (let index = 0; index < 205; index += 1) {
      queue.track("view_question", { position: index + 1 });
    }
    await queue.flush();
    await queue.flush();
    await queue.flush();

    expect(send.mock.calls.map(([payload]) => payload.events.length)).toEqual([100, 100, 5]);
    expect(queue.size).toBe(0);
    queue.dispose();
  });

  it("flushes periodically and immediately at the configured size threshold", async () => {
    vi.useFakeTimers();
    const send = vi.fn<(payload: AnalyticsBatchPayload) => Promise<void>>().mockResolvedValue(undefined);
    const queue = createAnalyticsEventQueue({
      send,
      flushAt: 3,
      flushIntervalMs: 1_000,
      lifecycle: false,
    });

    queue.track("event_1");
    await vi.advanceTimersByTimeAsync(999);
    expect(send).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(send).toHaveBeenCalledTimes(1);

    queue.track("event_2");
    queue.track("event_3");
    queue.track("event_4");
    await settlePromises();
    expect(send).toHaveBeenCalledTimes(2);
    expect(send.mock.calls[1][0].events).toHaveLength(3);
    queue.dispose();
  });

  it("retains a failed batch and retries it once without concurrent duplication", async () => {
    vi.useFakeTimers();
    const send = vi
      .fn<(payload: AnalyticsBatchPayload) => Promise<void>>()
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce(undefined);
    const queue = createAnalyticsEventQueue({
      send,
      flushAt: 2,
      retryBaseMs: 500,
      retryMaxMs: 2_000,
      lifecycle: false,
    });

    queue.track("event_1");
    queue.track("event_2");
    await settlePromises();
    expect(send).toHaveBeenCalledTimes(1);
    expect(queue.size).toBe(2);

    void queue.flush();
    await settlePromises();
    expect(send).toHaveBeenCalledTimes(2);
    expect(send.mock.calls[1][0]).toEqual(send.mock.calls[0][0]);
    expect(queue.size).toBe(0);

    await vi.advanceTimersByTimeAsync(2_000);
    expect(send).toHaveBeenCalledTimes(2);
    queue.dispose();
  });

  it("returns from track before sending and swallows synchronous sender failures", async () => {
    vi.useFakeTimers();
    const send = vi.fn<(payload: AnalyticsBatchPayload) => Promise<void>>(() => {
      throw new Error("offline");
    });
    const queue = createAnalyticsEventQueue({
      send,
      flushAt: 1,
      retryBaseMs: 500,
      lifecycle: false,
    });

    expect(() => queue.track("event_1")).not.toThrow();
    expect(send).not.toHaveBeenCalled();
    await settlePromises();
    expect(send).toHaveBeenCalledTimes(1);
    expect(queue.size).toBe(1);
    queue.dispose();
  });

  it("retries automatically after the deterministic backoff delay", async () => {
    vi.useFakeTimers();
    const send = vi
      .fn<(payload: AnalyticsBatchPayload) => Promise<void>>()
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce(undefined);
    const queue = createAnalyticsEventQueue({
      send,
      flushAt: 1,
      retryBaseMs: 500,
      retryMaxMs: 2_000,
      lifecycle: false,
    });

    queue.track("event_1");
    await settlePromises();
    expect(send).toHaveBeenCalledTimes(1);
    expect(queue.size).toBe(1);

    await vi.advanceTimersByTimeAsync(499);
    expect(send).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(send).toHaveBeenCalledTimes(2);
    expect(queue.size).toBe(0);
    queue.dispose();
  });

  it("flushes on hidden visibility and pagehide lifecycle boundaries", async () => {
    const originalVisibility = Object.getOwnPropertyDescriptor(document, "visibilityState");
    const send = vi.fn<(payload: AnalyticsBatchPayload) => Promise<void>>().mockResolvedValue(undefined);
    const queue = createAnalyticsEventQueue({
      send,
      flushAt: 10,
      flushIntervalMs: 60_000,
      lifecycle: true,
    });

    queue.track("event_1");
    Object.defineProperty(document, "visibilityState", { configurable: true, value: "hidden" });
    document.dispatchEvent(new Event("visibilitychange"));
    await settlePromises();
    expect(send).toHaveBeenCalledTimes(1);

    queue.track("event_2");
    window.dispatchEvent(new Event("pagehide"));
    await settlePromises();
    expect(send).toHaveBeenCalledTimes(2);

    queue.dispose();
    if (originalVisibility) {
      Object.defineProperty(document, "visibilityState", originalVisibility);
    }
  });

  it("is SSR-safe and delegates production sends through the existing BFF apiPost", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ data: { ok: true, count: 1 } }),
    } as Response);
    vi.stubGlobal("fetch", fetchMock);
    const queue = createAnalyticsEventQueue({ lifecycle: false, flushAt: 10 });
    queue.track("session_finish", { status: "passed", score: 18, total: 20 });
    await queue.flush();

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/proxy/events",
      expect.objectContaining({
        method: "POST",
        body: expect.any(String),
      })
    );
    const request = fetchMock.mock.calls[0][1] as RequestInit;
    expect(JSON.parse(String(request.body))).toEqual({
      idempotency_key: expect.any(String),
      events: [
        expect.objectContaining({
          name: "session_finish",
          props: { status: "passed", score: 18, total: 20 },
        }),
      ],
    });
    queue.dispose();

    vi.stubGlobal("window", undefined);
    vi.stubGlobal("document", undefined);
    expect(() => createAnalyticsEventQueue({ lifecycle: true })).not.toThrow();
  });
});
