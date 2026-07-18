package progress

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/db/sqlc"
)

// dailyGoalDefaultConfigKey is the limit_config key holding the default
// daily-question goal handed to a profile that has never recorded activity
// (and thus has no streak row / chosen goal of its own yet).
const dailyGoalDefaultConfigKey = "daily_goal_default"

type Service struct {
	Q *sqlc.Queries
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

// GetStreak returns a profile's current streak state. A profile that has
// never answered anything has no streak row yet — that's a normal state,
// not an error, matching how billing.Service.Status returns false,nil,nil
// for a fresh profile rather than erroring on pgx.ErrNoRows.
func (s *Service) GetStreak(ctx context.Context, profileID uuid.UUID) (StreakView, error) {
	row, err := s.Q.GetStreak(ctx, profileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			cfg, cfgErr := s.Q.GetLimitConfig(ctx, dailyGoalDefaultConfigKey)
			if cfgErr != nil {
				return StreakView{}, cfgErr
			}
			return StreakView{DailyGoal: int(cfg.FreeValue)}, nil
		}
		return StreakView{}, err
	}
	return streakToView(row), nil
}

// RecordActivity bumps the caller's streak for "one answered question
// today" — called once per answered question from internal/session.
func (s *Service) RecordActivity(ctx context.Context, profileID uuid.UUID) (StreakView, error) {
	row, err := s.Q.GetStreak(ctx, profileID)
	isNew := false
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return StreakView{}, err
		}
		isNew = true
		row = sqlc.Streak{ProfileID: profileID}
	}

	dailyGoal := row.DailyGoal
	if isNew {
		cfg, err := s.Q.GetLimitConfig(ctx, dailyGoalDefaultConfigKey)
		if err != nil {
			return StreakView{}, err
		}
		dailyGoal = cfg.FreeValue
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
	return streakToView(saved), nil
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
