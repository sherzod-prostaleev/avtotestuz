import { redirect } from "next/navigation";

// Landing target for the referral invite links shared via ReferralCard's
// "Copy Link" button (backend-generated as https://avtotest.uz/r/{code},
// see backend/internal/billing/referral.go ReferralStats.InviteURL).
// LoginPage captures ?ref=CODE into localStorage and login/verify applies
// it after a successful OTP verify (see lib/referral-storage.ts).
export default function ReferralInvitePage({
  params,
}: {
  params: { locale: string; code: string };
}) {
  redirect(`/${params.locale}/login?ref=${encodeURIComponent(params.code)}`);
}
