// Deliberately minimal. The classroom kiosk (/station) is a login-free page
// opened on shared school PCs and must not inherit (app)/layout.tsx's
// Sidebar, ReferralCapture or DemoProgressCapture — those all lead back to
// account, purchase or referral flows that have no place on a kiosk. Locale
// providers, theme wiring and global styles already come from the parent
// [locale]/layout.tsx, which this route group still sits under.
import { KioskChrome } from "./kiosk-chrome";

export default function KioskLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      {/* Language and light/dark belong to the student at the PC, not to the
          school's admin at download time. Mounted here so all seven sections
          get them without each reused (app) page growing kiosk-only chrome. */}
      <KioskChrome />
      {children}
    </>
  );
}
