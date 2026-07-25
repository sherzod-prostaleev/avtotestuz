"use client";

import { Sidebar } from "@/components/layout/sidebar";
import { ReferralCapture } from "@/components/referral/referral-capture";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen w-full">
      <ReferralCapture />
      <Sidebar />
      <main className="flex-1 md:ml-64 min-w-0">{children}</main>
    </div>
  );
}
