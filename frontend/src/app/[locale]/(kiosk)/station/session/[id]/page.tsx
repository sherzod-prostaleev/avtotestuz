// Kiosk session/exam screen: /[locale]/station/session/[id].
//
// This is where /station/session/start replaces to once a session is
// created. It is a distinct URL from the learner app's /[locale]/session/[id]
// specifically so the middleware exemption in src/proxy.ts stays narrow
// (only "station" and everything under it is login-free) instead of
// unauthenticating /session/[id] for every learner on the platform. It
// reuses the same page component as the learner app in kiosk mode, which
// routes every exit (exam close button, finish-screen buttons, exam-pass
// dashboard button, locale switcher) under /station/... instead of the
// login-gated learner routes — see TestSessionPageProps in the imported
// module.
import TestSessionPage from "@/app/[locale]/(session)/session/[id]/page";

export default function KioskSessionPage() {
  return <TestSessionPage kiosk />;
}
