"use client";

import { useEffect, useRef } from "react";
import { useTelegram } from "./telegram-provider";

/**
 * Shows Telegram's BackButton while the caller is mounted and routes it (and
 * Android's hardware back key, which Telegram maps onto it) to `onBack`. For
 * in-page sub-screens on a tab root — e.g. the phone profile's panels — where
 * the chrome keeps Back hidden because the route itself has nothing behind it.
 * No-op on the website.
 */
export function useTelegramBackButton(onBack: () => void): void {
  const webApp = useTelegram();
  // The latest callback without re-registering on every render.
  const latest = useRef(onBack);
  useEffect(() => {
    latest.current = onBack;
  }, [onBack]);

  useEffect(() => {
    if (!webApp) return;
    const handler = () => latest.current();
    try {
      webApp.BackButton.onClick(handler);
      webApp.BackButton.show();
    } catch {
      /* client too old for the BackButton: the on-screen arrow remains */
    }
    return () => {
      try {
        webApp.BackButton.offClick(handler);
        webApp.BackButton.hide();
      } catch {
        /* see above */
      }
    };
  }, [webApp]);
}
