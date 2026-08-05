// Kiosk tickets entry point: /[locale]/station/tickets.
//
// This is a distinct URL from the learner app's /[locale]/tickets
// specifically so the middleware exemption in src/proxy.ts stays narrow
// (only "station" and everything under it is login-free) instead of
// unauthenticating /tickets for every learner on the platform. It reuses
// the same page component as the learner app in kiosk mode, which hides the
// dashboard back-link and VIP checkout entry points — see
// TicketsPageProps in the imported module.
import TicketsPage from "@/app/[locale]/(app)/tickets/page";

export default function KioskTicketsPage() {
  return <TicketsPage kiosk />;
}
