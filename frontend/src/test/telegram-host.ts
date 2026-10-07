/**
 * Test-only: pretend a real Telegram client hosts the page. Detection requires
 * the bridge Telegram injects (`window.TelegramWebviewProxy`), never just the
 * #tgWebAppData hash, so every Mini App test installs one. vitest.setup.ts
 * removes it after each test.
 */
export function installTelegramHost(): void {
  (window as { TelegramWebviewProxy?: unknown }).TelegramWebviewProxy = { postEvent: () => {} };
}

export function removeTelegramHost(): void {
  delete (window as { TelegramWebviewProxy?: unknown }).TelegramWebviewProxy;
}
