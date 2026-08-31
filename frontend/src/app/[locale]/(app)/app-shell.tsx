"use client";

import { Sidebar } from "@/components/layout/sidebar";
import { ReferralCapture } from "@/components/referral/referral-capture";
import { DemoProgressCapture } from "@/components/demo/demo-progress-capture";
import { SupportBanner } from "@/components/support/support-banner";
import { MaintenanceBanner } from "@/components/support/maintenance-banner";
import { MustChangePasswordGate } from "@/components/auth/must-change-password-gate";
import { PageTransition } from "@/components/layout/page-transition";

export function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <MustChangePasswordGate>
      {/* Mobile: column (top bar above content). Desktop: row with fixed sidebar. */}
      <div className="flex min-h-screen w-full flex-col bg-background md:flex-row">
        <ReferralCapture />
        <DemoProgressCapture />
        <Sidebar />
        <div className="app-shell-main relative min-w-0 flex-1 pb-[calc(4.25rem+env(safe-area-inset-bottom))] md:ml-64 md:pb-0">
          <MaintenanceBanner />
          <SupportBanner />
          <PageTransition>{children}</PageTransition>
        </div>

      </div>
    </MustChangePasswordGate>
  );
}

