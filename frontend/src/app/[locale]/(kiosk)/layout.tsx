import type { Metadata } from "next";
import { ClientMessages } from "@/i18n/client-messages";
import { KIOSK_NAMESPACES } from "@/i18n/namespaces";
import { KioskChrome } from "./kiosk-chrome";

// Walk-up classroom PCs only, no login screen or unique copy to rank on.
export const metadata: Metadata = { robots: { index: false, follow: false } };

// Deliberately no learner Sidebar / referral / purchase chrome — see kiosk-chrome.
export default function KioskLayout({ children }: { children: React.ReactNode }) {
  return (
    <ClientMessages namespaces={KIOSK_NAMESPACES}>
      {/* Language and light/dark belong to the student at the PC, not to the
          school's admin at download time. Mounted here so all seven sections
          get them without each reused (app) page growing kiosk-only chrome. */}
      <KioskChrome />
      {children}
    </ClientMessages>
  );
}
