package bot

import (
	"context"
	"fmt"
	"strings"
	"testing"

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

func TestQuizStartSendsQuestionWithAnswers(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	_ = seedQuizQuestion(t, pool, true)
	fake, client := newFakeTelegram(t)
	svc := &QuizService{
		Q: q, Pool: pool, TG: client,
		MediaBaseURL: "http://media.test", PublicBaseURL: "http://app.test",
	}

	if err := svc.StartOrNext(context.Background(), -5001, 11); err != nil {
		t.Fatalf("StartOrNext: %v", err)
	}
	msg := fake.lastMessage()
	if !strings.Contains(msg, "Yo'l belgisi") && !strings.Contains(msg, "Yo''l") {
		// caption may be the question text
		if !strings.Contains(msg, "belgisi") {
			t.Fatalf("unexpected question payload: %q", msg)
		}
	}
	session, err := q.GetActiveQuizSessionByChat(context.Background(), -5001)
	if err != nil {
		t.Fatal(err)
	}
	if !session.AwaitingAnswer || session.AskedCount != 1 {
		t.Fatalf("session = %+v", session)
	}
}

func TestQuizAnswerGradesAndIsIdempotent(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	_ = seedQuizQuestion(t, pool, false)
	fake, client := newFakeTelegram(t)
	svc := &QuizService{
		Q: q, Pool: pool, TG: client,
		MediaBaseURL: "http://media.test", PublicBaseURL: "http://app.test",
	}
	ctx := context.Background()
	if err := svc.StartOrNext(ctx, -5002, 12); err != nil {
		t.Fatal(err)
	}
	session, _ := q.GetActiveQuizSessionByChat(ctx, -5002)

	// Wrong answer index 0, correct is index 1.
	if err := svc.handleAnswer(ctx, -5002, session.AnswerMessageID, 0); err != nil {
		t.Fatalf("handleAnswer: %v", err)
	}
	if !strings.Contains(fake.lastMessage(), "Noto'g'ri") {
		t.Fatalf("reply = %q", fake.lastMessage())
	}
	session, _ = q.GetActiveQuizSessionByChat(ctx, -5002)
	if session.AwaitingAnswer {
		t.Fatal("expected awaiting_answer=false after grade")
	}
	if session.CorrectCount != 0 {
		t.Fatalf("correct_count=%d", session.CorrectCount)
	}

	// Double-tap must not bump counts again.
	if err := svc.handleAnswer(ctx, -5002, session.AnswerMessageID, 1); err != nil {
		t.Fatal(err)
	}
	session, _ = q.GetActiveQuizSessionByChat(ctx, -5002)
	if session.CorrectCount != 0 || session.AskedCount != 1 {
		t.Fatalf("double-tap mutated session: %+v", session)
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

func TestParseAnswerIndex(t *testing.T) {
	n, ok := parseAnswerIndex("a:2")
	if !ok || n != 2 {
		t.Fatalf("n=%d ok=%v", n, ok)
	}
	if _, ok := parseAnswerIndex("a:x"); ok {
		t.Fatal("expected false")
	}
}
