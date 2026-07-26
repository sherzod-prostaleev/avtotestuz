package billing

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/db/sqlc"
)

const (
	referralAttachWindowKey     = "referral_attach_window_days"
	referralAttachWindowDefault = 30
)

var (
	ErrReferralSelf            = errors.New("cannot apply your own referral code")
	ErrReferralNotFound        = errors.New("referral code not found")
	ErrReferralAlreadyApplied  = errors.New("referral code already applied for this account")
	ErrReferralNotEligiblePaid = errors.New("referee already has a paid payment")
	ErrReferralWindowClosed    = errors.New("referral attach window closed")
)

type ReferralStats struct {
	ReferralCode       string `json:"referral_code"`
	InviteURL          string `json:"invite_url"`
	TotalInvited       int64  `json:"total_invited"`
	TotalRewarded      int64  `json:"total_rewarded"`
	EarnedUzs          int64  `json:"earned_uzs"`
	AvailableBalanceUzs int64 `json:"available_balance_uzs"`
	CommissionPercent  int32  `json:"commission_percent"`
	// Deprecated: kept for older clients; always 0 (cash rewards replaced VIP days).
	BonusDaysEarned int64 `json:"bonus_days_earned"`
}

func generateRandomCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	var sb strings.Builder
	sb.WriteString("REF-")
	for i := 0; i < 6; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			sb.WriteString("X")
		} else {
			sb.WriteByte(charset[num.Int64()])
		}
	}
	return sb.String()
}

func (s Service) GetOrCreateReferralCode(ctx context.Context, userID uuid.UUID) (string, error) {
	refCodeRow, err := s.Q.GetUserReferralCode(ctx, userID)
	if err == nil {
		return refCodeRow.Code, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	// Create new code with collision retry loop.
	const maxAttempts = 20
	var lastErr error
	for attempts := 0; attempts < maxAttempts; attempts++ {
		code := generateRandomCode()
		created, err := s.Q.CreateUserReferralCode(ctx, sqlc.CreateUserReferralCodeParams{
			UserID: userID,
			Code:   code,
		})
		if err == nil {
			return created.Code, nil
		}
		lastErr = err
		// If user_id already got created concurrently, fetch it
		existing, fetchErr := s.Q.GetUserReferralCode(ctx, userID)
		if fetchErr == nil {
			return existing.Code, nil
		}
	}
	// Report the real reason: a persistent non-collision failure (bad
	// connection, permissions) used to surface as "collisions exhausted",
	// pointing every future debugger at the random generator instead.
	return "", fmt.Errorf("generate unique referral code after %d attempts: %w", maxAttempts, lastErr)
}

func (s Service) ApplyReferralCode(ctx context.Context, refereeID uuid.UUID, rawCode string) error {
	code := strings.TrimSpace(rawCode)
	if code == "" {
		return ErrReferralNotFound
	}

	owner, err := s.Q.GetReferralCodeOwner(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReferralNotFound
		}
		return err
	}

	if owner.UserID == refereeID {
		return ErrReferralSelf
	}

	paidCount, err := s.Q.CountPaidPaymentsForProfile(ctx, refereeID)
	if err != nil {
		return fmt.Errorf("count paid payments: %w", err)
	}
	if paidCount > 0 {
		return ErrReferralNotEligiblePaid
	}

	profile, err := s.Q.GetProfileByID(ctx, refereeID)
	if err != nil {
		return fmt.Errorf("load referee profile: %w", err)
	}
	windowDays := referralAttachWindowDefault
	if cfg, cfgErr := s.Q.GetLimitConfig(ctx, referralAttachWindowKey); cfgErr == nil {
		windowDays = int(cfg.FreeValue)
	}
	if !profile.CreatedAt.Valid {
		return fmt.Errorf("referee profile has no created_at")
	}
	deadline := profile.CreatedAt.Time.Add(time.Duration(windowDays) * 24 * time.Hour)
	if time.Now().After(deadline) {
		return ErrReferralWindowClosed
	}

	_, err = s.Q.CreateReferral(ctx, sqlc.CreateReferralParams{
		ReferrerID:   owner.UserID,
		RefereeID:    refereeID,
		ReferralCode: owner.Code,
	})
	if err != nil {
		// Only a unique violation on referral.referee_id means "already
		// applied". This used to map *every* failure to that, so a deleted
		// profile (FK violation), the chk_no_self_referral CHECK, a dropped
		// connection or a statement timeout all told a user who had never
		// applied a code that they already had — with no path forward, and
		// with the infrastructure error invisible to monitoring.
		if isUniqueViolation(err) {
			return ErrReferralAlreadyApplied
		}
		return fmt.Errorf("create referral: %w", err)
	}

	return nil
}

