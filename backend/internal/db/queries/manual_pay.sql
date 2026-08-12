-- name: ListManualPayCards :many
SELECT id, network, pan_full, pan_last4, holder_name, sort_order, enabled, created_at, updated_at
FROM manual_pay_card
ORDER BY sort_order, created_at;

-- name: ListEnabledManualPayCards :many
SELECT id, network, pan_full, pan_last4, holder_name, sort_order, enabled, created_at, updated_at
FROM manual_pay_card
WHERE enabled = true
ORDER BY sort_order, created_at;

-- name: GetManualPayCard :one
SELECT id, network, pan_full, pan_last4, holder_name, sort_order, enabled, created_at, updated_at
FROM manual_pay_card
WHERE id = $1;

-- name: InsertManualPayCard :one
INSERT INTO manual_pay_card (network, pan_full, pan_last4, holder_name, sort_order, enabled)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, network, pan_full, pan_last4, holder_name, sort_order, enabled, created_at, updated_at;

-- name: UpdateManualPayCard :one
UPDATE manual_pay_card
SET pan_full = $2,
    pan_last4 = $3,
    holder_name = $4,
    sort_order = $5,
    enabled = $6,
    updated_at = now()
WHERE id = $1
RETURNING id, network, pan_full, pan_last4, holder_name, sort_order, enabled, created_at, updated_at;

-- name: DeleteManualPayCard :exec
DELETE FROM manual_pay_card WHERE id = $1;

-- name: ReleaseExpiredManualHolds :exec
-- Path B: free the card after hold_until; keep assignment open for late push / admin.
UPDATE manual_pay_assignment
SET released_at = now(),
    manual_state = CASE
      WHEN manual_state IN ('awaiting_transfer', 'claimed') THEN 'awaiting_review'
      ELSE manual_state
    END
WHERE released_at IS NULL
  AND hold_until <= now()
  AND manual_state IN ('awaiting_transfer', 'claimed');

-- name: LockFreeManualPayCard :one
-- Pick an enabled card with no active (unreleased) hold. Caller must run
-- ReleaseExpiredManualHolds first in the same transaction.
SELECT c.id, c.network, c.pan_full, c.pan_last4, c.holder_name, c.sort_order, c.enabled, c.created_at, c.updated_at
FROM manual_pay_card c
WHERE c.enabled = true
  AND NOT EXISTS (
    SELECT 1 FROM manual_pay_assignment a
    WHERE a.card_id = c.id AND a.released_at IS NULL
  )
ORDER BY c.sort_order, c.created_at
FOR UPDATE OF c SKIP LOCKED
LIMIT 1;

-- name: InsertManualPayAssignment :one
INSERT INTO manual_pay_assignment (
  payment_id, card_id, amount_uzs, assigned_at, hold_until, manual_state
) VALUES ($1, $2, $3, $4, $5, 'awaiting_transfer')
RETURNING id, payment_id, card_id, amount_uzs, assigned_at, hold_until, released_at,
          manual_state, claimed_at, confirmed_by, confirmed_at, admin_note, created_at;

-- name: GetManualPayAssignmentByPayment :one
SELECT a.id, a.payment_id, a.card_id, a.amount_uzs, a.assigned_at, a.hold_until, a.released_at,
       a.manual_state, a.claimed_at, a.confirmed_by, a.confirmed_at, a.admin_note, a.created_at,
       c.pan_full, c.pan_last4, c.holder_name, c.network
FROM manual_pay_assignment a
JOIN manual_pay_card c ON c.id = a.card_id
WHERE a.payment_id = $1;

-- name: GetManualPayAssignmentByPaymentForUpdate :one
SELECT a.id, a.payment_id, a.card_id, a.amount_uzs, a.assigned_at, a.hold_until, a.released_at,
       a.manual_state, a.claimed_at, a.confirmed_by, a.confirmed_at, a.admin_note, a.created_at
FROM manual_pay_assignment a
WHERE a.payment_id = $1
FOR UPDATE;

-- name: ClaimManualPayAssignment :exec
UPDATE manual_pay_assignment
SET manual_state = CASE
      WHEN manual_state = 'awaiting_transfer' THEN 'claimed'
      ELSE manual_state
    END,
    claimed_at = COALESCE(claimed_at, now())
