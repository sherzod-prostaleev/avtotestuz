package bot

import (
	"context"
	"strings"
	"testing"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func answerRow(pos int16, correct bool, text string) sqlc.ListQuizAnswersRow {
	return sqlc.ListQuizAnswersRow{Position: pos, IsCorrect: correct, Text: text}
}

func TestBuildPollRequestMapsCorrectIndex(t *testing.T) {
	req, err := buildPollRequest(
		"Savol matni",
		[]sqlc.ListQuizAnswersRow{
			answerRow(1, false, "Birinchi"),
			answerRow(2, true, "Ikkinchi"),
			answerRow(3, false, "Uchinchi"),
		},
		"Izoh matni", 10, 55,
	)
	if err != nil {
		t.Fatalf("buildPollRequest: %v", err)
	}
	if req.CorrectIdx != 1 {
		t.Fatalf("CorrectIdx = %d, want 1", req.CorrectIdx)
	}
	if len(req.Options) != 3 || req.Options[1] != "Ikkinchi" {
		t.Fatalf("Options = %v", req.Options)
	}
	if req.OpenPeriod != 10 || req.ReplyTo != 55 {
		t.Fatalf("req = %+v", req)
	}
	if req.Explanation != "Izoh matni" {
		t.Fatalf("Explanation = %q", req.Explanation)
	}
}

// A question with no correct answer is corrupt data, not a poll.
func TestBuildPollRequestRejectsNoCorrectAnswer(t *testing.T) {
	_, err := buildPollRequest("Savol", []sqlc.ListQuizAnswersRow{
		answerRow(1, false, "Bir"), answerRow(2, false, "Ikki"),
	}, "", 10, 0)
	if err == nil {
		t.Fatal("want error when no answer is marked correct")
	}
}

// Long text must be rejected here rather than truncated: the corpus filter
// is supposed to have excluded it, so reaching this point means the filter
// leaked and the operator needs to know.
func TestBuildPollRequestRejectsOversizeOption(t *testing.T) {
	_, err := buildPollRequest("Savol", []sqlc.ListQuizAnswersRow{
		answerRow(1, true, strings.Repeat("x", 101)),
		answerRow(2, false, "Ikki"),
	}, "", 10, 0)
	if err == nil {
		t.Fatal("want error for a 101-char option")
	}
}

// The 300-char question limit is enforced too, even though today's corpus
// tops out at 222.
func TestBuildPollRequestRejectsOversizeQuestion(t *testing.T) {
	_, err := buildPollRequest(strings.Repeat("s", 301), []sqlc.ListQuizAnswersRow{
		answerRow(1, true, "Bir"), answerRow(2, false, "Ikki"),
	}, "", 10, 0)
	if err == nil {
		t.Fatal("want error for a 301-char question")
	}
}

// The corpus filter is the whole reason polls are viable — prove it excludes
// a question whose answer is too long, and keeps one whose answers fit.
func TestPickPollableQuestionSkipsLongAnswers(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	longID := seedQuizQuestionWithAnswers(t, pool, false,
		[]string{strings.Repeat("u", 140), "Qisqa"})
	okID := seedQuizQuestionWithAnswers(t, pool, false,
		[]string{"Qisqa bir", "Qisqa ikki"})

	svc := &QuizService{Q: q, Pool: pool}
	seen := map[string]bool{}
	for i := 0; i < 25; i++ {
		id, err := svc.pickPollableQuestionID(ctx)
		if err != nil {
			t.Fatalf("pickPollableQuestionID: %v", err)
		}
		seen[id.String()] = true
	}
	if seen[longID.String()] {
		t.Fatal("picked a question whose answer exceeds the 100-char poll limit")
	}
	if !seen[okID.String()] {
		t.Fatal("never picked the question that fits — filter is too aggressive")
	}
}

func TestQuizSecondsFallsBackWhenKeyMissing(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	svc := &QuizService{Q: sqlc.New(pool), Pool: pool}

	if _, err := pool.Exec(ctx,
		`DELETE FROM limit_config WHERE key = 'tg_quiz_seconds'`); err != nil {
		t.Fatal(err)
	}
	if got := svc.quizSeconds(ctx); got != defaultQuizSeconds {
		t.Fatalf("quizSeconds = %d, want fallback %d", got, defaultQuizSeconds)
	}
}

// An operator typing 2 into /settings/limits must not produce a poll
// Telegram will reject — the value is clamped to the API's 5..600 window.
func TestQuizSecondsClampsOutOfRangeConfig(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	svc := &QuizService{Q: sqlc.New(pool), Pool: pool}

	if _, err := pool.Exec(ctx,
		`INSERT INTO limit_config (key, free_value, vip_value) VALUES ('tg_quiz_seconds', 2, 2)
		 ON CONFLICT (key) DO UPDATE SET free_value = 2`); err != nil {
		t.Fatal(err)
	}
	if got := svc.quizSeconds(ctx); got != pollMinOpenPeriod {
		t.Fatalf("quizSeconds = %d, want clamp to %d", got, pollMinOpenPeriod)
	}

	if _, err := pool.Exec(ctx,
		`UPDATE limit_config SET free_value = 5000 WHERE key = 'tg_quiz_seconds'`); err != nil {
		t.Fatal(err)
	}
	if got := svc.quizSeconds(ctx); got != pollMaxOpenPeriod {
		t.Fatalf("quizSeconds = %d, want clamp to %d", got, pollMaxOpenPeriod)
	}
}

func TestQuizQuestionCountReadsConfig(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	svc := &QuizService{Q: sqlc.New(pool), Pool: pool}

	if _, err := pool.Exec(ctx,
		`INSERT INTO limit_config (key, free_value, vip_value) VALUES ('tg_quiz_questions', 7, 7)
		 ON CONFLICT (key) DO UPDATE SET free_value = 7`); err != nil {
		t.Fatal(err)
	}
	if got := svc.quizQuestionCount(ctx); got != 7 {
		t.Fatalf("quizQuestionCount = %d, want 7", got)
	}
}

// winnerStickerID is not wired into the game flow until a later task, but the
// accessor is part of this task's produced interface — cover it here so it
// is not dead code.
func TestWinnerStickerIDReturnsConfiguredValue(t *testing.T) {
	svc := &QuizService{}
	if got := svc.winnerStickerID(); got != "" {
		t.Fatalf("winnerStickerID = %q, want empty default", got)
	}
	svc.WinnerSticker = "CAACAgIAAxkBAAIC"
	if got := svc.winnerStickerID(); got != "CAACAgIAAxkBAAIC" {
		t.Fatalf("winnerStickerID = %q, want configured value", got)
	}
}
