import type { Metadata } from "next";
import { ClientMessages } from "@/i18n/client-messages";
import { APP_NAMESPACES } from "@/i18n/namespaces";
import { SessionOriginTracker } from "@/components/layout/session-origin-tracker";
import { AppShell } from "./app-shell";

// Learner-only screens (dashboard, practice, exam, tickets, …): client-fetched,
// login-gated, nothing here for a crawler to rank on.
export const metadata: Metadata = { robots: { index: false, follow: false } };

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <ClientMessages namespaces={APP_NAMESPACES}>
      {/* Remembers the hub a session was opened from, so its exit button
          returns here instead of the dashboard. */}
      <SessionOriginTracker />
      <AppShell>{children}</AppShell>
    </ClientMessages>
  );
}