WHERE payment_id = $1
  AND manual_state IN ('awaiting_transfer', 'claimed', 'awaiting_review');

-- name: MarkManualPayAssignmentConsumed :exec
UPDATE manual_pay_assignment
SET manual_state = 'consumed',
    confirmed_by = $2,
    confirmed_at = now(),
    released_at = COALESCE(released_at, now())
WHERE payment_id = $1;

-- name: RejectManualPayAssignment :exec
UPDATE manual_pay_assignment
SET manual_state = 'rejected',
    confirmed_by = 'admin',
    confirmed_at = now(),
    released_at = COALESCE(released_at, now()),
    admin_note = $2
WHERE payment_id = $1
  AND manual_state IN ('awaiting_transfer', 'claimed', 'awaiting_review');

-- name: ListOpenManualAmountsForCard :many
-- Amounts currently claimable on one card. StartManualCheckout picks an
-- amount not in this set so every open assignment on a card is uniquely
-- identifiable by the transferred sum. The predicate must stay identical to
-- FindManualPayMatchCandidate's, or a "free" amount could collide with an
-- assignment that is in fact still matchable.
SELECT a.amount_uzs
FROM manual_pay_assignment a
JOIN payment p ON p.id = a.payment_id
WHERE a.card_id = $1
  AND a.manual_state IN ('awaiting_transfer', 'claimed', 'awaiting_review')
  AND p.status IN ('created', 'pending')
  AND p.created_at >= now() - interval '4 hours';

-- name: FindManualPayMatchCandidate :one
-- Match an open assignment by (last4, exact amount) inside a bounded window.
--
-- Amounts are unique per card among open assignments (see
-- ListOpenManualAmountsForCard), so at most one row can match and the old
-- "newest assignment on the card wins" tie-break is gone. That tie-break was
-- exploitable: with only 4 cards x 3 tariff prices, an attacker could loop
-- checkouts to always own the newest assignment on a card and have a genuine
-- late payer's transfer confirm HIS payment instead.
--
-- The window is bounded at both ends. Lower: the bank push carries only
-- HH:MM, so transfer_at has its seconds zeroed and can legitimately read up
-- to 59s before assigned_at — without the tolerance, anyone paying in the
-- same minute they checked out failed to auto-confirm. Upper: unpaid Humo
-- checkouts auto-cancel after 4 hours (see ExpireStaleManualPayments); the
-- same bound stops an abandoned assignment from capturing a later credit.
SELECT a.payment_id, a.id AS assignment_id, a.assigned_at, a.amount_uzs, a.manual_state
FROM manual_pay_assignment a
JOIN manual_pay_card c ON c.id = a.card_id
JOIN payment p ON p.id = a.payment_id
WHERE c.pan_last4 = $1
  AND a.amount_uzs = $2
  AND a.manual_state IN ('awaiting_transfer', 'claimed', 'awaiting_review')
  AND p.status IN ('created', 'pending')
  AND p.created_at >= now() - interval '4 hours'
  AND sqlc.arg(transfer_at)::timestamptz
      BETWEEN a.assigned_at - interval '90 seconds'
          AND a.assigned_at + interval '4 hours'
ORDER BY a.assigned_at DESC
LIMIT 1;

-- name: ListOpenManualPayAssignments :many
SELECT a.id, a.payment_id, a.card_id, a.amount_uzs, a.assigned_at, a.hold_until, a.released_at,
       a.manual_state, a.claimed_at, a.confirmed_by, a.confirmed_at, a.admin_note, a.created_at,
       c.pan_last4, c.holder_name, c.network,
       p.profile_id, p.status AS payment_status, p.provider,
       pr.phone AS profile_phone
FROM manual_pay_assignment a
JOIN manual_pay_card c ON c.id = a.card_id
JOIN payment p ON p.id = a.payment_id
JOIN profile pr ON pr.id = p.profile_id
WHERE a.manual_state IN ('awaiting_transfer', 'claimed', 'awaiting_review')
ORDER BY a.assigned_at DESC
LIMIT $1;

