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

function readRecent(refreshToken: string): RefreshedTokens | null {
  const hit = recentByOldToken.get(refreshToken);
  if (!hit) return null;
  if (hit.until <= Date.now()) {
    recentByOldToken.delete(refreshToken);
    return null;
  }
  return hit.tokens;
}

/** Test helper — clears in-flight + grace caches. */
export function resetRefreshLockForTests(): void {
  inFlightByToken.clear();
  recentByOldToken.clear();
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
