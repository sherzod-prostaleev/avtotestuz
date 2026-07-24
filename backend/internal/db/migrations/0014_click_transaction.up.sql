CREATE TABLE click_transaction (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  click_trans_id  text NOT NULL,
  click_paydoc_id text,
  payment_id      uuid NOT NULL REFERENCES payment(id),
  amount_uzs      bigint NOT NULL CHECK (amount_uzs >= 0),
  state           int NOT NULL DEFAULT 0 CHECK (state IN (0, 1, -1)),
  reason          text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  confirmed_at    timestamptz,
  rejected_at     timestamptz
);
CREATE INDEX click_transaction_payment_idx ON click_transaction(payment_id);
CREATE INDEX click_transaction_click_trans_idx ON click_transaction(click_trans_id);
-- Concurrent-double-grant guard, built in from the start (M2-02 had to
-- add this after merge via a follow-up migration — don't repeat that here).
CREATE UNIQUE INDEX click_transaction_one_active_per_payment
  ON click_transaction(payment_id) WHERE state IN (0, 1);
