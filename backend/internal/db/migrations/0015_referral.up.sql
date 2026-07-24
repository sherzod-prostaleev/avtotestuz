CREATE TABLE IF NOT EXISTS user_referral_code (
    user_id UUID PRIMARY KEY REFERENCES profile(id) ON DELETE CASCADE,
    code VARCHAR(32) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS referral (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    referrer_id UUID NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
    referee_id UUID NOT NULL UNIQUE REFERENCES profile(id) ON DELETE CASCADE,
    referral_code VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    rewarded_at TIMESTAMPTZ,
    CONSTRAINT chk_no_self_referral CHECK (referrer_id <> referee_id)
);

CREATE INDEX idx_referral_referrer ON referral(referrer_id);
CREATE INDEX idx_user_referral_code ON user_referral_code(code);
