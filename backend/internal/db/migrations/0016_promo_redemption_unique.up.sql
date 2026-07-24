-- Defense-in-depth backstop for the promo redemption race fixed in
-- billing.StartCheckout: max_uses/per_user_limit are now enforced by
-- locking the promo_code row (SELECT ... FOR UPDATE) for the whole
-- validate+redeem decision, the same pattern already used for
-- payme_transaction (migration 0013) and click_transaction. That row lock
-- is the real defense against exceeding max_uses/per_user_limit — those are
-- *count* limits (per_user_limit can be > 1), not "at most one row" limits,
-- so a single unique index can't encode them directly the way
-- payme_transaction_one_active_per_payment does for its 0/1 case.
--
-- What a unique index CAN usefully guard here: the exact same payment
-- creating two promo_redemption rows. That can only happen if application
-- code ever runs CreatePromoRedemption twice for one payment_id (a retried
-- statement after a transient error, a future refactor that forgets the
-- lock, etc.) — normally impossible once the lock above is in place, but a
-- cheap, always-correct backstop since per (promo_code_id, profile_id,
-- payment_id) there is never a legitimate reason for more than one
-- redemption row. payment_id is nullable in general (promo_redemption
-- predates payment-linked redemptions), so the index is partial: it only
-- applies where payment_id IS NOT NULL, matching how every current writer
-- (StartCheckout, ProcessPaymentGrant) always sets it.
CREATE UNIQUE INDEX promo_redemption_one_per_payment
  ON promo_redemption(promo_code_id, profile_id, payment_id)
  WHERE payment_id IS NOT NULL;
