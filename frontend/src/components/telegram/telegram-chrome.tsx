"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useTheme } from "next-themes";
import type { TelegramWebApp } from "@/lib/telegram/web-app";
import { cssColorToHex } from "@/lib/telegram/color";
import { canGoBackInApp, installHistoryDepth } from "@/lib/telegram/history-depth";
import { telegramLinkKind } from "@/lib/telegram/links";
import { isTabRoot, localeOf, needsClosingGuard } from "@/lib/telegram/routes";

// Telegram's own method calls throw on clients too old for them; the chrome is
// decoration and must never take the page down with it.
function attempt(fn: () => void): void {
  try {
    fn();
  } catch {
    /* unsupported on this client */
  }
}

/**
 * Makes the app behave like a native Mini App: Telegram's theme, frame colour,
 * BackButton, closing guard and link handling. Mounted by TelegramProvider
 * only when Telegram launched us, so none of this runs on the website.
 */
export function TelegramChrome({ webApp }: { webApp: TelegramWebApp }) {
  const pathname = usePathname() ?? "/";
  const router = useRouter();
  const { setTheme, resolvedTheme } = useTheme();

  // Follow Telegram's light/dark scheme (spec D4) and keep following it.
  // setTheme persists to localStorage("theme"), but the webview's storage is
  // its own (Android WebView, WKWebView, Telegram Desktop, a partitioned
  // web.telegram.org iframe), so the website's saved choice is untouched; and
  // since Telegram wins on every launch, what is stored here never matters.
  useEffect(() => {
    const apply = () => {
      const scheme = webApp.colorScheme;
      if (scheme === "light" || scheme === "dark") setTheme(scheme);
    };
    apply();
    webApp.onEvent("themeChanged", apply);
    return () => webApp.offEvent("themeChanged", apply);
  }, [setTheme, webApp]);

  // Paint Telegram's header/background with our page colour so the frame and
  // the page read as one surface. next-themes swaps the <html> class in its
  // own effect, which runs after this child's, so read the token a frame later.
  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      let hex: string | null = null;
      try {
        hex = cssColorToHex(getComputedStyle(document.documentElement).getPropertyValue("--background"));
      } catch {
        hex = null;
      }
      if (!hex) return;
      const color = hex;
      // Hex colours arrived in Bot API 6.1 for the background but 6.9 for the
      // header; the bottom bar only exists from 7.10.
      if (webApp.isVersionAtLeast("6.9")) attempt(() => webApp.setHeaderColor(color));
      if (webApp.isVersionAtLeast("6.1")) attempt(() => webApp.setBackgroundColor(color));
      if (webApp.isVersionAtLeast("7.10")) attempt(() => webApp.setBottomBarColor?.(color));
    });
    return () => cancelAnimationFrame(frame);
  }, [resolvedTheme, webApp]);

  useEffect(() => installHistoryDepth(), []);

  // One handler at a time: each path change offClicks the previous one, so a
  // tap never fires a stale route's handler or two handlers at once. Running
  // tests hide it too: their own exit control asks before abandoning the
  // attempt, and a Telegram Back would skip that question. With it hidden,
  // Android's back key tries to close the app and meets the closing guard.
  useEffect(() => {
    if (isTabRoot(pathname) || needsClosingGuard(pathname)) {
      attempt(() => webApp.BackButton.hide());
      return;
    }
    const onBack = () => {
      if (canGoBackInApp()) router.back();
      // The launch screen (e.g. /tg?next=… replaced into it) has nothing
      // behind it inside the app; history.back() would leave the Mini App.
      else router.replace(`/${localeOf(pathname)}/dashboard`);
    };
    attempt(() => webApp.BackButton.onClick(onBack));
    attempt(() => webApp.BackButton.show());
    return () => attempt(() => webApp.BackButton.offClick(onBack));
  }, [pathname, router, webApp]);

  useEffect(() => {
    if (needsClosingGuard(pathname)) attempt(() => webApp.enableClosingConfirmation());
    else attempt(() => webApp.disableClosingConfirmation());
  }, [pathname, webApp]);

  // Inside Telegram a plain external <a> would navigate the webview away from
  // the app (and payment pages refuse to be framed on Telegram Web). Modified
  // and non-primary clicks, downloads and same-origin links stay native.
  useEffect(() => {
    const onClick = (event: MouseEvent) => {
      if (event.defaultPrevented || event.button !== 0) return;
      if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      const target = event.target;
      if (!(target instanceof Element)) return;
      const anchor = target.closest("a[href]");
      if (!(anchor instanceof HTMLAnchorElement) || anchor.hasAttribute("download")) return;
      const kind = telegramLinkKind(anchor.href, window.location.href);
      if (!kind) return;
      event.preventDefault();
      if (kind === "telegram") attempt(() => webApp.openTelegramLink(anchor.href));
      else attempt(() => webApp.openLink(anchor.href));
    };
    // Bubble phase, after React's root listener: a component that handles its
    // own link click and calls preventDefault keeps that click.
    document.addEventListener("click", onClick);
    return () => document.removeEventListener("click", onClick);
  }, [webApp]);

  return null;
}
