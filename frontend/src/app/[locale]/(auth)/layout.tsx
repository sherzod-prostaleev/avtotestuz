import type { Metadata } from "next";
import { ClientMessages } from "@/i18n/client-messages";
import { AUTH_NAMESPACES } from "@/i18n/namespaces";
import { PageTransition } from "@/components/layout/page-transition";

// Login/register/password-reset: no unique content to rank, and indexing the
// login form itself is a nuisance click for searchers.
export const metadata: Metadata = { robots: { index: false, follow: false } };

export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <ClientMessages namespaces={AUTH_NAMESPACES}>
      <PageTransition>{children}</PageTransition>
    </ClientMessages>
  );
}
