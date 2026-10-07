"use client";

import { QueryClientProvider } from "@tanstack/react-query";
import { InitSentry } from "@/components/monitoring/init-sentry";
import { TelegramProvider } from "@/components/telegram/telegram-provider";
import { TelegramThemeProvider } from "@/components/telegram/telegram-theme-provider";
import { RegisterServiceWorker } from "@/components/pwa/register-sw";
import { getQueryClient } from "@/lib/query-client";

export function Providers({ children }: { children: React.ReactNode }) {
  const queryClient = getQueryClient();

  return (
    <QueryClientProvider client={queryClient}>
      {/* Telegram outside the theme: inside the Mini App the theme is forced
          from Telegram's colour scheme, so the provider must know it first. */}
      <TelegramProvider>
        <TelegramThemeProvider>
          <InitSentry />
          <RegisterServiceWorker />
          {children}
        </TelegramThemeProvider>
      </TelegramProvider>
    </QueryClientProvider>
  );
}
