-- Defense-in-depth against a race between two DIFFERENT payme_ids for the
-- SAME payment both passing the application-level "no active transaction"
-- check and both inserting a state=1 row: without this, both could later be
-- performed and GrantDays would run twice for one payment. The DB itself
-- now rejects a second concurrent active (state 1 or 2) transaction per
-- payment, independent of any race in the application-level guard.
CREATE UNIQUE INDEX payme_transaction_one_active_per_payment
  ON payme_transaction(payment_id) WHERE state IN (1, 2);
