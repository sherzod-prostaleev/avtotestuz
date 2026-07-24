package billing

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/db/sqlc"
)

var (
	ErrReferralSelf           = errors.New("cannot apply your own referral code")
	ErrReferralNotFound       = errors.New("referral code not found")
	ErrReferralAlreadyApplied = errors.New("referral code already applied for this account")
)

type ReferralStats struct {
	ReferralCode    string `json:"referral_code"`
	InviteURL       string `json:"invite_url"`
	TotalInvited    int64  `json:"total_invited"`
	TotalRewarded   int64  `json:"total_rewarded"`
	BonusDaysEarned int64  `json:"bonus_days_earned"`
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

	// Create new code with collision retry loop
	for attempts := 0; attempts < 20; attempts++ {
		code := generateRandomCode()
		created, err := s.Q.CreateUserReferralCode(ctx, sqlc.CreateUserReferralCodeParams{
			UserID: userID,
			Code:   code,
		})
		if err == nil {
			return created.Code, nil
		}
		// If user_id already got created concurrently, fetch it
		existing, fetchErr := s.Q.GetUserReferralCode(ctx, userID)
		if fetchErr == nil {
			return existing.Code, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique referral code after 5 attempts")
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

	_, err = s.Q.CreateReferral(ctx, sqlc.CreateReferralParams{
		ReferrerID:   owner.UserID,
		RefereeID:    refereeID,
		ReferralCode: owner.Code,
	})
	if err != nil {
		// Unique constraint violation on referee_id
		return ErrReferralAlreadyApplied
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

	totalInvited := stats.TotalInvited
	totalRewarded := stats.TotalRewarded
	bonusDays := totalRewarded * 7

	return &ReferralStats{
		ReferralCode:    code,
		InviteURL:       fmt.Sprintf("https://avtotest.uz/r/%s", code),
		TotalInvited:    totalInvited,
		TotalRewarded:   totalRewarded,
		BonusDaysEarned: bonusDays,
	}, nil
}

func (s Service) processReferralRewardOnPayment(ctx context.Context, refereeID uuid.UUID) error {
	pending, err := s.Q.GetPendingReferralForReferee(ctx, refereeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // No pending referral for this user
		}
		return err
	}

	// Grant +7 VIP days to referrer
	if _, err := s.GrantDays(ctx, pending.ReferrerID, 7, "referral", "Referral bonus for inviting friend", uuid.NullUUID{}); err != nil {
		return fmt.Errorf("failed to grant referral bonus days to referrer: %w", err)
	}

	// Mark referral as rewarded
	if err := s.Q.MarkReferralRewarded(ctx, pending.ID); err != nil {
		return fmt.Errorf("failed to mark referral as rewarded: %w", err)
	}

	return nil
}
