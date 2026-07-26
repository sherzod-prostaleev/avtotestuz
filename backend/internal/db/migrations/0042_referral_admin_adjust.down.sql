DELETE FROM referral_ledger WHERE entry_type = 'admin_adjust';
ALTER TABLE referral_ledger DROP CONSTRAINT referral_ledger_entry_type_check;
ALTER TABLE referral_ledger ADD CONSTRAINT referral_ledger_entry_type_check
  CHECK (entry_type IN (
    'commission',
    'payout_hold',
    'payout_paid',
    'payout_reject_release',
    'clawback'
  ));
