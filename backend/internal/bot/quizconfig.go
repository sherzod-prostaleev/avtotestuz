package bot

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Defaults used when limit_config has no row or the read fails. The bot must
// keep running with a sane game rather than refuse to start a quiz.
const (
	defaultQuizSeconds   = 10
	defaultQuizQuestions = 10

	limitKeyQuizSeconds   = "tg_quiz_seconds"
	limitKeyQuizQuestions = "tg_quiz_questions"

	minQuizQuestions = 1
	maxQuizQuestions = 50
)

func (s *QuizService) limitValue(ctx context.Context, key string, fallback int) int {
	if s == nil || s.Q == nil {
		return fallback
	}
	v, err := s.Q.GetLimitConfigValue(ctx, key)
	if err != nil {
		s.logger().Debug("quiz: limit_config read failed, using default",
			zap.String("key", key), zap.Error(err))
		return fallback
	}
	return int(v)
}

// quizSeconds is the per-question countdown, clamped to what sendPoll accepts
// so a mistyped admin value cannot make every poll fail.
func (s *QuizService) quizSeconds(ctx context.Context) int {
	v := s.limitValue(ctx, limitKeyQuizSeconds, defaultQuizSeconds)
	if v < pollMinOpenPeriod {
		return pollMinOpenPeriod
	}
	if v > pollMaxOpenPeriod {
		return pollMaxOpenPeriod
	}
	return v
}

// quizQuestionCount is how many questions one game asks.
func (s *QuizService) quizQuestionCount(ctx context.Context) int {
	v := s.limitValue(ctx, limitKeyQuizQuestions, defaultQuizQuestions)
	if v < minQuizQuestions {
		return minQuizQuestions
	}
	if v > maxQuizQuestions {
		return maxQuizQuestions
	}
	return v
}

// winnerStickerID is optional decoration. Empty means no sticker is sent —
// an unverified file_id would be a runtime error on every finished game.
func (s *QuizService) winnerStickerID() string {
	if s == nil {
		return ""
	}
	return s.WinnerSticker
}

// NewAdvanceScheduler returns a QuizService.Advance implementation. It runs
// detached because the caller is a webhook handler or a long-poll loop —
// both must return promptly — and the next question is due only after the
// poll's own countdown has run out.
//
// Each call for a chat supersedes any earlier one still waiting: a per-chat
// generation counter is bumped on every call, and a fired timer checks it is
// still current before acting. Without this, a chat that gets a manual
// /next (which itself schedules a fresh timer) before its previous timer
// elapses would be advanced twice — once for the manual tap, once for the
// stale timer — silently skipping a question out from under the group.
//
// A closed pool or any other advance error is logged and dropped, never
// panicked: this goroutine outlives the request that spawned it, so nothing
// is listening for its return value, and a panic here would take down
// update handling for every chat, not just this one.
//
// gen never shrinks — it keeps one entry per chat that has ever scheduled an
// advance, for the life of the process. That is intentional, not an
// oversight: deleting a chat's entry after a successful fire would let its
// counter restart at 1 on the next schedule, and an older pending timer
// captured before the delete could then match that restarted value and fire
// as if it were current. A slowly growing map of int64->uint64, one entry
// per distinct chat the bot has ever run a quiz in, is not worth that risk.
func NewAdvanceScheduler(svc *QuizService, log *zap.Logger) func(int64, time.Duration) {
	if log == nil {
		log = zap.NewNop()
	}
	var mu sync.Mutex
	gen := make(map[int64]uint64)

	return func(chatID int64, after time.Duration) {
		mu.Lock()
		gen[chatID]++
		myGen := gen[chatID]
		mu.Unlock()

		go func() {
			timer := time.NewTimer(after)
			defer timer.Stop()
			<-timer.C

			mu.Lock()
			current := gen[chatID]
			mu.Unlock()
			if current != myGen {
				// Superseded by a newer schedule for this chat — that call
				// already asked the next question, so acting on this one
				// would ask a second.
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			// ContinueScheduledGame, not StartOrNext: a timer that fires after
			// /stop, an idle timeout, or the quiz flag going off must not
			// resurrect a game that no longer has an active session.
			if err := svc.ContinueScheduledGame(ctx, chatID); err != nil {
				log.Warn("quiz: scheduled advance failed",
					zap.Int64("chat_id", chatID), zap.Error(err))
			}
		}()
	}
}
