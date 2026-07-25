"use client";

import { Sidebar } from "@/components/layout/sidebar";
import { ReferralCapture } from "@/components/referral/referral-capture";
import { DemoProgressCapture } from "@/components/demo/demo-progress-capture";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    // Mobile: column (top bar above content). Desktop: row with fixed sidebar.
    <div className="flex min-h-screen w-full flex-col bg-background md:flex-row">
      <ReferralCapture />
      <DemoProgressCapture />
      <Sidebar />
      <main className="min-w-0 flex-1 pb-[env(safe-area-inset-bottom)] md:ml-64">
        {children}
      </main>
    </div>
  );
}
