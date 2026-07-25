-- name: ListActiveTariffs :many
-- Localized tariff list with uz-Latn fallback (mirrors ListCategories).
SELECT t.code, t.days, t.price_uzs, t.old_price_uzs, t.badge,
       COALESCE(tr.name, ftr.name, t.code) AS name,
       COALESCE(tr.description, ftr.description, '') AS description
FROM tariff t
LEFT JOIN tariff_translation tr ON tr.tariff_id = t.id AND tr.locale = $1
LEFT JOIN tariff_translation ftr ON ftr.tariff_id = t.id AND ftr.locale = 'uz-Latn'
WHERE t.active = true
ORDER BY t.sort_order, t.code;

-- name: GetActiveTariffByCode :one
SELECT id, days, price_uzs FROM tariff WHERE code = $1 AND active = true;

-- name: CreatePayment :one
-- tariff_days_snapshot / tariff_price_uzs_snapshot freeze what the customer is
-- being sold; see migration 0019 for why the live tariff row must not be
-- consulted again after this point.
INSERT INTO payment (profile_id, tariff_id, amount_uzs, provider, status, idempotency_key, promo_code_id,
                     tariff_days_snapshot, tariff_price_uzs_snapshot)
VALUES ($1, $2, $3, $4, 'created', $5, $6, $7, $8)
RETURNING id;

-- name: GetPaymentForPayme :one
-- Both tariff figures come from the payment's own snapshot, NOT from a join on
-- tariff: a 'created' payment never expires, so re-reading a mutable tariff at
-- completion time would let a later price or term edit change what an
-- in-flight purchase is worth (migration 0019). tariff_price_uzs is the
-- undiscounted list price at checkout, needed alongside the possibly
-- promo-discounted amount_uzs so billing.ProcessPaymentGrant can pro-rate the
-- grant when a promo is no longer redeemable at completion (see proRatedDays).
SELECT p.id, p.profile_id, p.tariff_id, p.amount_uzs, p.status, p.promo_code_id,
       p.tariff_days_snapshot AS tariff_days,
       p.tariff_price_uzs_snapshot AS tariff_price_uzs
FROM payment p
WHERE p.id = $1;

-- name: LockProfileForGrant :one
-- Serializes entitlement grants for one profile. billing.GrantDays is a
-- read-modify-write (ActiveEntitlementEnd, then InsertEntitlement stacking on
-- top of it); under READ COMMITTED two concurrent grants both read the same
-- pre-existing end and both write the same interval, so the second grant's
-- days are silently lost. There is no entitlement row to lock (the conflict is
-- over a row that does not exist yet), so the profile row stands in as the
-- serialization point. Only effective when the caller's Queries is bound to a
-- transaction — see GrantDays' doc comment.
SELECT id FROM profile WHERE id = $1 FOR UPDATE;

-- name: GetPromoCodeByCode :one
SELECT id, code, kind, value, max_uses, per_user_limit, valid_from, valid_to, active, created_by
FROM promo_code
WHERE LOWER(code) = LOWER($1) AND active = true;

-- name: GetPromoCodeByCodeForUpdate :one
-- Row-locking twin of GetPromoCodeByCode, for the redeem path (checkout.go's
-- StartCheckout): locks the promo_code row for the duration of the
-- validate+redeem transaction so concurrent redemptions of the same code
-- serialize instead of all reading a stale max_uses/per_user_limit count.
SELECT id, code, kind, value, max_uses, per_user_limit, valid_from, valid_to, active, created_by
FROM promo_code
WHERE LOWER(code) = LOWER($1) AND active = true
FOR UPDATE;

-- name: GetPromoCodeByID :one
SELECT id, code, kind, value, max_uses, per_user_limit, valid_from, valid_to, active, created_by
FROM promo_code
WHERE id = $1;

-- name: GetPromoCodeByIDForUpdate :one
-- Row-locking lookup by id, for the completion path
-- (billing.ProcessPaymentGrant, called inside the payme/click webhook
-- transactions). StartCheckout's FOR UPDATE only protects the counts as they
-- were when checkout STARTED; the promo_redemption row is not written until
-- the payment completes, so without re-validating under the same lock here a
-- single user can bank an unlimited number of discounted 'created' payments
-- and complete them all. Deliberately does NOT filter on active = true: an
-- admin deactivating a code mid-flight must be visible to the caller as
-- "no longer redeemable", not as a missing row.
SELECT id, code, kind, value, max_uses, per_user_limit, valid_from, valid_to, active, created_by
FROM promo_code
WHERE id = $1
FOR UPDATE;

