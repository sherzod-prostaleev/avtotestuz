import { afterEach, beforeEach, describe, it, expect, vi } from "vitest";
import { MAX_RECENT_ENTRIES, getRecentMapSizeForTests, refreshOnce, resetRefreshLockForTests } from "./refresh-lock";

describe("refreshOnce", () => {
  beforeEach(() => {
    resetRefreshLockForTests();
  });

  it("only calls doRefresh once for concurrent callers with the same token", async () => {
    let resolveRefresh: (v: { accessToken: string; refreshToken: string }) => void;
    const doRefresh = vi.fn(
      () =>
        new Promise<{ accessToken: string; refreshToken: string } | null>((resolve) => {
          resolveRefresh = resolve;
        })
    );

    const call1 = refreshOnce("rt-1", doRefresh);
    const call2 = refreshOnce("rt-1", doRefresh);
    resolveRefresh!({ accessToken: "new-at", refreshToken: "new-rt" });

    const [result1, result2] = await Promise.all([call1, call2]);

    expect(doRefresh).toHaveBeenCalledTimes(1);
    expect(result1).toEqual({ accessToken: "new-at", refreshToken: "new-rt" });
    expect(result2).toEqual({ accessToken: "new-at", refreshToken: "new-rt" });
  });

  it("allows a new refresh after the previous one settles", async () => {
    const doRefresh = vi
      .fn()
      .mockResolvedValueOnce({ accessToken: "at-1", refreshToken: "rt-1" })
      .mockResolvedValueOnce({ accessToken: "at-2", refreshToken: "rt-2" });

    const first = await refreshOnce("rt-0", doRefresh);
    const second = await refreshOnce("rt-1", doRefresh);

    expect(doRefresh).toHaveBeenCalledTimes(2);
    expect(first?.accessToken).toBe("at-1");
    expect(second?.accessToken).toBe("at-2");
  });

  it("propagates a null result (refresh failed) to all concurrent callers", async () => {
    const doRefresh = vi.fn().mockResolvedValue(null);
    const [result1, result2] = await Promise.all([refreshOnce("rt-x", doRefresh), refreshOnce("rt-x", doRefresh)]);
    expect(doRefresh).toHaveBeenCalledTimes(1);
    expect(result1).toBeNull();
    expect(result2).toBeNull();
  });

  it("never mixes up two different users' concurrent refreshes (cross-session isolation)", async () => {
    // This server-side lock handles every logged-in user's requests in the
    // same process. If the lock were a single global (not keyed by token),
    // user B's concurrent request would collapse into user A's in-flight
    // refresh and come back with user A's rotated tokens — a cross-user
    // session leak. Two DIFFERENT tokens must always refresh independently,
    // even when their calls overlap in time.
    const doRefresh = vi.fn((rt: string) =>
      rt === "rt-userA"
        ? Promise.resolve({ accessToken: "at-userA", refreshToken: "rt-userA-2" })
        : Promise.resolve({ accessToken: "at-userB", refreshToken: "rt-userB-2" })
    );

    const [resultA, resultB] = await Promise.all([
      refreshOnce("rt-userA", doRefresh),
      refreshOnce("rt-userB", doRefresh),
    ]);

    expect(doRefresh).toHaveBeenCalledTimes(2);
    expect(doRefresh).toHaveBeenCalledWith("rt-userA");
    expect(doRefresh).toHaveBeenCalledWith("rt-userB");
    expect(resultA).toEqual({ accessToken: "at-userA", refreshToken: "rt-userA-2" });
    expect(resultB).toEqual({ accessToken: "at-userB", refreshToken: "rt-userB-2" });
  });

  it("returns cached rotation for the same old token after settle (no reuse call)", async () => {
    const doRefresh = vi
      .fn()
      .mockResolvedValueOnce({ accessToken: "at-1", refreshToken: "rt-new" })
      .mockResolvedValueOnce({ accessToken: "should-not", refreshToken: "happen" });

    const first = await refreshOnce("rt-old", doRefresh);
    const late = await refreshOnce("rt-old", doRefresh);

    expect(doRefresh).toHaveBeenCalledTimes(1);
    expect(first).toEqual({ accessToken: "at-1", refreshToken: "rt-new" });
    expect(late).toEqual({ accessToken: "at-1", refreshToken: "rt-new" });
  });

  describe("rotation-grace cache eviction (FE-1)", () => {
    beforeEach(() => {
      vi.useFakeTimers();
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it("evicts an entry once its TTL passes even if it is never read again", async () => {
      const doRefresh = vi.fn().mockResolvedValue({ accessToken: "at-1", refreshToken: "rt-new" });

      await refreshOnce("rt-old", doRefresh);
      expect(getRecentMapSizeForTests()).toBe(1);

      // Never call refreshOnce/readRecent for "rt-old" again — the finding
      // is specifically that eviction used to require re-reading the exact
      // same key. Advance real time past the 30s grace window plus at least
      // one sweep interval so the background sweep (not a read) does the
      // evicting.
      await vi.advanceTimersByTimeAsync(45_000);

      expect(getRecentMapSizeForTests()).toBe(0);
    });

    it("does not exceed the cap when a burst of refreshes arrives faster than the sweep", async () => {
      const doRefresh = vi.fn((rt: string) =>
        Promise.resolve({ accessToken: `at-${rt}`, refreshToken: `${rt}-2` })
      );

      const burst = MAX_RECENT_ENTRIES + 250;
      for (let i = 0; i < burst; i++) {
        await refreshOnce(`rt-burst-${i}`, doRefresh);
        expect(getRecentMapSizeForTests()).toBeLessThanOrEqual(MAX_RECENT_ENTRIES);
      }

      expect(getRecentMapSizeForTests()).toBeLessThanOrEqual(MAX_RECENT_ENTRIES);
    });

    it("still single-flights concurrent callers with the same token once eviction is wired up", async () => {
      let resolveRefresh: (v: { accessToken: string; refreshToken: string }) => void;
      const doRefresh = vi.fn(
        () =>
          new Promise<{ accessToken: string; refreshToken: string } | null>((resolve) => {
            resolveRefresh = resolve;
          })
      );

      const call1 = refreshOnce("rt-concurrent", doRefresh);
      const call2 = refreshOnce("rt-concurrent", doRefresh);
      resolveRefresh!({ accessToken: "new-at", refreshToken: "new-rt" });

      const [result1, result2] = await Promise.all([call1, call2]);

      expect(doRefresh).toHaveBeenCalledTimes(1);
      expect(result1).toEqual({ accessToken: "new-at", refreshToken: "new-rt" });
      expect(result2).toEqual({ accessToken: "new-at", refreshToken: "new-rt" });
    });

    it("still serves late arrivals from the grace cache within the TTL, and stops after it expires", async () => {
      const doRefresh = vi
        .fn()
        .mockResolvedValueOnce({ accessToken: "at-1", refreshToken: "rt-new" })
        .mockResolvedValueOnce({ accessToken: "should-not", refreshToken: "happen" });

      const first = await refreshOnce("rt-grace", doRefresh);
      await vi.advanceTimersByTimeAsync(10_000); // still inside the 30s grace window
      const late = await refreshOnce("rt-grace", doRefresh);

      expect(doRefresh).toHaveBeenCalledTimes(1);
      expect(first).toEqual({ accessToken: "at-1", refreshToken: "rt-new" });
      expect(late).toEqual({ accessToken: "at-1", refreshToken: "rt-new" });

      // Past the grace window the cached rotation must no longer be served
      // (and the entry must already be gone, evicted by the sweep).
      await vi.advanceTimersByTimeAsync(30_000);
      expect(getRecentMapSizeForTests()).toBe(0);
    });
  });
});
