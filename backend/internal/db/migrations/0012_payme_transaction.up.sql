-- Payme's own transaction record (separate from our `payment` order): tracks
-- the Payme transaction id, integer state and the millisecond timestamps the
-- Merchant API must echo back in CheckTransaction / GetStatement.
CREATE TABLE payme_transaction (
  payme_id     text PRIMARY KEY,                  -- Payme params.id
  payment_id   uuid NOT NULL REFERENCES payment(id),
  amount_tiyin bigint NOT NULL,
  state        int  NOT NULL,                     -- 1 pending, 2 paid, -1/-2 cancelled
  reason       int,                               -- cancel reason code
  create_time  bigint NOT NULL,                   -- ms
  perform_time bigint NOT NULL DEFAULT 0,         -- ms (0 = not performed)
  cancel_time  bigint NOT NULL DEFAULT 0,         -- ms (0 = not cancelled)
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX payme_transaction_payment_idx ON payme_transaction(payment_id);
CREATE INDEX payme_transaction_time_idx ON payme_transaction(create_time);
