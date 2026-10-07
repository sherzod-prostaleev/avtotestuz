import { getWebApp } from "./web-app";

// Every call is a no-op outside Telegram, so shared components can call these
// unconditionally. Old clients throw on unsupported methods; feedback is
// decoration, so it must never break the caller.
function safely(fn: (h: NonNullable<ReturnType<typeof getWebApp>>["HapticFeedback"]) => void) {
  try {
    const haptic = getWebApp()?.HapticFeedback;
    if (haptic) fn(haptic);
  } catch {
    /* unsupported on this client */
  }
}

export const haptics = {
  select() {
    safely((h) => h.selectionChanged());
  },
  result(correct: boolean) {
    safely((h) => h.notificationOccurred(correct ? "success" : "error"));
  },
  impact() {
    safely((h) => h.impactOccurred("light"));
  },
};
