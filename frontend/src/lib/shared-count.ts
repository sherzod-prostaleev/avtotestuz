"use client";

import { useEffect, useState } from "react";

/**
 * A badge count that several mounted components read from one request.
 *
 * The notification bell is rendered twice — once for the mobile top bar, once
 * for the desktop sidebar — and CSS hides whichever does not apply. Both were
 * fetching on their own, and both refetched on every navigation, so changing
 * page cost two `me/notifications/unread-count` calls on top of the sidebar's
 * `me/support/unread`. Those are the two most-requested endpoints on the
 * platform, and each one queues behind the others on the BFF's event loop.
 *
 * A shared store collapses that three ways: concurrent readers join one
 * in-flight request, a read inside the TTL is served from memory, and the
 * poll timer belongs to the store rather than to each subscriber — so two
 * mounted bells still produce exactly one request per interval.
 */
export interface SharedCount {
  /** Fetch if stale, join the in-flight request otherwise. */
  load(options?: { force?: boolean }): Promise<number>;
  /** Publish a value the caller already knows (e.g. after marking as read). */
  set(value: number): void;
  subscribe(listener: (value: number) => void): () => void;
  get(): number;
  /** Drop cached state — call on sign-out so the next reader refetches. */
  reset(): void;
}

export function createSharedCount(
  fetcher: () => Promise<number>,
  /** `pollMs: 0` (the default) keeps the count navigation-driven only. */
  { ttlMs, pollMs = 0 }: { ttlMs: number; pollMs?: number },
): SharedCount {
  let value = 0;
  let fetchedAt = 0;
  let inFlight: Promise<number> | null = null;
  let timer: ReturnType<typeof setInterval> | null = null;
  const listeners = new Set<(value: number) => void>();

  const publish = (next: number) => {
    value = next;
    fetchedAt = Date.now();
    for (const listener of listeners) listener(next);
  };

  const store: SharedCount = {
    load({ force = false } = {}) {
      if (inFlight) return inFlight;
      if (!force && Date.now() - fetchedAt < ttlMs) return Promise.resolve(value);

      const request = fetcher()
        .then((next) => {
          publish(next);
          return next;
        })
        // The badge is best-effort: a failed poll keeps the last known value
        // rather than blanking the badge or surfacing an error.
        .catch(() => value)
        .finally(() => {
          inFlight = null;
        });

      inFlight = request;
      return request;
    },

    set(next) {
      publish(next);
    },

    subscribe(listener) {
      listeners.add(listener);
      if (pollMs > 0 && timer === null) {
        timer = setInterval(() => void store.load({ force: true }), pollMs);
      }
      return () => {
        listeners.delete(listener);
        if (listeners.size === 0 && timer !== null) {
          clearInterval(timer);
          timer = null;
        }
      };
    },

    get() {
      return value;
    },

    reset() {
      value = 0;
      fetchedAt = 0;
    },
  };

  return store;
}

/**
 * Subscribe to a shared count and refresh it on navigation. The refresh is
 * TTL-guarded, so moving between pages inside the window costs nothing, and
 * the store's own timer keeps the badge current after that.
 */
export function useSharedCount(store: SharedCount, pathname: string | null): number {
  const [value, setValue] = useState(() => store.get());

  useEffect(() => store.subscribe(setValue), [store]);

  useEffect(() => {
    void store.load();
  }, [store, pathname]);

  return value;
}
