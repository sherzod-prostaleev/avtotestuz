/**
 * How many in-app history entries sit behind the current one.
 *
 * Telegram's BackButton must never call history.back() past the first screen:
 * in the web.telegram.org iframe that walks the PARENT page's history, and on
 * phones it does nothing at all — either way the learner is stuck or thrown
 * out. history.length cannot tell (it counts forward entries and, in an
 * iframe, the parent's), and Next rebuilds history.state on every navigation,
 * dropping custom keys. So each entry is stamped as it is written: a push is
 * one deeper than the entry it leaves, a replace keeps the depth. Next's
 * popstate restore spreads the existing state, so the stamp survives Back.
 */
const KEY = "__tgDepth";

type HistoryState = Record<string, unknown> | null | undefined;

function depthOf(state: HistoryState): number {
  const value = state?.[KEY];
  return typeof value === "number" && value >= 0 ? value : 0;
}

function stamp(data: unknown, depth: number): Record<string, unknown> {
  const base = data && typeof data === "object" ? (data as Record<string, unknown>) : {};
  return { ...base, [KEY]: depth };
}

export function canGoBackInApp(): boolean {
  if (typeof window === "undefined") return false;
  return depthOf(window.history.state as HistoryState) > 0;
}

/**
 * Wraps pushState/replaceState for as long as the Mini App chrome is mounted.
 * The entry we start on is depth 0 unless it was already stamped (a reload).
 * Returns the undo; if someone wrapped the methods after us, our wrappers stay
 * in their chain but go inert, so their wrapping is never torn out.
 */
export function installHistoryDepth(): () => void {
  const history = window.history;
  const originalPush = history.pushState;
  const originalReplace = history.replaceState;
  let active = true;

  const current = history.state as HistoryState;
  originalReplace.call(history, stamp(current, depthOf(current)), "");

  const push: History["pushState"] = function (data, unused, url) {
    if (!active) return originalPush.call(history, data, unused, url);
    return originalPush.call(history, stamp(data, depthOf(history.state as HistoryState) + 1), unused, url);
  };
  const replace: History["replaceState"] = function (data, unused, url) {
    if (!active) return originalReplace.call(history, data, unused, url);
    return originalReplace.call(history, stamp(data, depthOf(history.state as HistoryState)), unused, url);
  };
  history.pushState = push;
  history.replaceState = replace;

  return () => {
    active = false;
    if (history.pushState === push) history.pushState = originalPush;
    if (history.replaceState === replace) history.replaceState = originalReplace;
  };
}
