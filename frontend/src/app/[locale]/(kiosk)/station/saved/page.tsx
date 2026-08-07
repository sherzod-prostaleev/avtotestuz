// Kiosk saved-questions entry point: /[locale]/station/saved.
//
// A distinct URL from the learner app's /[locale]/saved so the middleware
// exemption in src/proxy.ts stays narrow — only "station" and everything
// under it is login-free.
//
// The list belongs to the PC, not to a student: a station has one shadow
// profile that every student who sits down shares. Known and accepted.
import SavedPage from "@/app/[locale]/(app)/saved/page";

export default function KioskSavedPage() {
  return <SavedPage kiosk />;
}
