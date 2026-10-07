export interface TelegramWebApp {
  initData: string;
  initDataUnsafe: { user?: { id: number; first_name?: string; language_code?: string } };
  colorScheme: "light" | "dark";
  version: string;
  platform: string;
  ready(): void;
  expand(): void;
  isVersionAtLeast(v: string): boolean;
  disableVerticalSwipes?(): void;
  enableClosingConfirmation(): void;
  disableClosingConfirmation(): void;
  setHeaderColor(color: string): void;
  setBackgroundColor(color: string): void;
  setBottomBarColor?(color: string): void;
  onEvent(event: string, cb: () => void): void;
  offEvent(event: string, cb: () => void): void;
  openLink(url: string): void;
  openTelegramLink(url: string): void;
  requestContact(
    cb: (shared: boolean, res?: { responseUnsafe?: { contact?: { phone_number?: string } } }) => void,
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

/** Remembers, for this webview's lifetime, that we were launched by Telegram. */
export function markTelegramMiniApp(): void {
  try {
    sessionStorage.setItem(TG_SESSION_FLAG, "1");
  } catch {
    /* storage blocked: the hash check below still works on the launch page */
  }
}

export function isTelegramMiniApp(): boolean {
  if (typeof window === "undefined") return false;
  try {
    if (sessionStorage.getItem(TG_SESSION_FLAG) === "1") return true;
  } catch {
    /* fall through */
  }
  return window.location.hash.includes("tgWebAppData=");
}

/** The SDK object, but only when Telegram actually launched us. */
export function getWebApp(): TelegramWebApp | null {
  if (typeof window === "undefined") return null;
  const webApp = window.Telegram?.WebApp;
  return webApp && webApp.initData ? webApp : null;
}

export function cloudGet(key: string): Promise<string | null> {
  const storage = getWebApp()?.CloudStorage;
  if (!storage) return Promise.resolve(null);
  return new Promise((resolve) => {
    try {
      storage.getItem(key, (err, value) => resolve(err ? null : value || null));
    } catch {
      resolve(null);
    }
  });
}

export function cloudSet(key: string, value: string): Promise<void> {
  const storage = getWebApp()?.CloudStorage;
  if (!storage) return Promise.resolve();
  return new Promise((resolve) => {
    try {
      storage.setItem(key, value, () => resolve());
    } catch {
      resolve();
    }
  });
}

export function cloudRemove(key: string): Promise<void> {
  const storage = getWebApp()?.CloudStorage;
  if (!storage) return Promise.resolve();
  return new Promise((resolve) => {
    try {
      storage.removeItem(key, () => resolve());
    } catch {
      resolve();
    }
  });
}
