import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createSharedCount } from "@/lib/shared-count";

describe("createSharedCount", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("collapses concurrent readers into a single request", async () => {
    let resolve!: (value: number) => void;
    const fetcher = vi.fn(() => new Promise<number>((r) => (resolve = r)));
    const store = createSharedCount(fetcher, { ttlMs: 30_000 });

    const a = store.load();
    const b = store.load();
    resolve(7);

    expect(await a).toBe(7);
    expect(await b).toBe(7);
    // Two mounted bells asking at once must not become two round trips.
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("serves a read inside the TTL from memory", async () => {
    const fetcher = vi.fn(async () => 3);
    const store = createSharedCount(fetcher, { ttlMs: 30_000 });

    expect(await store.load()).toBe(3);
    vi.advanceTimersByTime(29_000);
    expect(await store.load()).toBe(3);
    expect(fetcher).toHaveBeenCalledTimes(1);

    vi.advanceTimersByTime(2_000);
    expect(await store.load()).toBe(3);
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("refetches inside the TTL when forced", async () => {
    const fetcher = vi.fn(async () => 1);
    const store = createSharedCount(fetcher, { ttlMs: 30_000 });

    await store.load();
    await store.load({ force: true });

    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("keeps the last known value when a poll fails", async () => {
    const fetcher = vi
      .fn<[], Promise<number>>()
      .mockResolvedValueOnce(5)
      .mockRejectedValueOnce(new Error("offline"));
    const store = createSharedCount(fetcher, { ttlMs: 0 });

    expect(await store.load()).toBe(5);
    // A failed refresh must not blank the badge.
    expect(await store.load()).toBe(5);
    expect(store.get()).toBe(5);
  });

  it("notifies every subscriber and stops after unsubscribe", async () => {
    const fetcher = vi.fn(async () => 9);
    const store = createSharedCount(fetcher, { ttlMs: 30_000 });
    const first: number[] = [];
    const second: number[] = [];

    const unsubscribeFirst = store.subscribe((v) => first.push(v));
    store.subscribe((v) => second.push(v));

    await store.load();
    expect(first).toEqual([9]);
    expect(second).toEqual([9]);

    unsubscribeFirst();
    store.set(4);
    expect(first).toEqual([9]);
    expect(second).toEqual([9, 4]);
  });

  it("runs one poll timer no matter how many subscribers there are", async () => {
    const fetcher = vi.fn(async () => 2);
    const store = createSharedCount(fetcher, { ttlMs: 30_000, pollMs: 60_000 });

    const offA = store.subscribe(() => {});
    const offB = store.subscribe(() => {});

    await vi.advanceTimersByTimeAsync(60_000);
    // Two bells, one interval.
    expect(fetcher).toHaveBeenCalledTimes(1);

    offA();
    offB();
    await vi.advanceTimersByTimeAsync(120_000);
    // Timer is released with the last subscriber.
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("does not start a timer when polling is disabled", async () => {
    const fetcher = vi.fn(async () => 2);
    const store = createSharedCount(fetcher, { ttlMs: 30_000 });

    store.subscribe(() => {});
    await vi.advanceTimersByTimeAsync(300_000);

    expect(fetcher).not.toHaveBeenCalled();
  });

  it("reset makes the next read refetch", async () => {
    const fetcher = vi.fn(async () => 6);
    const store = createSharedCount(fetcher, { ttlMs: 30_000 });

    await store.load();
    store.reset();

    expect(store.get()).toBe(0);
    await store.load();
    expect(fetcher).toHaveBeenCalledTimes(2);
  });
});
