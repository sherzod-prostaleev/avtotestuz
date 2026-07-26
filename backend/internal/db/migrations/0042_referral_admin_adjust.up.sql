-- Allow admin to credit/debit referral wallet via ledger entries.
ALTER TABLE referral_ledger DROP CONSTRAINT referral_ledger_entry_type_check;
ALTER TABLE referral_ledger ADD CONSTRAINT referral_ledger_entry_type_check
  CHECK (entry_type IN (
    'commission',
    'payout_hold',
    'payout_paid',
    'payout_reject_release',
    'clawback',
    'admin_adjust'
  ));
