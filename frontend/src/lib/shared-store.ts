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
  let gen = 0;
  const listeners = new Set<(value: T) => void>();

  const publish = (next: T) => {
    value = next;
    fetchedAt = Date.now();
    for (const listener of listeners) listener(next);
  };

  const store: SharedStore<T> = {
    load({ force = false } = {}) {
      if (inFlight) return inFlight;
      if (!force && fetchedAt > 0 && Date.now() - fetchedAt < ttlMs) {
        return Promise.resolve(value);
      }

      const requestGen = gen;
      const request = fetcher()
        .then((next) => {
          if (requestGen === gen) {
            publish(next);
          }
          return next;
        })
        .catch((err) => {
          if (requestGen === gen && !force && fetchedAt > 0) return value;
          throw err;
        })
        .finally(() => {
          if (inFlight === request) {
            inFlight = null;
          }
        });

      inFlight = request;
      return request;
    },

    set(next) {
      gen += 1;
      inFlight = null;
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
      gen += 1;
      value = initialValue;
      fetchedAt = 0;
      inFlight = null;
      for (const listener of listeners) listener(initialValue);
    },
  };

  return store;
}

export function useSharedStore<T>(store: SharedStore<T>): T {
  const [value, setValue] = useState<T>(() => store.get());

  useEffect(() => store.subscribe(setValue), [store]);

  return value;
}
