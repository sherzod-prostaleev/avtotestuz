"use client";

import { QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "next-themes";
import { InitSentry } from "@/components/monitoring/init-sentry";
import { RegisterServiceWorker } from "@/components/pwa/register-sw";
import { getQueryClient } from "@/lib/query-client";

export function Providers({ children }: { children: React.ReactNode }) {
  const queryClient = getQueryClient();

  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider
        attribute="class"
        defaultTheme="dark"
        enableSystem={false}
        storageKey="theme"
        disableTransitionOnChange
        value={{ light: "light", dark: "dark" }}
      >
        <InitSentry />
        <RegisterServiceWorker />
        {children}
      </ThemeProvider>
    </QueryClientProvider>
  );
}
