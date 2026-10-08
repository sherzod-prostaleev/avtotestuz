"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import type { TelegramWebApp } from "@/lib/telegram/web-app";
import { useSessionRunning, useSessionSettled } from "@/lib/session-running";
import { cssColorToHex } from "@/lib/telegram/color";
import { useTelegramFrameColor } from "@/lib/telegram/frame-color";
import { canGoBackInApp, installHistoryDepth } from "@/lib/telegram/history-depth";
import { telegramLinkKind } from "@/lib/telegram/links";
import { isTabRoot, localeOf, needsClosingGuard } from "@/lib/telegram/routes";
import { TelegramHint } from "./telegram-hint";

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
 * Makes the app behave like a native Mini App: Telegram's frame colour,
 * BackButton, closing guard and link handling. Mounted by TelegramProvider
 * only when Telegram launched us, so none of this runs on the website. The
 * theme itself is not set here: the provider wiring forces next-themes to
 * Telegram's `colorScheme` (forcedTheme), which never writes localStorage.
 * Loaded with next/dynamic, so none of this ships in the website's bundle.
 */
export function TelegramChrome({
  webApp,
  colorScheme,
}: {
  webApp: TelegramWebApp;
  colorScheme: "light" | "dark" | null;
}) {
  const pathname = usePathname() ?? "/";
  const router = useRouter();
  // The route alone cannot tell a running attempt from its result screen
  // (same /session/<id>), so the guard also needs the runner's own word.
  const sessionRunning = useSessionRunning();
  const sessionSettled = useSessionSettled();
  const onRunner = needsClosingGuard(pathname);
  const guarded = onRunner && sessionRunning;
  // On a runner Back appears only once it settled on a result or error
  // screen: shown during the load it would flash and vanish as the attempt
  // starts.
  const hideBack = isTabRoot(pathname) || guarded || (onRunner && !sessionSettled);
  // A full-screen view with its own fixed palette (the exam) beats the theme.
  const frameColor = useTelegramFrameColor();

  // Paint Telegram's header/background with our page colour so the frame and
  // the page read as one surface. next-themes swaps the <html> class in its
  // own effect, which runs after this one (the ThemeProvider subtree commits
  // after this sibling), so read the token a frame later.
  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      let hex: string | null = frameColor ? cssColorToHex(frameColor) : null;
      if (!hex) {
        try {
          hex = cssColorToHex(getComputedStyle(document.documentElement).getPropertyValue("--background"));
        } catch {
          hex = null;
        }
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
  }, [colorScheme, frameColor, webApp]);

  useEffect(() => installHistoryDepth(), []);

  // One handler at a time: each path change offClicks the previous one, so a
  // tap never fires a stale route's handler or two handlers at once. Running
  // tests hide it too: their own exit control asks before abandoning the
  // attempt, and a Telegram Back would skip that question. With it hidden,
  // Android's back key tries to close the app and meets the closing guard.
  // A finished attempt's result screen gets Back again.
  useEffect(() => {
    if (hideBack) {
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
  }, [hideBack, pathname, router, webApp]);

  useEffect(() => {
    if (guarded) attempt(() => webApp.enableClosingConfirmation());
    else attempt(() => webApp.disableClosingConfirmation());
  }, [guarded, webApp]);

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
      // Open first, cancel only once Telegram took it: a client too old for
      // the opener throws, and the click must then still go ahead natively.
      try {
        if (kind === "telegram") webApp.openTelegramLink(anchor.href);
        else webApp.openLink(anchor.href);
      } catch {
        return;
      }
      event.preventDefault();
    };
    // Bubble phase, after React's root listener: a component that handles its
    // own link click and calls preventDefault keeps that click.
    document.addEventListener("click", onClick);
    return () => document.removeEventListener("click", onClick);
  }, [webApp]);

  return <TelegramHint />;
}
