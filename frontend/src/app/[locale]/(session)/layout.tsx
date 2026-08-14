import { ClientMessages } from "@/i18n/client-messages";
import { SESSION_NAMESPACES } from "@/i18n/namespaces";

export default function SessionLayout({ children }: { children: React.ReactNode }) {
  return <ClientMessages namespaces={SESSION_NAMESPACES}>{children}</ClientMessages>;
}
