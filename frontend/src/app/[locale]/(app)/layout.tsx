import type { Metadata } from "next";
import { ClientMessages } from "@/i18n/client-messages";
import { APP_NAMESPACES } from "@/i18n/namespaces";
import { AppShell } from "./app-shell";

// Learner-only screens (dashboard, practice, exam, tickets, …): client-fetched,
// login-gated, nothing here for a crawler to rank on.
export const metadata: Metadata = { robots: { index: false, follow: false } };

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <ClientMessages namespaces={APP_NAMESPACES}>
      <AppShell>{children}</AppShell>
    </ClientMessages>
  );
}
