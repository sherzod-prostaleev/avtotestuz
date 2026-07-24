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
INSERT INTO payment (profile_id, tariff_id, amount_uzs, provider, status, idempotency_key)
VALUES ($1, $2, $3, $4, 'created', $5)
RETURNING id;

-- name: GetPaymentForPayme :one
SELECT p.id, p.profile_id, p.tariff_id, p.amount_uzs, p.status, t.days AS tariff_days
FROM payment p JOIN tariff t ON t.id = p.tariff_id
WHERE p.id = $1;

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
