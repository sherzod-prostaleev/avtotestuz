export interface TelegramWebApp {
  initData: string;
  initDataUnsafe: { user?: { id: number; first_name?: string; username?: string; language_code?: string } };
  colorScheme: "light" | "dark";
  version: string;
  platform: string;
  ready(): void;
  expand(): void;
  close(): void;
  isVersionAtLeast(v: string): boolean;
  disableVerticalSwipes?(): void;
  // Bot API 8.0+; absent on older clients.
  requestFullscreen?(): void;
  isFullscreen?: boolean;
  enableClosingConfirmation(): void;
  disableClosingConfirmation(): void;
  setHeaderColor(color: string): void;
  setBackgroundColor(color: string): void;
  setBottomBarColor?(color: string): void;
  // "fullscreenChanged" (8.0+) is one of the events: isFullscreen flips, and
  // Telegram republishes the --tg-*safe-area-inset-* variables.
  onEvent(event: string, cb: () => void): void;
  offEvent(event: string, cb: () => void): void;
  openLink(url: string): void;
  openTelegramLink(url: string): void;
  // `response` is the raw query string Telegram signed (contact JSON +
  // auth_date + hash); `responseUnsafe` is the SDK's unverified parse of it.
  requestContact(
    cb: (
      shared: boolean,
      res?: { response?: string; responseUnsafe?: { contact?: { phone_number?: string } } },
    ) => void,
  ): void;
  BackButton: { show(): void; hide(): void; onClick(cb: () => void): void; offClick(cb: () => void): void };
  HapticFeedback: {
    impactOccurred(style: "light" | "medium" | "heavy" | "rigid" | "soft"): void;
    notificationOccurred(type: "error" | "success" | "warning"): void;
    selectionChanged(): void;
  };
  // Absent before Bot API 6.9, so every caller goes through the cloud* helpers.
  CloudStorage?: {
    getItem(key: string, cb: (err: string | null, value?: string) => void): void;
    setItem(key: string, value: string, cb?: (err: string | null, ok?: boolean) => void): void;
    removeItem(key: string, cb?: (err: string | null, ok?: boolean) => void): void;
  };
}

export const TELEGRAM_SDK_URL = "https://telegram.org/js/telegram-web-app.js";
export const TG_SESSION_FLAG = "tg-webapp";
export const AUTOLOGIN_OFF_KEY = "autologin_off";

declare global {
  interface Window {
    Telegram?: { WebApp?: TelegramWebApp };
  }
}

/**
 * Whether a Telegram client actually hosts this page. These are exactly the
 * three channels telegram-web-app.js's own postEvent() can talk through:
 * `TelegramWebviewProxy` (Android, iOS, new Desktop), `window.external.notify`
 * (legacy Desktop) and a parent frame (web.telegram.org). "Framed" is only a
 * safe signal because of CSP frame-ancestors: learner pages can be framed by
 * web.telegram.org or same-origin alone, and nothing same-origin frames them,
 * so a hostile page cannot fake a host by embedding us.
 *
 * The #tgWebAppData hash is NOT a host: anyone can paste their own fresh
 * launch data into a link, and in a plain browser the SDK happily reads it
 * into WebApp.initData. Trusting it let an attacker's Telegram be linked to a
 * victim who signed in through such a link.
 */
export function hasTelegramHost(): boolean {
  if (typeof window === "undefined") return false;
  const w = window as Window & { TelegramWebviewProxy?: unknown };
  if (w.TelegramWebviewProxy !== undefined) return true;
  try {
    const external = (window as { external?: unknown }).external;
    if (external && typeof external === "object" && "notify" in external) return true;
  } catch {
    /* some engines throw on window.external access: not a Telegram host */
  }
  try {
    return window.parent != null && window.parent !== window;
  } catch {
    // A cross-origin parent can throw on access; it still is a parent frame.
    return true;
  }
}

/**
 * Remembers, for this webview's lifetime, that Telegram launched us — only
 * when a Telegram client is really around, so a planted link cannot turn a
 * plain browser tab into a "Mini App" for the rest of the session.
 */
export function markTelegramMiniApp(): void {
  if (!hasTelegramHost()) return;
  try {
    sessionStorage.setItem(TG_SESSION_FLAG, "1");
  } catch {
    /* storage blocked: the hash check below still works on the launch page */
  }
}

export function isTelegramMiniApp(): boolean {
  if (!hasTelegramHost()) return false;
  try {
    if (sessionStorage.getItem(TG_SESSION_FLAG) === "1") return true;
  } catch {
    /* fall through */
  }
  return window.location.hash.includes("tgWebAppData=");
}

/** The SDK object, but only when a Telegram client actually launched us. */
export function getWebApp(): TelegramWebApp | null {
  if (!hasTelegramHost()) return null;
  const webApp = window.Telegram?.WebApp;
  return webApp && webApp.initData ? webApp : null;
}

// CloudStorage answers over the Telegram bridge; on a bad link it can stay
// silent forever, and callers sit on a spinner until it does.
const CLOUD_GET_TIMEOUT_MS = 3000;

export type CloudGetResult = { status: "ok"; value: string | null } | { status: "unavailable" };

/**
 * Like cloudGet, but a timeout or an error is "unavailable", not "no value":
 * a caller reading a sign-out flag must not mistake a slow bridge for "never
 * signed out". A client without CloudStorage is "ok, null": nothing could ever
 * have been stored there.
 */
export function cloudGetResult(key: string): Promise<CloudGetResult> {
  const storage = getWebApp()?.CloudStorage;
  if (!storage) return Promise.resolve({ status: "ok", value: null });
  return new Promise((resolve) => {
    const timer = setTimeout(() => resolve({ status: "unavailable" }), CLOUD_GET_TIMEOUT_MS);
    const done = (result: CloudGetResult) => {
      clearTimeout(timer);
      resolve(result);
    };
    try {
      storage.getItem(key, (err, value) =>
        done(err ? { status: "unavailable" } : { status: "ok", value: value || null })
      );
    } catch {
      done({ status: "unavailable" });
    }
  });
}

export async function cloudGet(key: string): Promise<string | null> {
  const result = await cloudGetResult(key);
  return result.status === "ok" ? result.value : null;
}

/**
 * Helper to wrap a CloudStorage write (setItem or removeItem) with a timeout.
 * Always resolves (never throws) and clears its timer, ensuring a stalled bridge
 * never blocks the app.
 */
function withCloudWriteTimeout(
  call: (cb: (err: string | null, ok?: boolean) => void) => void
): Promise<void> {
  return new Promise((resolve) => {
    const timer = setTimeout(() => resolve(), CLOUD_GET_TIMEOUT_MS);
    const done = () => {
      clearTimeout(timer);
      resolve();
    };
    try {
      call((err) => done());
    } catch {
      done();
    }
  });
}

export function cloudSet(key: string, value: string): Promise<void> {
  const storage = getWebApp()?.CloudStorage;
  if (!storage) return Promise.resolve();
  return withCloudWriteTimeout((cb) => storage.setItem(key, value, cb));
}

export function cloudRemove(key: string): Promise<void> {
  const storage = getWebApp()?.CloudStorage;
  if (!storage) return Promise.resolve();
  return withCloudWriteTimeout((cb) => storage.removeItem(key, cb));
}
