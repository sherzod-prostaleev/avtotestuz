import type { TelegramWebApp } from "./web-app";

// Phones only: on desktop and web clients fullscreen turns a chat-side panel
// into a whole-screen takeover, which is not what opening the app means there.
const PHONE_PLATFORMS = new Set(["android", "ios"]);

/**
 * Opens the Mini App edge to edge on phones (Bot API 8.0+). Telegram then
 * overlays its close/menu controls on our top strip and publishes their
 * height as --tg-content-safe-area-inset-top, which globals.css adds to the
 * device inset (--tg-inset-top). Returns whether fullscreen was requested.
 */
export function requestPhoneFullscreen(webApp: TelegramWebApp): boolean {
  if (!PHONE_PLATFORMS.has(webApp.platform)) return false;
  if (!webApp.isVersionAtLeast("8.0") || webApp.isFullscreen) return false;
  if (typeof webApp.requestFullscreen !== "function") return false;
  try {
    webApp.requestFullscreen();
    return true;
  } catch {
    // An unsupported client throws WebAppMethodUnsupported; the app still
    // works expanded, just not fullscreen.
    return false;
  }
}
