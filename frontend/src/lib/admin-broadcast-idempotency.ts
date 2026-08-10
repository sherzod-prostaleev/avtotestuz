/**
 * Ensures one logical admin broadcast SEND reuses a single Idempotency-Key
 * across double-clicks, double-submits, and network/browser retries.
 * A new UUID is minted only after a successful create (or explicit reset).
 */
export type BroadcastIdempotencySession = {
  /** Start a send attempt. skip=true means another attempt is already in flight. */
  begin: () => { key: string; skip: boolean };
  /** Clear the key after the server accepted the campaign. */
  succeed: () => void;
  /** Keep the key for retry after a failed/network attempt. */
  failKeepKey: () => void;
  /** Current key (null when idle after success). */
  currentKey: () => string | null;
  /** Force a fresh key (e.g. after intentional cancel of the draft). */
  reset: () => void;
};

export function createBroadcastIdempotencySession(
  createKey: () => string = () => globalThis.crypto.randomUUID(),
): BroadcastIdempotencySession {
  let key: string | null = null;
  let inFlight = false;

  return {
    begin() {
      if (inFlight && key) {
        return { key, skip: true };
      }
      if (!key) {
        key = createKey();
      }
      inFlight = true;
      return { key, skip: false };
    },
    succeed() {
      key = null;
      inFlight = false;
    },
    failKeepKey() {
      inFlight = false;
    },
    currentKey() {
      return key;
    },
    reset() {
      key = null;
      inFlight = false;
    },
  };
}
