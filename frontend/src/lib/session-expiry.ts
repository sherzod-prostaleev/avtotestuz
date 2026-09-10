/**
 * One place that hears "this browser's session is gone".
 *
 * Before this existed the app had exactly one 401 handler: the `/me` query
 * behind MustChangePasswordGate. That query is mounted once per tab in the
 * (app) layout and — with `retry: false`, `refetchOnWindowFocus: false` and
 * nothing polling it — is fetched exactly once, at mount. So a session that
 * died AFTER the tab was opened (the learner signed in somewhere else, the
 * refresh token aged out, a reuse was detected) was never noticed: every other
 * request answered 401 and each caller painted its own local "could not load"
 * line. The learner sat on a fully drawn dashboard full of error text with no
 * way to understand it short of a hard reload.
 *
 * The transport layer is the only layer that sees ALL of those 401s, so it
 * announces them here and SessionExpiredGate — mounted only on the learner
 * shells, never on the kiosk or the public site — turns the first one into a
 * trip to the login screen.
 */

type Listener = () => void;

const listeners = new Set<Listener>();

/** Subscribe to session-expiry announcements. Returns the unsubscribe. */
export function onSessionExpired(listener: Listener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/**
 * Announce that a request came back unauthorized.
 *
 * Iterates a copy: a listener is free to unsubscribe itself while it runs,
 * which is exactly what the gate does once it has decided to redirect. A
 * throwing listener must not swallow the announcement for the others, nor
 * turn a plain API error into a crash inside `fetch`.
 */
export function notifySessionExpired(): void {
  for (const listener of [...listeners]) {
    try {
      listener();
    } catch {
      /* a broken subscriber is not the API call's problem */
    }
  }
}

/** Test helper — drops every subscriber. */
export function resetSessionExpiryListenersForTests(): void {
  listeners.clear();
}
