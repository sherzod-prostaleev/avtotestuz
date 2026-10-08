"use client";

import { createContext, useContext, useEffect, useState } from "react";
import dynamic from "next/dynamic";
import { getWebApp, isTelegramMiniApp, TELEGRAM_SDK_URL, type TelegramWebApp } from "@/lib/telegram/web-app";

// Only a Telegram launch ever renders the chrome, so the website's shared
// bundle must not carry it (BackButton, link interception, history depth,
// frame colours, the hint). No SSR: it needs the SDK object anyway.
const TelegramChrome = dynamic(() => import("./telegram-chrome").then((mod) => mod.TelegramChrome), {
  ssr: false,
});

export type TelegramStatus = "off" | "loading" | "ready" | "failed";
export type TelegramColorScheme = "light" | "dark";

const TelegramContext = createContext<TelegramWebApp | null>(null);
const TelegramStatusContext = createContext<TelegramStatus>("off");
const TelegramColorSchemeContext = createContext<TelegramColorScheme | null>(null);

function knownScheme(value: unknown): TelegramColorScheme | null {
  return value === "light" || value === "dark" ? value : null;
}

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
 * Telegram's current light/dark scheme (spec D4), kept live through
 * themeChanged; null on the website, before the SDK is ready, or for a scheme
 * we do not know.
 */
export function useTelegramColorScheme(): TelegramColorScheme | null {
  return useContext(TelegramColorSchemeContext);
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
  const [colorScheme, setColorScheme] = useState<TelegramColorScheme | null>(null);

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

  useEffect(() => {
    if (!webApp) return;
    const sync = () => setColorScheme(knownScheme(webApp.colorScheme));
    sync();
    webApp.onEvent("themeChanged", sync);
    return () => webApp.offEvent("themeChanged", sync);
  }, [webApp]);

  // The chrome exists only once Telegram launched us: on the website, and
  // while the SDK loads or after it fails, nothing Telegram-specific runs.
  return (
    <TelegramStatusContext.Provider value={status}>
      <TelegramContext.Provider value={webApp}>
        <TelegramColorSchemeContext.Provider value={colorScheme}>
          {webApp && <TelegramChrome webApp={webApp} colorScheme={colorScheme} />}
          {children}
        </TelegramColorSchemeContext.Provider>
      </TelegramContext.Provider>
    </TelegramStatusContext.Provider>
  );
}