func (s Service) GetReferralStats(ctx context.Context, userID uuid.UUID) (*ReferralStats, error) {
	code, err := s.GetOrCreateReferralCode(ctx, userID)
	if err != nil {
		return nil, err
	}

	stats, err := s.Q.GetReferralStatsForUser(ctx, userID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	earned, err := s.Q.GetReferralEarnedCommission(ctx, userID)
	if err != nil {
		return nil, err
	}
	balance, err := s.Q.GetReferralBalance(ctx, userID)
	if err != nil {
		return nil, err
	}
	percent, err := s.Q.GetReferralCommissionPercent(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &ReferralStats{
		ReferralCode:        code,
		InviteURL:           fmt.Sprintf("%s/r/%s", s.publicBaseURL(), code),
		TotalInvited:        stats.TotalInvited,
		TotalRewarded:       stats.TotalRewarded,
		EarnedUzs:           earned,
		AvailableBalanceUzs: balance,
		CommissionPercent:   percent,
		BonusDaysEarned:     0,
	}, nil
}

// processReferralRewardOnPayment claims a pending referral for the buyer,
// grants them tariff-mapped bonus VIP days, and credits the referrer a cash
// commission on the paid amount. Referrer VIP days are intentionally not granted.
func (s Service) processReferralRewardOnPayment(ctx context.Context, refereeID uuid.UUID, payment sqlc.GetPaymentForPaymeRow) error {
	// Claim first so concurrent payments for the same referee cannot both reward.
	claimed, err := s.Q.ClaimPendingReferralForReferee(ctx, refereeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}

	bonusDays := BuyerReferralBonusDays(payment.TariffDays)
	if bonusDays > 0 {
		note := fmt.Sprintf("Referral buyer bonus (+%d days)", bonusDays)
		if _, err := s.GrantDaysForPayment(ctx, refereeID, bonusDays, "referral_buyer", note, uuid.NullUUID{}, uuid.NullUUID{UUID: payment.ID, Valid: true}); err != nil {
			return fmt.Errorf("grant buyer referral bonus days: %w", err)
		}
	}

	percent, err := s.Q.GetReferralCommissionPercent(ctx, claimed.ReferrerID)
	if err != nil {
		return fmt.Errorf("load referrer commission percent: %w", err)
	}
	if err := s.creditReferralCommission(ctx, claimed.ReferrerID, refereeID, payment.ID, payment.AmountUzs, percent, payment.TariffDays); err != nil {
		return err
	}
	return nil
}

// TryApplyReferralAtCheckout applies a referral code when the checkout promo
// field holds a personal REF code. Returns (true, nil) if the code was a
// referral code (applied or already applied). Returns (false, nil) if it is
// not a referral code so the caller may treat it as a discount promo.
func (s Service) TryApplyReferralAtCheckout(ctx context.Context, refereeID uuid.UUID, rawCode string) (wasReferral bool, err error) {
	code := strings.TrimSpace(rawCode)
	if code == "" {
		return false, nil
	}
	if _, err := s.Q.GetReferralCodeOwner(ctx, code); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if err := s.ApplyReferralCode(ctx, refereeID, code); err != nil {
		if errors.Is(err, ErrReferralAlreadyApplied) {
			return true, nil
		}
		return true, err
	}
	return true, nil
}
