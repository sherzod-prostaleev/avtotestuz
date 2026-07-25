"use client";

import { ThemeProvider } from "next-themes";
import { RegisterServiceWorker } from "@/components/pwa/register-sw";

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <ThemeProvider attribute="class" defaultTheme="dark" enableSystem={false}>
      <RegisterServiceWorker />
      {children}
    </ThemeProvider>
  );
}
