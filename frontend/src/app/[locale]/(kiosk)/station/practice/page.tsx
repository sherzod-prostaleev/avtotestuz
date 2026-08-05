// Kiosk practice entry point: /[locale]/station/practice.
//
// This is a distinct URL from the learner app's /[locale]/practice
// specifically so the middleware exemption in src/proxy.ts stays narrow
// (only "station" and everything under it is login-free) instead of
// unauthenticating /practice for every learner on the platform. It reuses
// the same page component as the learner app in kiosk mode, which hides the
// dashboard back-link and VIP checkout entry points — see
// PracticePageProps in the imported module.
import PracticePage from "@/app/[locale]/(app)/practice/page";

export default function KioskPracticePage() {
  return <PracticePage kiosk />;
}
