import { useSyncExternalStore } from "react";

/**
 * A one-line, self-dismissing note shown by the Telegram chrome — e.g. why
 * Telegram's phone-share sheet is about to open after a sign-in that already
 * navigated away from the form. A tiny store rather than React state because
 * the caller (a fire-and-forget follow-up) outlives the page that started it;
 * the chrome, mounted only inside Telegram, is what renders it.
 */
export type TelegramHint = { id: number; text: string };

let current: TelegramHint | null = null;
let nextId = 1;
const listeners = new Set<() => void>();

function emit() {
  listeners.forEach((listener) => listener());
}

export function showTelegramHint(text: string): void {
  current = { id: nextId++, text };
  emit();
}

/** Dismisses `id` only if it is still the hint on screen. */
export function dismissTelegramHint(id?: number): void {
  if (!current || (id !== undefined && current.id !== id)) return;
  current = null;
  emit();
}

export function currentTelegramHint(): TelegramHint | null {
  return current;
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useTelegramHint(): TelegramHint | null {
  return useSyncExternalStore(subscribe, currentTelegramHint, () => null);
}
