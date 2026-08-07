// Kiosk stats entry point: /[locale]/station/stats.
//
// A distinct URL from the learner app's /[locale]/stats so the middleware
// exemption in src/proxy.ts stays narrow — only "station" and everything
// under it is login-free.
//
// The numbers here belong to the PC, not to a student: a station has one
// shadow profile that every student who sits down shares. That is a known
// and accepted consequence of a login-free classroom, not a bug.
import StatsPage from "@/app/[locale]/(app)/stats/page";

export default function KioskStatsPage() {
  return <StatsPage kiosk />;
}
