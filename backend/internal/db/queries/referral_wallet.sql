-- name: GetReferralCommissionPercent :one
SELECT referral_commission_percent FROM profile WHERE id = $1;

-- name: UpdateReferralCommissionPercent :one
UPDATE profile
SET referral_commission_percent = $2
WHERE id = $1
RETURNING id, referral_commission_percent;

-- name: InsertReferralLedger :one
INSERT INTO referral_ledger (profile_id, entry_type, amount_uzs, payment_id, payout_id, meta)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, profile_id, entry_type, amount_uzs, payment_id, payout_id, meta, created_at;

-- name: GetReferralBalance :one
SELECT COALESCE(SUM(amount_uzs), 0)::bigint AS balance_uzs
FROM referral_ledger
WHERE profile_id = $1;

-- name: GetReferralEarnedCommission :one
SELECT COALESCE(SUM(amount_uzs), 0)::bigint AS earned_uzs
FROM referral_ledger
WHERE profile_id = $1 AND entry_type = 'commission';

-- name: ListReferralLedgerForUser :many
SELECT id, profile_id, entry_type, amount_uzs, payment_id, payout_id, meta, created_at
FROM referral_ledger
WHERE profile_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: GetCommissionByPaymentID :one
SELECT id, profile_id, entry_type, amount_uzs, payment_id, payout_id, meta, created_at
FROM referral_ledger
WHERE payment_id = $1 AND entry_type = 'commission';

-- name: GetClawbackByPaymentID :one
SELECT id, profile_id, entry_type, amount_uzs, payment_id, payout_id, meta, created_at
FROM referral_ledger
WHERE payment_id = $1 AND entry_type = 'clawback';

-- name: CreateReferralPayout :one
INSERT INTO referral_payout (profile_id, amount_uzs, card_number, card_network)
VALUES ($1, $2, $3, $4)
RETURNING id, profile_id, amount_uzs, card_number, card_network, status, admin_note, processed_by, created_at, processed_at;

-- name: GetReferralPayoutForUpdate :one
SELECT id, profile_id, amount_uzs, card_number, card_network, status, admin_note, processed_by, created_at, processed_at
FROM referral_payout
WHERE id = $1
FOR UPDATE;

-- name: ListReferralPayouts :many
SELECT p.id, p.profile_id, p.amount_uzs, p.card_number, p.card_network, p.status,
       p.admin_note, p.processed_by, p.created_at, p.processed_at,
       pr.phone AS profile_phone, pr.name AS profile_name
FROM referral_payout p
JOIN profile pr ON pr.id = p.profile_id
WHERE ($1::text = '' OR p.status = $1)
ORDER BY p.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountReferralPayouts :one
SELECT COUNT(*)::bigint
FROM referral_payout
WHERE ($1::text = '' OR status = $1);

-- name: MarkReferralPayoutPaid :one
UPDATE referral_payout
SET status = 'paid',
    processed_by = sqlc.arg(processed_by),
    processed_at = NOW(),
    admin_note = COALESCE(NULLIF(sqlc.arg(admin_note)::text, ''), admin_note)
WHERE id = sqlc.arg(id) AND status = 'pending'
RETURNING id, profile_id, amount_uzs, card_number, card_network, status, admin_note, processed_by, created_at, processed_at;

-- name: MarkReferralPayoutRejected :one
UPDATE referral_payout
SET status = 'rejected',
    processed_by = sqlc.arg(processed_by),
    processed_at = NOW(),
    admin_note = COALESCE(NULLIF(sqlc.arg(admin_note)::text, ''), admin_note)
WHERE id = sqlc.arg(id) AND status = 'pending'
RETURNING id, profile_id, amount_uzs, card_number, card_network, status, admin_note, processed_by, created_at, processed_at;

-- name: ListRefereesForReferrer :many
SELECT r.id, r.referee_id, r.referral_code, r.status, r.created_at, r.rewarded_at,
       p.phone AS referee_phone, p.name AS referee_name,
       COALESCE((
         SELECT SUM(l.amount_uzs)
         FROM referral_ledger l
         WHERE l.profile_id = r.referrer_id
           AND l.entry_type = 'commission'
           AND (l.meta->>'referee_id') = r.referee_id::text
       ), 0)::bigint AS commission_uzs
FROM referral r
JOIN profile p ON p.id = r.referee_id
WHERE r.referrer_id = $1
ORDER BY r.created_at DESC
LIMIT $2;

-- name: GetAdminReferralUserStats :one
SELECT
  (SELECT COUNT(*)::bigint FROM referral r WHERE r.referrer_id = pr.id) AS total_invited,
  (SELECT COUNT(*)::bigint FROM referral r WHERE r.referrer_id = pr.id AND r.status = 'rewarded') AS total_rewarded,
  (SELECT COALESCE(SUM(l.amount_uzs), 0)::bigint FROM referral_ledger l WHERE l.profile_id = pr.id AND l.entry_type = 'commission') AS earned_uzs,
  (SELECT COALESCE(SUM(l.amount_uzs), 0)::bigint FROM referral_ledger l WHERE l.profile_id = pr.id) AS balance_uzs,
  pr.referral_commission_percent AS commission_percent
FROM profile pr
WHERE pr.id = $1;

-- name: ListReferralPayoutsForProfile :many
SELECT id, amount_uzs, card_number, card_network, status, admin_note, created_at, processed_at
FROM referral_payout
WHERE profile_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: GetReferralPayoutSummaryForProfile :one
SELECT
  COUNT(*) FILTER (WHERE status = 'pending')::bigint AS pending_count,
  COALESCE(SUM(amount_uzs) FILTER (WHERE status = 'pending'), 0)::bigint AS pending_uzs,
  COUNT(*) FILTER (WHERE status = 'paid')::bigint AS paid_count,
  COALESCE(SUM(amount_uzs) FILTER (WHERE status = 'paid'), 0)::bigint AS paid_uzs,
  COUNT(*) FILTER (WHERE status = 'rejected')::bigint AS rejected_count,
  COALESCE(SUM(amount_uzs) FILTER (WHERE status = 'rejected'), 0)::bigint AS rejected_uzs
FROM referral_payout
WHERE profile_id = $1;

-- name: ListReferralEarningsDetail :many
-- Honest transparency: each commission with payment/tariff/referee context.
SELECT
  l.id AS ledger_id,
  l.amount_uzs AS commission_uzs,
  l.created_at AS rewarded_at,
  l.payment_id,
  COALESCE(pay.amount_uzs, 0)::bigint AS payment_amount_uzs,
  COALESCE(pay.tariff_days_snapshot, 0)::int AS tariff_days,
  COALESCE(t.code, '') AS tariff_code,
  COALESCE(ref.name, '') AS referee_name,
  COALESCE(ref.phone, '') AS referee_phone,
  COALESCE((l.meta->>'percent')::int, 0)::int AS percent_snapshot
FROM referral_ledger l
LEFT JOIN payment pay ON pay.id = l.payment_id
LEFT JOIN tariff t ON t.id = pay.tariff_id
LEFT JOIN profile ref ON ref.id = NULLIF(l.meta->>'referee_id', '')::uuid
WHERE l.profile_id = $1 AND l.entry_type = 'commission'
ORDER BY l.created_at DESC
LIMIT $2;
