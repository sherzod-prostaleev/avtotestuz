"use client";

import { ThemeProvider } from "next-themes";
import { InitSentry } from "@/components/monitoring/init-sentry";
import { RegisterServiceWorker } from "@/components/pwa/register-sw";

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <ThemeProvider attribute="class" defaultTheme="dark" enableSystem={false}>
      <InitSentry />
      <RegisterServiceWorker />
      {children}
    </ThemeProvider>
  );
}
