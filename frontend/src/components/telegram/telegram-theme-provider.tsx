"use client";

import { useState } from "react";
import { ThemeProvider } from "next-themes";
import { earlyTelegramScheme } from "@/lib/telegram/boot-script";
import { hasTelegramHost } from "@/lib/telegram/web-app";
import { useTelegramColorScheme } from "./telegram-provider";

/**
 * The site's next-themes provider. Inside the Mini App it is forced to
 * Telegram's colour scheme (spec D4) and follows themeChanged live: a forced
 * theme is applied to <html> but never persisted, so the Mini App cannot
 * overwrite the learner's saved site theme in any storage it shares with the
 * website. On the website the scheme is null and nothing changes.
 *
 * Until the SDK reports its scheme, the one /tg's boot script read from the
 * launch hash is used, so hydration does not flip a light page back to the
 * saved (or default dark) site theme. Only differs from the server render in
 * forcedTheme, which next-themes' own script tolerates (suppressHydrationWarning).
 */
export function TelegramThemeProvider({ children }: { children: React.ReactNode }) {
  const colorScheme = useTelegramColorScheme();
  const [early] = useState(() => (hasTelegramHost() ? earlyTelegramScheme() : null));
  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="dark"
      enableSystem={false}
      storageKey="theme"
      disableTransitionOnChange
      value={{ light: "light", dark: "dark" }}
      forcedTheme={colorScheme ?? early ?? undefined}
    >
      {children}
    </ThemeProvider>
  );
}
