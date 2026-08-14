import { ClientMessages } from "@/i18n/client-messages";
import { PUBLIC_NAMESPACES } from "@/i18n/namespaces";

export default function PublicLayout({ children }: { children: React.ReactNode }) {
  return <ClientMessages namespaces={PUBLIC_NAMESPACES}>{children}</ClientMessages>;
}
