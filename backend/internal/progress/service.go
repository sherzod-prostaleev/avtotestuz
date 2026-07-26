package progress

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/learning"
)

// dailyGoalDefaultConfigKey is the limit_config key holding the free/VIP
// daily-question goal shown on the dashboard and written into streak rows.
const dailyGoalDefaultConfigKey = "daily_goal_default"

type Service struct {
	Q        *sqlc.Queries
	Learning *learning.Service // required for MigrateDemoProgress
	Billing  billing.Service   // VIP status for daily_goal_default.vip_value
}

func NewService(q *sqlc.Queries) *Service {
	return &Service{Q: q}
}

// todayUTC truncates the current instant to midnight UTC, the same
// day-boundary convention internal/session uses for its daily practice
// limit (see clampToDailyAllowance's startOfDay).
func todayUTC() time.Time {
	return time.Now().UTC().Truncate(24 * time.Hour)
}

// SaveQuestion bookmarks questionID for profileID. Idempotent: saving an
// already-saved question is not an error.
func (s *Service) SaveQuestion(ctx context.Context, profileID, questionID uuid.UUID) error {
	return s.Q.SaveQuestion(ctx, sqlc.SaveQuestionParams{ProfileID: profileID, QuestionID: questionID})
}

// UnsaveQuestion removes a bookmark. Removing a question that was never
// saved (or already removed) is not an error.
func (s *Service) UnsaveQuestion(ctx context.Context, profileID, questionID uuid.UUID) error {
	return s.Q.UnsaveQuestion(ctx, sqlc.UnsaveQuestionParams{ProfileID: profileID, QuestionID: questionID})
}

type SavedItem struct {
	QuestionID uuid.UUID
	CreatedAt  time.Time
}

// ListSaved returns a profile's bookmarked questions, most recently saved
// first.
func (s *Service) ListSaved(ctx context.Context, profileID uuid.UUID) ([]SavedItem, error) {
	rows, err := s.Q.ListSavedQuestions(ctx, profileID)
	if err != nil {
		return nil, err
	}
	items := make([]SavedItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, SavedItem{QuestionID: r.QuestionID, CreatedAt: r.CreatedAt.Time})
	}
	return items, nil
}

type StreakView struct {
	Current, Best, TodayDone, DailyGoal int
	LastActiveDate                      *time.Time
}

// dailyGoalFor returns the admin-configured daily goal for this profile's
// entitlement (free vs VIP). Always reads live limit_config so Settings →
// Limits changes apply immediately, not only for brand-new streak rows.
func (s *Service) dailyGoalFor(ctx context.Context, profileID uuid.UUID) (int32, error) {
	cfg, err := s.Q.GetLimitConfig(ctx, dailyGoalDefaultConfigKey)
	if err != nil {
		return 0, err
	}
	goal := cfg.FreeValue
	if s.Billing.Q == nil {
		return goal, nil
	}
	active, _, err := s.Billing.Status(ctx, profileID)
	if err != nil {
		return 0, err
	}
	if active {
		goal = cfg.VipValue
	}
	return goal, nil
}

// GetStreak returns a profile's current streak state. A profile that has
// never answered anything has no streak row yet — that's a normal state,
// not an error, matching how billing.Service.Status returns false,nil,nil
// for a fresh profile rather than erroring on pgx.ErrNoRows.
//
// DailyGoal always comes from limit_config (free/VIP), not the frozen
// streak.daily_goal column, so admin updates apply on the next read.
func (s *Service) GetStreak(ctx context.Context, profileID uuid.UUID) (StreakView, error) {
	goal, err := s.dailyGoalFor(ctx, profileID)
	if err != nil {
		return StreakView{}, err
	}
	row, err := s.Q.GetStreak(ctx, profileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return StreakView{DailyGoal: int(goal)}, nil
		}
		return StreakView{}, err
	}
	view := streakToView(row)
	view.DailyGoal = int(goal)
	return view, nil
}

// RecordActivity bumps the caller's streak for "one answered question
// today" — called once per answered question from internal/session.
func (s *Service) RecordActivity(ctx context.Context, profileID uuid.UUID) (StreakView, error) {
	row, err := s.Q.GetStreak(ctx, profileID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return StreakView{}, err
		}
		row = sqlc.Streak{ProfileID: profileID}
	}

	dailyGoal, err := s.dailyGoalFor(ctx, profileID)
	if err != nil {
		return StreakView{}, err
	}

	state := streakStateFromRow(row)
	updated := BumpStreak(state, todayUTC())

	var lastActiveDate pgtype.Date
	if updated.LastActiveDate != nil {
		lastActiveDate = pgtype.Date{Time: *updated.LastActiveDate, Valid: true}
	}

	saved, err := s.Q.UpsertStreak(ctx, sqlc.UpsertStreakParams{
		ProfileID:      profileID,
		Current:        int32(updated.Current),
		Best:           int32(updated.Best),
		LastActiveDate: lastActiveDate,
		DailyGoal:      dailyGoal,
		TodayDone:      int32(updated.TodayDone),
	})
	if err != nil {
		return StreakView{}, err
	}
	view := streakToView(saved)
	view.DailyGoal = int(dailyGoal)
	return view, nil
}

func streakStateFromRow(row sqlc.Streak) StreakState {
	var lastActive *time.Time
	if row.LastActiveDate.Valid {
		t := row.LastActiveDate.Time
		lastActive = &t
	}
	return StreakState{
		Current:        int(row.Current),
		Best:           int(row.Best),
		TodayDone:      int(row.TodayDone),
		LastActiveDate: lastActive,
	}
}

func streakToView(row sqlc.Streak) StreakView {
	var lastActive *time.Time
	if row.LastActiveDate.Valid {
		t := row.LastActiveDate.Time
		lastActive = &t
	}
	return StreakView{
		Current:        int(row.Current),
		Best:           int(row.Best),
		TodayDone:      int(row.TodayDone),
		DailyGoal:      int(row.DailyGoal),
		LastActiveDate: lastActive,
	}
}
