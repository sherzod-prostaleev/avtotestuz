-- U-23: drop unused parallel referral table from migration 0003.
-- Live attribution is `referral` (0015); nothing in app code reads referral_attribution.
-- Antifraud design explicitly rejected reviving this table (two attribution truths).
DROP TABLE IF EXISTS referral_attribution;