-- name: InsertManualPayEvent :one
INSERT INTO manual_pay_event (
  fingerprint, telegram_msg_id, raw_text, amount_uzs, pan_last4, transfer_at, balance_uzs, status, matched_payment_id, parse_ok
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, fingerprint, telegram_msg_id, raw_text, amount_uzs, pan_last4, transfer_at, balance_uzs,
          status, matched_payment_id, parse_ok, created_at;

-- name: GetManualPayEventByFingerprint :one
SELECT id, fingerprint, telegram_msg_id, raw_text, amount_uzs, pan_last4, transfer_at, balance_uzs,
       status, matched_payment_id, parse_ok, created_at
FROM manual_pay_event
WHERE fingerprint = $1;

-- name: MarkManualPayEventMatched :exec
UPDATE manual_pay_event
SET status = 'matched', matched_payment_id = $2
WHERE id = $1;

-- name: MarkManualPayEventIgnored :exec
UPDATE manual_pay_event
SET status = 'ignored'
WHERE id = $1;

-- name: ListUnmatchedManualPayEvents :many
SELECT id, fingerprint, telegram_msg_id, raw_text, amount_uzs, pan_last4, transfer_at, balance_uzs,
       status, matched_payment_id, parse_ok, created_at
FROM manual_pay_event
WHERE status = 'unmatched'
ORDER BY created_at DESC
LIMIT $1;

-- name: GetManualTgSettings :one
SELECT id, api_id_enc, api_hash_enc, session_enc, humo_bot_username,
       last_test_ok, last_test_at, last_test_detail, updated_at
FROM manual_tg_settings
WHERE id = 1;

-- name: UpsertManualTgSettings :one
UPDATE manual_tg_settings
SET api_id_enc = CASE WHEN @set_api_id::bool THEN @api_id_enc ELSE api_id_enc END,
    api_hash_enc = CASE WHEN @set_api_hash::bool THEN @api_hash_enc ELSE api_hash_enc END,
    session_enc = CASE WHEN @set_session::bool THEN @session_enc ELSE session_enc END,
    humo_bot_username = CASE WHEN @set_bot_username::bool THEN @humo_bot_username ELSE humo_bot_username END,
    updated_at = now()
WHERE id = 1
RETURNING id, api_id_enc, api_hash_enc, session_enc, humo_bot_username,
          last_test_ok, last_test_at, last_test_detail, updated_at;

-- name: SetManualTgTestResult :exec
UPDATE manual_tg_settings
SET last_test_ok = $1,
    last_test_at = now(),
    last_test_detail = $2,
    updated_at = now()
WHERE id = 1;

-- name: SetPaymentPending :exec
UPDATE payment SET status = 'pending' WHERE id = $1 AND status = 'created';

-- name: SetPaymentCanceled :exec
UPDATE payment SET status = 'canceled' WHERE id = $1 AND status IN ('created', 'pending');

-- name: ExpireStaleManualPayments :many
-- Keep the row: abandoned Humo/manual checkouts become canceled, never deleted.
-- Data-modifying CTEs always run to completion in PostgreSQL.
WITH expired AS (
  UPDATE payment p
  SET status = 'canceled'
  WHERE p.provider = 'manual'
    AND p.status IN ('created', 'pending')
    AND p.paid_at IS NULL
    AND p.created_at < now() - interval '4 hours'
  RETURNING p.id
), _closed AS (
  UPDATE manual_pay_assignment a
  SET manual_state = 'rejected',
      confirmed_by = 'system',
      confirmed_at = now(),
      released_at = COALESCE(released_at, now()),
      admin_note = COALESCE(NULLIF(btrim(a.admin_note), ''), 'auto-expired: unpaid after 4 hours')
  WHERE a.payment_id IN (SELECT id FROM expired)
    AND a.manual_state IN ('awaiting_transfer', 'claimed', 'awaiting_review')
)
SELECT id FROM expired;

-- name: MarkPaymentPaidIfOpen :execrows
UPDATE payment
SET status = 'paid', paid_at = now()
WHERE id = $1 AND status IN ('created', 'pending');
