// Kiosk road-signs entry point: /[locale]/station/signs.
//
// A distinct URL from the learner app's /[locale]/signs so the middleware
// exemption in src/proxy.ts stays narrow — only "station" and everything
// under it is login-free — instead of unauthenticating /signs for every
// learner on the platform.
import SignsPage from "@/app/[locale]/(app)/signs/page";

export default function KioskSignsPage() {
  return <SignsPage kiosk />;
}
