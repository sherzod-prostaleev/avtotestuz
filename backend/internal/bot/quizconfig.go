package bot

import (
	"context"

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
