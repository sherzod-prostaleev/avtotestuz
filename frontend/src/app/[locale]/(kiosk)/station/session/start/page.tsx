// Kiosk session-start entry point: /[locale]/station/session/start.
//
// practice/tickets push a walk-up student here to actually create a session.
// This is a distinct URL from the learner app's /[locale]/session/start
// specifically so the middleware exemption in src/proxy.ts stays narrow
// (only "station" and everything under it is login-free) instead of
// unauthenticating /session/start for every learner on the platform. It
// reuses the same page component as the learner app in kiosk mode, which
// redirects every success/error exit under /station/... instead of the
// login-gated learner routes — see SessionStartPageProps in the imported
// module.
import SessionStartPage from "@/app/[locale]/(session)/session/start/page";

export default function KioskSessionStartPage() {
  return <SessionStartPage kiosk />;
}