-- name: CountPromoRedemptions :one
SELECT COUNT(*)::int AS count FROM promo_redemption WHERE promo_code_id = $1;

-- name: CountUserPromoRedemptions :one
SELECT COUNT(*)::int AS count FROM promo_redemption WHERE promo_code_id = $1 AND profile_id = $2;

-- name: CreatePromoRedemption :exec
INSERT INTO promo_redemption (promo_code_id, profile_id, payment_id)
VALUES ($1, $2, $3);


-- name: SetPaymentStatus :exec
UPDATE payment SET status = $2 WHERE id = $1;

-- name: MarkPaymentPaid :exec
UPDATE payment SET status = 'paid', paid_at = now() WHERE id = $1;

-- name: CreatePaymeTransaction :exec
INSERT INTO payme_transaction (payme_id, payment_id, amount_tiyin, state, create_time)
VALUES ($1, $2, $3, 1, $4);

-- name: GetPaymeTransaction :one
SELECT payme_id, payment_id, amount_tiyin, state, reason, create_time, perform_time, cancel_time
FROM payme_transaction WHERE payme_id = $1;

-- name: GetPaymeTransactionForUpdate :one
SELECT payme_id, payment_id, amount_tiyin, state, reason, create_time, perform_time, cancel_time
FROM payme_transaction WHERE payme_id = $1 FOR UPDATE;

-- name: GetActivePaymeTxByPayment :one
SELECT payme_id, state FROM payme_transaction
WHERE payment_id = $1 AND state IN (1, 2) LIMIT 1;

-- name: PerformPaymeTransaction :exec
UPDATE payme_transaction SET state = 2, perform_time = $2 WHERE payme_id = $1;

-- name: CancelPaymeTransaction :exec
UPDATE payme_transaction SET state = $2, reason = $3, cancel_time = $4 WHERE payme_id = $1;

-- name: ListPaymeTransactionsByTime :many
SELECT payme_id, payment_id, amount_tiyin, state, reason, create_time, perform_time, cancel_time
FROM payme_transaction WHERE create_time >= $1 AND create_time <= $2 ORDER BY create_time;

-- name: CreateClickTransaction :one
INSERT INTO click_transaction (click_trans_id, click_paydoc_id, payment_id, amount_uzs)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: GetClickTransactionByClickTransID :one
SELECT id, click_trans_id, click_paydoc_id, payment_id, amount_uzs, state, reason, created_at, confirmed_at, rejected_at
FROM click_transaction WHERE click_trans_id = $1;

-- name: GetClickTransactionByID :one
SELECT id, click_trans_id, click_paydoc_id, payment_id, amount_uzs, state, reason, created_at, confirmed_at, rejected_at
FROM click_transaction WHERE id = $1;

-- name: GetClickTransactionByIDForUpdate :one
SELECT id, click_trans_id, click_paydoc_id, payment_id, amount_uzs, state, reason, created_at, confirmed_at, rejected_at
FROM click_transaction WHERE id = $1 FOR UPDATE;

-- name: GetActiveClickTxByPayment :one
SELECT id, state FROM click_transaction WHERE payment_id = $1 AND state IN (0, 1) LIMIT 1;

-- name: ConfirmClickTransaction :exec
UPDATE click_transaction SET state = 1, confirmed_at = now() WHERE id = $1;

-- name: RejectClickTransaction :exec
UPDATE click_transaction SET state = -1, rejected_at = now(), reason = $2 WHERE id = $1;

-- name: MarkPaymentPending :exec
UPDATE payment SET status = 'pending', provider_txn_id = $2 WHERE id = $1;

-- name: ListMyPayments :many
-- tariff_days comes from the payment snapshot so history shows the term the
-- customer actually bought, even if the tariff was edited since. code/name
-- still come from tariff — those are labels, not terms.
SELECT p.id, p.amount_uzs, p.provider, p.status, p.created_at, p.paid_at,
       t.code AS tariff_code, p.tariff_days_snapshot AS tariff_days,
       COALESCE(tr.name, ftr.name, t.code) AS tariff_name
FROM payment p
JOIN tariff t ON t.id = p.tariff_id
LEFT JOIN tariff_translation tr ON tr.tariff_id = t.id AND tr.locale = $2
LEFT JOIN tariff_translation ftr ON ftr.tariff_id = t.id AND ftr.locale = 'uz-Latn'
WHERE p.profile_id = $1
ORDER BY p.created_at DESC
LIMIT $3;
