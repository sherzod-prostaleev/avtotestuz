"use client";

import { Sidebar } from "@/components/layout/sidebar";
import { ReferralCapture } from "@/components/referral/referral-capture";
import { DemoProgressCapture } from "@/components/demo/demo-progress-capture";
import { SupportBanner } from "@/components/support/support-banner";
import { MaintenanceBanner } from "@/components/support/maintenance-banner";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    // Mobile: column (top bar above content). Desktop: row with fixed sidebar.
    <div className="flex min-h-screen w-full flex-col bg-background md:flex-row">
      <ReferralCapture />
      <DemoProgressCapture />
      <Sidebar />
      <main className="min-w-0 flex-1 pb-[calc(4.25rem+env(safe-area-inset-bottom))] md:ml-64 md:pb-0">
        <MaintenanceBanner />
        <SupportBanner />
        {children}
      </main>
    </div>
  );
}
