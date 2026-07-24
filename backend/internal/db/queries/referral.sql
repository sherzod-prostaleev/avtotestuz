-- name: GetUserReferralCode :one
SELECT user_id, code, created_at
FROM user_referral_code
WHERE user_id = $1;

-- name: GetReferralCodeOwner :one
SELECT user_id, code, created_at
FROM user_referral_code
WHERE LOWER(code) = LOWER($1);

-- name: CreateUserReferralCode :one
INSERT INTO user_referral_code (user_id, code)
VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE SET user_id = EXCLUDED.user_id
RETURNING user_id, code, created_at;

-- name: CreateReferral :one
INSERT INTO referral (referrer_id, referee_id, referral_code, status)
VALUES ($1, $2, $3, 'pending')
RETURNING id, referrer_id, referee_id, referral_code, status, created_at, rewarded_at;

-- name: GetPendingReferralForReferee :one
SELECT id, referrer_id, referee_id, referral_code, status, created_at, rewarded_at
FROM referral
WHERE referee_id = $1 AND status = 'pending';

-- name: MarkReferralRewarded :exec
UPDATE referral
SET status = 'rewarded', rewarded_at = NOW()
WHERE id = $1 AND status = 'pending';

-- name: GetReferralStatsForUser :one
SELECT 
    COUNT(*)::bigint AS total_invited,
    COUNT(CASE WHEN status = 'rewarded' THEN 1 END)::bigint AS total_rewarded
FROM referral
WHERE referrer_id = $1;
