export interface RefreshedTokens {
  accessToken: string;
  refreshToken: string;
}

let inFlight: Promise<RefreshedTokens | null> | null = null;

// Single-flight: concurrent callers share the SAME in-flight refresh call
// rather than each triggering their own — the backend rotates+revokes
// refresh tokens per use, so two concurrent refresh calls with the same
// token would make the backend treat the second as replay/theft and
// revoke ALL of the user's sessions (this exact bug happened in the
// Flutter-era AuthInterceptor before its single-flight fix).
export function refreshOnce(
  refreshToken: string,
  doRefresh: (rt: string) => Promise<RefreshedTokens | null>
): Promise<RefreshedTokens | null> {
  if (!inFlight) {
    inFlight = doRefresh(refreshToken).finally(() => {
      inFlight = null;
    });
  }
  return inFlight;
}
