export interface RefreshedTokens {
  accessToken: string;
  refreshToken: string;
}

// Keyed by the refresh-token VALUE, not a single global lock — this runs on
// the server proxying requests for every logged-in user concurrently. A
// single shared lock would let one user's concurrent request collapse into
// another user's in-flight refresh and receive THEIR rotated tokens back
// (cross-user session leakage). Keying by token scopes the single-flight
// dedup to "the same session refreshing concurrently", which is the only
// case that needs it.
const inFlightByToken = new Map<string, Promise<RefreshedTokens | null>>();

// After a successful rotation the old refresh token is revoked server-side.
// Parallel browser requests that still carry the OLD cookie (sent before
// Set-Cookie is applied) must NOT call /auth/refresh again — reuse detection
// revokes the entire session family and logs the user out. Cache the rotated
// pair briefly so late arrivals reuse the same result.
const recentByOldToken = new Map<string, { tokens: RefreshedTokens; until: number }>();
const ROTATION_GRACE_MS = 30_000;

// `readRecent` below only evicts a key when THAT SAME key is looked up
// again. In the normal case the old token is never presented a second time
// once rotated, so left alone every successful refresh would retain one
// entry (tokens + rotation record) for the life of the Node process —
// unbounded growth, and rotated token material kept in plaintext memory far
// longer than the 30s grace window that justifies caching it at all. A
// periodic sweep evicts by TIME instead, independent of reads, and a hard
// cap bounds memory even if a burst of refreshes outruns the sweep.
const RECENT_SWEEP_INTERVAL_MS = 10_000;
export const MAX_RECENT_ENTRIES = 1000;

let sweepTimer: ReturnType<typeof setInterval> | null = null;

function sweepExpired(): void {
  const now = Date.now();
  for (const [token, entry] of recentByOldToken) {
    if (entry.until <= now) {
      recentByOldToken.delete(token);
    }
  }
}

function ensureSweepTimer(): void {
  if (sweepTimer) return;
  sweepTimer = setInterval(sweepExpired, RECENT_SWEEP_INTERVAL_MS);
  // A background hygiene sweep must never be the reason a process (or a
  // serverless/route-handler runtime) can't exit — unref it so it only
  // fires while something else is already keeping the event loop alive.
  sweepTimer.unref();
}

function rememberRotation(oldToken: string, tokens: RefreshedTokens): void {
  if (recentByOldToken.size >= MAX_RECENT_ENTRIES) {
    // Make room by discarding entries that are already stale before falling
    // back to evicting a still-valid one.
    sweepExpired();
  }
  if (recentByOldToken.size >= MAX_RECENT_ENTRIES) {
    // Still full: a genuine burst outran the sweep. Map iteration order is
    // insertion order, and every entry is stamped `until = now +
    // ROTATION_GRACE_MS` with the same fixed grace window, so the
    // oldest-inserted entry is also the one closest to expiry — drop it.
    const oldestKey = recentByOldToken.keys().next().value;
    if (oldestKey !== undefined) {
      recentByOldToken.delete(oldestKey);
    }
  }
  recentByOldToken.set(oldToken, { tokens, until: Date.now() + ROTATION_GRACE_MS });
  ensureSweepTimer();
}

function readRecent(refreshToken: string): RefreshedTokens | null {
  const hit = recentByOldToken.get(refreshToken);
  if (!hit) return null;
  if (hit.until <= Date.now()) {
    recentByOldToken.delete(refreshToken);
    return null;
  }
  return hit.tokens;
}

/** Test helper — clears in-flight + grace caches and stops the sweep timer. */
export function resetRefreshLockForTests(): void {
  inFlightByToken.clear();
  recentByOldToken.clear();
  if (sweepTimer) {
    clearInterval(sweepTimer);
    sweepTimer = null;
  }
}

/** Test helper — current size of the rotation-grace cache. */
export function getRecentMapSizeForTests(): number {
  return recentByOldToken.size;
}

// Single-flight per refresh-token: concurrent callers presenting the SAME
// token share one in-flight refresh call rather than each triggering their
// own — the backend rotates+revokes refresh tokens per use, so two
// concurrent refresh calls with the same token would make the backend treat
// the second as replay/theft and revoke ALL of that session's tokens (this
// exact bug happened in the Flutter-era AuthInterceptor before its
// single-flight fix). Different tokens (different users/sessions) never
// share a lock and always refresh independently.
export function refreshOnce(
  refreshToken: string,
  doRefresh: (rt: string) => Promise<RefreshedTokens | null>
): Promise<RefreshedTokens | null> {
  const recent = readRecent(refreshToken);
  if (recent) {
    return Promise.resolve(recent);
  }

  let inFlight = inFlightByToken.get(refreshToken);
  if (!inFlight) {
    inFlight = doRefresh(refreshToken)
      .then((tokens) => {
        if (tokens) {
          recentByOldToken.set(refreshToken, {
            tokens,
            until: Date.now() + ROTATION_GRACE_MS,
          });
        }
        return tokens;
      })
      .finally(() => {
        inFlightByToken.delete(refreshToken);
      });
    inFlightByToken.set(refreshToken, inFlight);
  }
  return inFlight;
}
