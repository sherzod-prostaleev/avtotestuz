-- Unpaid Humo/manual checkouts auto-cancel after 4 hours (rows are kept).
-- Partial index supports ExpireStaleManualPayments without a seq scan of paid history.
CREATE INDEX IF NOT EXISTS payment_manual_open_created_idx
  ON payment (created_at)
  WHERE provider = 'manual' AND status IN ('created', 'pending');

COMMENT ON INDEX payment_manual_open_created_idx IS
  'Open Humo/manual checkouts; sweeper cancels rows older than 4 hours.';
