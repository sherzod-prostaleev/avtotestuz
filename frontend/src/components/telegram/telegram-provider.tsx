"use client";

import { createContext, useContext, useEffect, useState } from "react";
import { getWebApp, isTelegramMiniApp, TELEGRAM_SDK_URL, type TelegramWebApp } from "@/lib/telegram/web-app";

const TelegramContext = createContext<TelegramWebApp | null>(null);

// null means "not Telegram", "SDK still loading" or "SDK failed": consumers
// must treat it as the plain website and never wait for it.
export function useTelegram(): TelegramWebApp | null {
  return useContext(TelegramContext);
}

/**
 * Loads Telegram's SDK only when Telegram launched us. On the website this
 * renders its children and does nothing else — no script request, no work.
 * Detection lives in effects so server and first client render match.
 */
export function TelegramProvider({ children }: { children: React.ReactNode }) {
  const [webApp, setWebApp] = useState<TelegramWebApp | null>(null);

  useEffect(() => {
    if (!isTelegramMiniApp()) return;
    const existing = getWebApp();
    if (existing) {
      setWebApp(existing);
      return;
    }
    let script = document.querySelector<HTMLScriptElement>(`script[src="${TELEGRAM_SDK_URL}"]`);
    if (!script) {
      script = document.createElement("script");
      script.src = TELEGRAM_SDK_URL;
      script.async = true;
      document.head.appendChild(script);
    }
    const onLoad = () => setWebApp(getWebApp());
    // A blocked/failed SDK leaves webApp null: the app stays a normal web page.
    script.addEventListener("load", onLoad);
    return () => script?.removeEventListener("load", onLoad);
  }, []);

  useEffect(() => {
    if (!webApp) return;
    document.documentElement.classList.add("tg-webapp");
    webApp.ready();
    webApp.expand();
    if (webApp.isVersionAtLeast("7.7")) webApp.disableVerticalSwipes?.();
  }, [webApp]);

  // Task 9 mounts <TelegramChrome /> here, next to the children.
  return <TelegramContext.Provider value={webApp}>{children}</TelegramContext.Provider>;
}
