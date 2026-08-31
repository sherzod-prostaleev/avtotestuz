import { ClientMessages } from "@/i18n/client-messages";
import { AUTH_NAMESPACES } from "@/i18n/namespaces";
import { PageTransition } from "@/components/layout/page-transition";

export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <ClientMessages namespaces={AUTH_NAMESPACES}>
      <PageTransition>{children}</PageTransition>
    </ClientMessages>
  );
}
