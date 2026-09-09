// Kiosk memorize entry point: /[locale]/station/practice/memorize/[code].
//
// Reuses the learner app's memorize page in kiosk mode: exit and the
// vip_required fallback push to /station/... instead of the login-gated
// learner routes. See MemorizePageProps in the imported module, and
// billing.StationVIPChecker for why a licensed station's Billing.Status
// already comes back active without any kiosk-specific code on the server.
import MemorizePage from "@/app/[locale]/(app)/practice/memorize/[code]/page";

export default function KioskMemorizePage() {
  return <MemorizePage kiosk />;
}
