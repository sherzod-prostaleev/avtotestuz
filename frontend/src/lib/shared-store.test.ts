import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { createSharedStore, useSharedStore } from "@/lib/shared-store";

describe("createSharedStore", () => {
  beforeEach(() => {
    vi.useRealTimers();
  });

  it("returns initial value before load", () => {
    const fetcher = vi.fn().mockResolvedValue(42);
    const store = createSharedStore(fetcher, 0);
    expect(store.get()).toBe(0);
  });

  it("fetches and caches value with 30s TTL", async () => {
    let callCount = 0;
    const fetcher = vi.fn().mockImplementation(async () => {
      callCount++;
      return `result-${callCount}`;
    });

    const store = createSharedStore(fetcher, "initial", { ttlMs: 30_000 });

    // First load hits fetcher
    const first = await store.load();
    expect(first).toBe("result-1");
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(store.get()).toBe("result-1");

    // Second load within TTL returns cached value without calling fetcher
    const second = await store.load();
    expect(second).toBe("result-1");
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("deduplicates parallel in-flight requests", async () => {
    let resolvePromise: (val: string) => void = () => {};
    const fetcher = vi.fn().mockImplementation(
      () =>
        new Promise<string>((resolve) => {
          resolvePromise = resolve;
        })
    );

    const store = createSharedStore(fetcher, "initial");

    // Trigger 3 concurrent loads
    const promise1 = store.load();
    const promise2 = store.load();
    const promise3 = store.load();

    expect(fetcher).toHaveBeenCalledTimes(1);

    resolvePromise("done");

    const [res1, res2, res3] = await Promise.all([promise1, promise2, promise3]);
    expect(res1).toBe("done");
    expect(res2).toBe("done");
    expect(res3).toBe("done");
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("force option bypasses TTL cache", async () => {
    let callCount = 0;
    const fetcher = vi.fn().mockImplementation(async () => {
      callCount++;
      return `count-${callCount}`;
    });

    const store = createSharedStore(fetcher, "initial", { ttlMs: 30_000 });

    await store.load();
    expect(store.get()).toBe("count-1");
    expect(fetcher).toHaveBeenCalledTimes(1);

    const forced = await store.load({ force: true });
    expect(forced).toBe("count-2");
    expect(store.get()).toBe("count-2");
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("notifies subscribers when value changes via load or set", async () => {
    const fetcher = vi.fn().mockResolvedValue("loaded-value");
    const store = createSharedStore(fetcher, "initial");

    const listener = vi.fn();
    const unsubscribe = store.subscribe(listener);

    await store.load();
    expect(listener).toHaveBeenCalledWith("loaded-value");

    store.set("manual-value");
    expect(listener).toHaveBeenCalledWith("manual-value");
    expect(store.get()).toBe("manual-value");

    unsubscribe();
    store.set("after-unsub");
    expect(listener).toHaveBeenCalledTimes(2);
  });

  it("falls back to stale cache on network failure if previously loaded", async () => {
    let shouldFail = false;
    const fetcher = vi.fn().mockImplementation(async () => {
      if (shouldFail) throw new Error("Network offline");
      return "fresh-data";
    });

    const store = createSharedStore(fetcher, "fallback", { ttlMs: 1 });

    const first = await store.load();
    expect(first).toBe("fresh-data");

    // Wait for TTL expiry
    await new Promise((r) => setTimeout(r, 5));

    shouldFail = true;
    const second = await store.load();
    expect(second).toBe("fresh-data");
  });

  it("resets store state properly", async () => {
    const fetcher = vi.fn().mockResolvedValue("loaded");
    const store = createSharedStore(fetcher, "initial");

    await store.load();
    expect(store.get()).toBe("loaded");

    store.reset();
    expect(store.get()).toBe("initial");
  });
});

describe("useSharedStore hook", () => {
  it("subscribes and updates reactively", async () => {
    const fetcher = vi.fn().mockResolvedValue(100);
    const store = createSharedStore(fetcher, 10);

    const { result } = renderHook(() => useSharedStore(store));
    expect(result.current).toBe(10);

    await act(async () => {
      await store.load();
    });

    expect(result.current).toBe(100);

    act(() => {
      store.set(200);
    });

    expect(result.current).toBe(200);
  });
});
