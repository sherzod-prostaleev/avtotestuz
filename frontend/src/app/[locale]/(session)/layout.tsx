import type { Metadata } from "next";
import { ClientMessages } from "@/i18n/client-messages";
import { SESSION_NAMESPACES } from "@/i18n/namespaces";

// Live exam-session runtime, per-user state only.
export const metadata: Metadata = { robots: { index: false, follow: false } };

export default function SessionLayout({ children }: { children: React.ReactNode }) {
  return <ClientMessages namespaces={SESSION_NAMESPACES}>{children}</ClientMessages>;
}
