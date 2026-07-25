"use client";

import { Sidebar } from "@/components/layout/sidebar";
import { ReferralCapture } from "@/components/referral/referral-capture";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen w-full bg-background">
      <ReferralCapture />
      <Sidebar />
      <main className="min-w-0 flex-1 pb-[env(safe-area-inset-bottom)] md:ml-64">
        {children}
      </main>
    </div>
  );
}
