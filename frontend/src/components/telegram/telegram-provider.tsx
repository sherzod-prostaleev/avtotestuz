"use client";

import { createContext, useContext, useEffect, useState } from "react";
import { getWebApp, isTelegramMiniApp, TELEGRAM_SDK_URL, type TelegramWebApp } from "@/lib/telegram/web-app";

export type TelegramStatus = "off" | "loading" | "ready" | "failed";

const TelegramContext = createContext<TelegramWebApp | null>(null);
const TelegramStatusContext = createContext<TelegramStatus>("off");

// A blocked or hung SDK must not trap the learner on a disabled form.
const SDK_TIMEOUT_MS = 10_000;

// null means "not Telegram", "SDK still loading" or "SDK failed": consumers
// must treat it as the plain website and never wait for it.
export function useTelegram(): TelegramWebApp | null {
  return useContext(TelegramContext);
}

/**
 * "loading" is the window in which useTelegram() is still null inside the Mini
 * App: submitting then would skip tg_init_data and get lax cookies that the
 * web.telegram.org iframe never sends. Forms wait while it lasts; "failed"
 * (error, empty initData, timeout) falls back to the plain website flow.
 */
export function useTelegramStatus(): TelegramStatus {
  return useContext(TelegramStatusContext);
}

/**
 * Loads Telegram's SDK only when Telegram launched us. On the website this
 * renders its children and does nothing else — no script request, no work.
 * Detection lives in effects so server and first client render match.
 */
export function TelegramProvider({ children }: { children: React.ReactNode }) {
  const [webApp, setWebApp] = useState<TelegramWebApp | null>(null);
  // "off" on the server and the first client render; the effect below decides.
  const [status, setStatus] = useState<TelegramStatus>("off");

  useEffect(() => {
    if (!isTelegramMiniApp()) return;
    const resolve = (app: TelegramWebApp | null) => {
      if (app?.initData) {
        setWebApp(app);
        setStatus("ready");
      } else {
        setStatus("failed");
      }
    };
    const existing = getWebApp();
    if (existing) {
      resolve(existing);
      return;
    }
    setStatus("loading");
    let script = document.querySelector<HTMLScriptElement>(`script[src="${TELEGRAM_SDK_URL}"]`);
    if (!script) {
      script = document.createElement("script");
      script.src = TELEGRAM_SDK_URL;
      script.async = true;
      document.head.appendChild(script);
    }
    const timer = window.setTimeout(() => setStatus((s) => (s === "loading" ? "failed" : s)), SDK_TIMEOUT_MS);
    const onLoad = () => {
      window.clearTimeout(timer);
      resolve(getWebApp());
    };
    // A blocked/failed SDK leaves webApp null: the app stays a normal web page.
    const onError = () => {
      window.clearTimeout(timer);
      setStatus("failed");
    };
    script.addEventListener("load", onLoad);
    script.addEventListener("error", onError);
    return () => {
      window.clearTimeout(timer);
      script?.removeEventListener("load", onLoad);
      script?.removeEventListener("error", onError);
    };
  }, []);

  useEffect(() => {
    if (!webApp) return;
    document.documentElement.classList.add("tg-webapp");
    webApp.ready();
    webApp.expand();
    if (webApp.isVersionAtLeast("7.7")) webApp.disableVerticalSwipes?.();
  }, [webApp]);

  // Task 9 mounts <TelegramChrome /> here, next to the children.
  return (
    <TelegramStatusContext.Provider value={status}>
      <TelegramContext.Provider value={webApp}>{children}</TelegramContext.Provider>
    </TelegramStatusContext.Provider>
  );
}
