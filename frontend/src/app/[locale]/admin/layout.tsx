import { ClientMessages } from "@/i18n/client-messages";
import { ADMIN_NAMESPACES } from "@/i18n/namespaces";

export default function AdminRootLayout({ children }: { children: React.ReactNode }) {
  return <ClientMessages namespaces={ADMIN_NAMESPACES}>{children}</ClientMessages>;
}
