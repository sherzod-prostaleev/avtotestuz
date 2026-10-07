"use client";

import { ThemeProvider } from "next-themes";
import { useTelegramColorScheme } from "./telegram-provider";

/**
 * The site's next-themes provider. Inside the Mini App it is forced to
 * Telegram's colour scheme (spec D4) and follows themeChanged live: a forced
 * theme is applied to <html> but never persisted, so the Mini App cannot
 * overwrite the learner's saved site theme in any storage it shares with the
 * website. On the website the scheme is null and nothing changes.
 */
export function TelegramThemeProvider({ children }: { children: React.ReactNode }) {
  const colorScheme = useTelegramColorScheme();
  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="dark"
      enableSystem={false}
      storageKey="theme"
      disableTransitionOnChange
      value={{ light: "light", dark: "dark" }}
      forcedTheme={colorScheme ?? undefined}
    >
      {children}
    </ThemeProvider>
  );
}
