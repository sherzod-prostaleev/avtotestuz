import { redirect } from "next/navigation";

// Landing target for the referral invite links shared via ReferralCard's
// "Copy Link" button (backend-generated as {PUBLIC_BASE_URL}/r/{code}, see
// backend/internal/billing/referral.go ReferralStats.InviteURL).
// The code is handed to /login as ?ref=CODE; from there lib/referral-storage.ts
// stashes it and either login/verify (new user) or ReferralCapture (already
// signed in, bounced off /login by the middleware) redeems it.
export default function ReferralInvitePage({
  params,
}: {
  params: { locale: string; code: string };
}) {
  redirect(`/${params.locale}/login?ref=${encodeURIComponent(params.code)}`);
}
