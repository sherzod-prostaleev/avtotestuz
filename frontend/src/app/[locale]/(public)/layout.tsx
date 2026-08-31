import { ClientMessages } from "@/i18n/client-messages";
import { PUBLIC_NAMESPACES } from "@/i18n/namespaces";
import { PageTransition } from "@/components/layout/page-transition";

export default function PublicLayout({ children }: { children: React.ReactNode }) {
  return (
    <ClientMessages namespaces={PUBLIC_NAMESPACES}>
      <PageTransition>{children}</PageTransition>
    </ClientMessages>
  );
}
