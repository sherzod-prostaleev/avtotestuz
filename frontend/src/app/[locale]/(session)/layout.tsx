import type { Metadata } from "next";
import { ClientMessages } from "@/i18n/client-messages";
import { SESSION_NAMESPACES } from "@/i18n/namespaces";
import { SessionExpiredGate } from "@/components/auth/session-expired-gate";

// Live exam-session runtime, per-user state only.
export const metadata: Metadata = { robots: { index: false, follow: false } };

export default function SessionLayout({ children }: { children: React.ReactNode }) {
  return (
    <ClientMessages namespaces={SESSION_NAMESPACES}>
      {/* The runner is the one screen with no shell around it, so an expired
          session used to leave a half-loaded exam and no way out. The kiosk's
          own runner lives under (kiosk)/station/session and stays login-free. */}
      <SessionExpiredGate />
      {children}
    </ClientMessages>
  );
}
