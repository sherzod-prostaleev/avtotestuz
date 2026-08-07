// Kiosk leaderboard entry point: /[locale]/station/leaderboard.
//
// A distinct URL from the learner app's /[locale]/leaderboard so the
// middleware exemption in src/proxy.ts stays narrow — only "station" and
// everything under it is login-free.
//
// Read-only by nature: stations are excluded from the rankings on purpose
// (leaderboard.Service.RecordPoint returns early for a kind='station'
// profile), so a classroom PC can see the board but never joins it.
import LeaderboardPage from "@/app/[locale]/(app)/leaderboard/page";

export default function KioskLeaderboardPage() {
  return <LeaderboardPage kiosk />;
}
