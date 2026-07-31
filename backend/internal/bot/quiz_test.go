package bot

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func seedQuizQuestion(t *testing.T, pool *pgxpool.Pool, withImage bool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var catID, qID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO category (code, sort_order) VALUES ($1, 1) RETURNING id`,
		"tg_quiz_"+uuid.NewString()[:8],
	).Scan(&catID); err != nil {
		t.Fatal(err)
	}
	var imageID any
	if withImage {
		var img uuid.UUID
		key := "q/" + uuid.NewString() + ".png"
		if err := pool.QueryRow(ctx,
			`INSERT INTO image (storage_key, sha256, width, height, mime)
			 VALUES ($1, $2, 100, 100, 'image/png') RETURNING id`,
			key, uuid.NewString(),
		).Scan(&img); err != nil {
			t.Fatal(err)
		}
		imageID = img
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO question (source_ext_id, category_id, content_hash, image_id)
		VALUES ($1, $2, $3, $4) RETURNING id`,
		"tg-"+uuid.NewString(), catID, uuid.NewString(), imageID,
	).Scan(&qID); err != nil {
		t.Fatal(err)
	}
	var correctID uuid.UUID
	for i := 0; i < 3; i++ {
		var aID uuid.UUID
		if err := pool.QueryRow(ctx,
			`INSERT INTO answer (question_id, position, is_correct) VALUES ($1,$2,$3) RETURNING id`,
			qID, i+1, i == 1,
		).Scan(&aID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO answer_translation (answer_id, locale, text, status) VALUES ($1,'uz-Latn',$2,'verified')`,
			aID, fmt.Sprintf("Variant %c", 'A'+i),
		); err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			correctID = aID
		}
	}
	if _, err := pool.Exec(ctx,
		`UPDATE question SET correct_answer_id=$2 WHERE id=$1`, qID, correctID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO question_translation (question_id, locale, text, status, source)
		 VALUES ($1,'uz-Latn','Yo''l belgisi nimani anglatadi?','verified','')`, qID); err != nil {
		t.Fatal(err)
	}
	return qID
}

// seedQuizQuestionWithAnswers seeds a question whose answer texts are given
// explicitly, so tests can exercise the poll length filter.
func seedQuizQuestionWithAnswers(t *testing.T, pool *pgxpool.Pool, withImage bool, texts []string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var catID, qID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO category (code, sort_order) VALUES ($1, 1) RETURNING id`,
		"tg_poll_"+uuid.NewString()[:8],
	).Scan(&catID); err != nil {
		t.Fatal(err)
	}
	var imageID any
	if withImage {
		var img uuid.UUID
		if err := pool.QueryRow(ctx,
			`INSERT INTO image (storage_key, sha256, width, height, mime)
			 VALUES ($1, $2, 100, 100, 'image/png') RETURNING id`,
			"q/"+uuid.NewString()+".png", uuid.NewString(),
		).Scan(&img); err != nil {
			t.Fatal(err)
		}
		imageID = img
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO question (source_ext_id, category_id, content_hash, image_id)
		VALUES ($1, $2, $3, $4) RETURNING id`,
		"tgp-"+uuid.NewString(), catID, uuid.NewString(), imageID,
	).Scan(&qID); err != nil {
		t.Fatal(err)
	}
	var correctID uuid.UUID
	for i, text := range texts {
		var aID uuid.UUID
		if err := pool.QueryRow(ctx,
			`INSERT INTO answer (question_id, position, is_correct) VALUES ($1,$2,$3) RETURNING id`,
			qID, i+1, i == 0,
		).Scan(&aID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO answer_translation (answer_id, locale, text, status)
			 VALUES ($1,'uz-Latn',$2,'verified')`, aID, text); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			correctID = aID
		}
	}
	if _, err := pool.Exec(ctx,
		`UPDATE question SET correct_answer_id=$2 WHERE id=$1`, qID, correctID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO question_translation (question_id, locale, text, status, source)
		 VALUES ($1,'uz-Latn','Poll savoli?','verified','')`, qID); err != nil {
		t.Fatal(err)
	}
	return qID
}

func TestQuizStartSendsQuestionWithAnswers(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, true, []string{"To'g'ri", "Xato"})

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client,
		MediaBaseURL: "http://media.test", PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -5001, 11, "supergroup"); err != nil {
		t.Fatalf("StartGame: %v", err)
	}
	polls := rec.methodCalls("sendPoll")
	if len(polls) != 1 {
		t.Fatalf("want 1 poll, got %d", len(polls))
	}
	session, err := q.GetActiveQuizSessionByChat(ctx, -5001)
	if err != nil {
		t.Fatal(err)
	}
	if session.QuestionNo != 1 {
		t.Fatalf("QuestionNo = %d, want 1", session.QuestionNo)
	}
}

func TestQuizStartOrNextDeliversAfterMinInterval(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	_ = seedQuizQuestion(t, pool, false)
	_ = seedQuizQuestion(t, pool, false)
	fake, client := newFakeTelegram(t)
	svc := &QuizService{
		Q: q, Pool: pool, TG: client,
		MediaBaseURL: "http://media.test", PublicBaseURL: "http://app.test",
	}
	ctx := context.Background()

	prev := quizMinInterval
	quizMinInterval = 40 * time.Millisecond
	t.Cleanup(func() { quizMinInterval = prev })

	if err := svc.StartOrNext(ctx, -5004, 14); err != nil {
		t.Fatal(err)
	}
	session, err := q.GetActiveQuizSessionByChat(ctx, -5004)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate the question having been answered (the poll flow now clears
	// awaiting_answer via a poll_answer, not a callback tap) so the throttle
	// gate below is free to deliver the next question.
	if _, err := q.MarkQuizSessionAnswered(ctx, sqlc.MarkQuizSessionAnsweredParams{
		CorrectDelta: 0, ID: session.ID,
	}); err != nil {
		t.Fatal(err)
	}
	before := len(fake.allMessages())
	if err := svc.StartOrNext(ctx, -5004, 14); err != nil {
		t.Fatalf("StartOrNext after answer: %v", err)
	}
	msgs := fake.allMessages()
	if len(msgs) < before+2 {
		t.Fatalf("expected wait ack + next question, got %d new msgs from %v", len(msgs)-before, msgs[before:])
	}
	if !strings.Contains(msgs[before], "Biroz kuting") {
		t.Fatalf("expected wait ack, got %q", msgs[before])
	}
	session, err = q.GetActiveQuizSessionByChat(ctx, -5004)
	if err != nil {
		t.Fatal(err)
	}
	if !session.AwaitingAnswer || session.AskedCount != 2 {
		t.Fatalf("expected second question armed, session=%+v", session)
	}
}

func TestQuizStopSummarizes(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	_ = seedQuizQuestion(t, pool, false)
	fake, client := newFakeTelegram(t)
	svc := &QuizService{
		Q: q, Pool: pool, TG: client,
		MediaBaseURL: "http://media.test", PublicBaseURL: "http://app.test",
	}
	ctx := context.Background()
	if err := svc.StartOrNext(ctx, -5003, 13); err != nil {
		t.Fatal(err)
	}
	if err := svc.Stop(ctx, -5003); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fake.lastMessage(), "to'xtatildi") {
		t.Fatalf("reply = %q", fake.lastMessage())
	}
	if _, err := q.GetActiveQuizSessionByChat(ctx, -5003); err == nil {
		t.Fatal("expected no active session")
	}
}

func TestTruncateRunes(t *testing.T) {
	got := truncateRunes("abcdef", 4)
	if got != "abc…" {
		t.Fatalf("got %q", got)
	}
}
