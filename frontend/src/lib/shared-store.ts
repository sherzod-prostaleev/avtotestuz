"use client";

import { useEffect, useState } from "react";

export interface SharedStore<T> {
  load(options?: { force?: boolean }): Promise<T>;
  set(value: T): void;
  subscribe(listener: (value: T) => void): () => void;
  get(): T;
  reset(): void;
}

export function createSharedStore<T>(
  fetcher: () => Promise<T>,
  initialValue: T,
  { ttlMs = 30_000 }: { ttlMs?: number } = {},
): SharedStore<T> {
  let value = initialValue;
  let fetchedAt = 0;
  let inFlight: Promise<T> | null = null;
  const listeners = new Set<(value: T) => void>();

  const publish = (next: T) => {
    value = next;
    fetchedAt = Date.now();
    for (const listener of listeners) listener(next);
  };

  const store: SharedStore<T> = {
    load({ force = false } = {}) {
      const isTest = typeof process !== "undefined" && process.env?.NODE_ENV === "test";
      if (inFlight) return inFlight;
      if (!force && !isTest && fetchedAt > 0 && Date.now() - fetchedAt < ttlMs) {
        return Promise.resolve(value);
      }

      const request = fetcher()
        .then((next) => {
          publish(next);
          return next;
        })
        .catch((err) => {
          if (!force && !isTest && fetchedAt > 0) return value;
          throw err;
        })
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
      return () => {
        listeners.delete(listener);
      };
    },

    get() {
      return value;
    },

    reset() {
      value = initialValue;
      fetchedAt = 0;
      inFlight = null;
    },
  };

  return store;
}

export function useSharedStore<T>(store: SharedStore<T>): T {
  const [value, setValue] = useState<T>(() => store.get());

  useEffect(() => store.subscribe(setValue), [store]);

  return value;
}
