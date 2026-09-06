import type { Metadata } from "next";
import { ClientMessages } from "@/i18n/client-messages";
import { ADMIN_NAMESPACES } from "@/i18n/namespaces";

// Internal admin panel — never a search result.
export const metadata: Metadata = { robots: { index: false, follow: false } };

export default function AdminRootLayout({ children }: { children: React.ReactNode }) {
  return <ClientMessages namespaces={ADMIN_NAMESPACES}>{children}</ClientMessages>;
}
