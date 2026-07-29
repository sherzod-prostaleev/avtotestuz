package bot

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
)

// buildPollRequest turns a question and its answers into a quiz poll.
// Oversize text is an error, not something to truncate: the corpus filter is
// supposed to have excluded it upstream, so hitting it means the filter has
// a hole worth surfacing.
func buildPollRequest(question string, answers []sqlc.ListQuizAnswersRow, explanation string, seconds int, replyTo int64) (PollRequest, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return PollRequest{}, fmt.Errorf("question text is empty")
	}
	if n := utf8.RuneCountInString(question); n > pollQuestionMaxChars {
		return PollRequest{}, fmt.Errorf("question is %d chars, poll limit is %d", n, pollQuestionMaxChars)
	}

	options := make([]string, 0, len(answers))
	correctIdx := -1
	for i, a := range answers {
		text := strings.TrimSpace(a.Text)
		if text == "" {
			return PollRequest{}, fmt.Errorf("answer %d has no text", i)
		}
		if n := utf8.RuneCountInString(text); n > pollOptionMaxChars {
			return PollRequest{}, fmt.Errorf("answer %d is %d chars, poll limit is %d", i, n, pollOptionMaxChars)
		}
		if a.IsCorrect && correctIdx < 0 {
			correctIdx = i
		}
		options = append(options, text)
	}
	if len(options) < pollMinOptions || len(options) > pollMaxOptions {
		return PollRequest{}, fmt.Errorf("question has %d answers, poll needs %d..%d", len(options), pollMinOptions, pollMaxOptions)
	}
	if correctIdx < 0 {
		return PollRequest{}, fmt.Errorf("no answer marked correct")
	}

	return PollRequest{
		Question:    question,
		Options:     options,
		CorrectIdx:  correctIdx,
		Explanation: strings.TrimSpace(explanation),
		OpenPeriod:  seconds,
		ReplyTo:     replyTo,
	}, nil
}

// pickPollableQuestionID prefers an illustrated question, then falls back to
// a text-only one. Both draws exclude questions carrying an answer longer
// than a poll option allows.
func (s *QuizService) pickPollableQuestionID(ctx context.Context) (uuid.UUID, error) {
	for _, hasImage := range []bool{true, false} {
		ids, err := s.Q.RandomPollableQuestionIDs(ctx, sqlc.RandomPollableQuestionIDsParams{
			HasImage:     hasImage,
			MaxAnswerLen: pollOptionMaxChars,
			LimitCount:   1,
		})
		if err != nil {
			return uuid.Nil, err
		}
		if len(ids) > 0 {
			return ids[0], nil
		}
	}
	return uuid.Nil, nil
}
