package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/db/sqlc"
)

var (
	ErrReferralPayoutNotFound   = errors.New("referral payout not found")
	ErrReferralPayoutNotPending = errors.New("referral payout is not pending")
)

type ReferralPayoutRow struct {
	ID           uuid.UUID  `json:"id"`
	ProfileID    uuid.UUID  `json:"profile_id"`
	ProfilePhone string     `json:"profile_phone"`
	ProfileName  string     `json:"profile_name"`
	AmountUzs    int64      `json:"amount_uzs"`
	CardNumber   string     `json:"card_number"`
	CardMasked   string     `json:"card_masked"`
	CardNetwork  string     `json:"card_network"`
	Status       string     `json:"status"`
	AdminNote    string     `json:"admin_note"`
	ProcessedBy  *uuid.UUID `json:"processed_by,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ProcessedAt  *time.Time `json:"processed_at,omitempty"`
}

type ReferralPayoutListResult struct {
	Items []ReferralPayoutRow `json:"items"`
	Total int64               `json:"total"`
}

type UserReferralAdminStats struct {
	TotalInvited       int64              `json:"total_invited"`
	TotalRewarded      int64              `json:"total_rewarded"`
	EarnedUzs          int64              `json:"earned_uzs"`
	BalanceUzs         int64              `json:"balance_uzs"`
	CommissionPercent  int32              `json:"commission_percent"`
	Referees           []RefereeAdminRow  `json:"referees"`
}

type RefereeAdminRow struct {
	ID             uuid.UUID  `json:"id"`
	RefereeID      uuid.UUID  `json:"referee_id"`
	RefereePhone   string     `json:"referee_phone"`
	RefereeName    string     `json:"referee_name"`
	ReferralCode   string     `json:"referral_code"`
	Status         string     `json:"status"`
	CommissionUzs  int64      `json:"commission_uzs"`
	CreatedAt      time.Time  `json:"created_at"`
	RewardedAt     *time.Time `json:"rewarded_at,omitempty"`
}

func maskCard(digits string) string {
	if len(digits) < 4 {
		return "****"
	}
	return "**** **** **** " + digits[len(digits)-4:]
}

func (s Store) ListReferralPayouts(ctx context.Context, status string, limit, offset int32) (*ReferralPayoutListResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	q := sqlc.New(s.Pool)
	total, err := q.CountReferralPayouts(ctx, status)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListReferralPayouts(ctx, sqlc.ListReferralPayoutsParams{
		Column1: status,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, err
	}
	items := make([]ReferralPayoutRow, 0, len(rows))
	for _, row := range rows {
		item := ReferralPayoutRow{
			ID:           row.ID,
			ProfileID:    row.ProfileID,
			ProfilePhone: row.ProfilePhone,
			ProfileName:  row.ProfileName,
			AmountUzs:    row.AmountUzs,
			CardNumber:   row.CardNumber,
			CardMasked:   maskCard(row.CardNumber),
			CardNetwork:  row.CardNetwork,
			Status:       row.Status,
			AdminNote:    row.AdminNote,
			CreatedAt:    row.CreatedAt.Time,
		}
		if row.ProcessedBy.Valid {
			id := row.ProcessedBy.UUID
			item.ProcessedBy = &id
		}
		if row.ProcessedAt.Valid {
			t := row.ProcessedAt.Time
			item.ProcessedAt = &t
		}
		items = append(items, item)
	}
	return &ReferralPayoutListResult{Items: items, Total: total}, nil
}

func (s Store) MarkReferralPayoutPaid(ctx context.Context, payoutID, adminID uuid.UUID, note string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)
	payout, err := q.GetReferralPayoutForUpdate(ctx, payoutID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReferralPayoutNotFound
		}
		return err
	}
	if payout.Status != "pending" {
		return ErrReferralPayoutNotPending
	}
	if _, err := q.MarkReferralPayoutPaid(ctx, sqlc.MarkReferralPayoutPaidParams{
		ID:          payoutID,
		ProcessedBy: uuid.NullUUID{UUID: adminID, Valid: true},
		AdminNote:   note,
	}); err != nil {
		return err
	}
	// Balance already reduced by payout_hold; paid is status-only.
	return tx.Commit(ctx)
}

func (s Store) RejectReferralPayout(ctx context.Context, payoutID, adminID uuid.UUID, note string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)
	payout, err := q.GetReferralPayoutForUpdate(ctx, payoutID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReferralPayoutNotFound
		}
		return err
	}
	if payout.Status != "pending" {
		return ErrReferralPayoutNotPending
	}
	if _, err := q.MarkReferralPayoutRejected(ctx, sqlc.MarkReferralPayoutRejectedParams{
		ID:          payoutID,
		ProcessedBy: uuid.NullUUID{UUID: adminID, Valid: true},
		AdminNote:   note,
	}); err != nil {
		return err
	}
	meta, _ := json.Marshal(map[string]any{
		"admin_id": adminID.String(),
		"reason":   "rejected",
	})
	if _, err := q.InsertReferralLedger(ctx, sqlc.InsertReferralLedgerParams{
		ProfileID: payout.ProfileID,
		EntryType: "payout_reject_release",
		AmountUzs: payout.AmountUzs,
		PaymentID: uuid.NullUUID{},
		PayoutID:  uuid.NullUUID{UUID: payout.ID, Valid: true},
		Meta:      meta,
	}); err != nil {
		return fmt.Errorf("release hold: %w", err)
	}
	return tx.Commit(ctx)
}

func (s Store) GetUserReferralAdmin(ctx context.Context, profileID uuid.UUID) (*UserReferralAdminStats, error) {
	q := sqlc.New(s.Pool)
	stats, err := q.GetAdminReferralUserStats(ctx, profileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReferralPayoutNotFound
		}
		return nil, err
	}
	refs, err := q.ListRefereesForReferrer(ctx, sqlc.ListRefereesForReferrerParams{
		ReferrerID: profileID,
		Limit:      100,
	})
	if err != nil {
		return nil, err
	}
	out := &UserReferralAdminStats{
		TotalInvited:      stats.TotalInvited,
		TotalRewarded:     stats.TotalRewarded,
		EarnedUzs:         stats.EarnedUzs,
		BalanceUzs:        stats.BalanceUzs,
		CommissionPercent: stats.CommissionPercent,
		Referees:          make([]RefereeAdminRow, 0, len(refs)),
	}
	for _, r := range refs {
		row := RefereeAdminRow{
			ID:            r.ID,
			RefereeID:     r.RefereeID,
			RefereePhone:  r.RefereePhone,
			RefereeName:   r.RefereeName,
			ReferralCode:  r.ReferralCode,
			Status:        r.Status,
			CommissionUzs: r.CommissionUzs,
			CreatedAt:     r.CreatedAt.Time,
		}
		if r.RewardedAt.Valid {
			t := r.RewardedAt.Time
			row.RewardedAt = &t
		}
		out.Referees = append(out.Referees, row)
	}
	return out, nil
}

func (s Store) UpdateUserReferralCommissionPercent(ctx context.Context, profileID uuid.UUID, percent int32) (int32, error) {
	if percent < 0 || percent > 100 {
		return 0, fmt.Errorf("percent must be 0-100")
	}
	q := sqlc.New(s.Pool)
	row, err := q.UpdateReferralCommissionPercent(ctx, sqlc.UpdateReferralCommissionPercentParams{
		ID:                        profileID,
		ReferralCommissionPercent: percent,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrReferralPayoutNotFound
		}
		return 0, err
	}
	return row.ReferralCommissionPercent, nil
}
