package bot

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/db/sqlc"
)

// displayName picks the best label Telegram gave us for the ranking.
func displayName(u User) string {
	if n := strings.TrimSpace(u.FirstName); n != "" {
		return n
	}
	if n := strings.TrimSpace(u.Username); n != "" {
		return n
	}
	return "Ishtirokchi"
}

// HandlePollAnswer records one vote. Unlike the old callback path it knows
// who voted, so every participant keeps their own score instead of the first
// tap answering for the whole chat.
func (s *QuizService) HandlePollAnswer(ctx context.Context, pa PollAnswer) error {
	if s == nil || s.Q == nil {
		return nil
	}
	// A retracted vote arrives with an empty option list.
	if len(pa.OptionIDs) == 0 {
		return nil
	}
	poll, err := s.Q.GetQuizPoll(ctx, pa.PollID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// An old game or another process's poll — not our business.
			return nil
		}
		return err
	}

	correctDelta := int32(0)
	if pa.OptionIDs[0] == int(poll.CorrectIdx) {
		correctDelta = 1
	}

	elapsed := time.Since(poll.OpenedAt.Time).Milliseconds()
	if elapsed < 0 {
		elapsed = 0
	}

	return s.Q.UpsertQuizParticipant(ctx, sqlc.UpsertQuizParticipantParams{
		SessionID:    poll.SessionID,
		TgUserID:     pa.User.ID,
		DisplayName:  displayName(pa.User),
		CorrectDelta: correctDelta,
		ElapsedMs:    elapsed,
	})
}
