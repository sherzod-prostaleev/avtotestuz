// Deliberately minimal. The classroom kiosk (/station) is a login-free page
// opened on shared school PCs and must not inherit (app)/layout.tsx's
// Sidebar, ReferralCapture or DemoProgressCapture — those all lead back to
// account, purchase or referral flows that have no place on a kiosk. Locale
// providers, theme wiring and global styles already come from the parent
// [locale]/layout.tsx, which this route group still sits under.
export default function KioskLayout({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}
