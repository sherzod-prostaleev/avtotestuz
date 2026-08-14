import { ClientMessages } from "@/i18n/client-messages";
import { APP_NAMESPACES } from "@/i18n/namespaces";
import { AppShell } from "./app-shell";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <ClientMessages namespaces={APP_NAMESPACES}>
      <AppShell>{children}</AppShell>
    </ClientMessages>
  );
}
