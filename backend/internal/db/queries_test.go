package db_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %s: %v", sql, err)
	}
}

func TestContentReadQueries(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	q := sqlc.New(pool)

	// seed: category + question(4 answers) + variant + translations
	var catID, qID, vID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO category (code, sort_order) VALUES ('signs', 1) RETURNING id`).Scan(&catID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, pool, `INSERT INTO category_translation VALUES ($1,'uz-Latn','Belgilar','verified'),($1,'ru','Знаки','verified')`, catID)
	if err := pool.QueryRow(ctx,
		`INSERT INTO question (source_ext_id, category_id, content_hash) VALUES ('q-1',$1,'h1') RETURNING id`, catID).Scan(&qID); err != nil {
		t.Fatal(err)
	}
	answerIDs := make([]uuid.UUID, 4)
	for i := 0; i < 4; i++ {
		if err := pool.QueryRow(ctx,
			`INSERT INTO answer (question_id, position, is_correct) VALUES ($1,$2,$3) RETURNING id`,
			qID, i+1, i == 1).Scan(&answerIDs[i]); err != nil {
			t.Fatal(err)
		}
		mustExec(t, pool, `INSERT INTO answer_translation VALUES ($1,'uz-Latn',$2,'verified')`, answerIDs[i], "Javob")
	}
	mustExec(t, pool, `UPDATE question SET correct_answer_id=$2 WHERE id=$1`, qID, answerIDs[1])
	mustExec(t, pool, `INSERT INTO question_translation VALUES ($1,'uz-Latn','Savol matni','verified','')`, qID)
	if err := pool.QueryRow(ctx, `INSERT INTO variant (number) VALUES (1) RETURNING id`).Scan(&vID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, pool, `INSERT INTO variant_question VALUES ($1,$2,1)`, vID, qID)

	cats, err := q.ListCategories(ctx, "ru")
	if err != nil || len(cats) != 1 || cats[0].Name != "Знаки" || cats[0].FallbackUsed {
		t.Fatalf("cats=%+v err=%v", cats, err)
	}
	catsKaa, err := q.ListCategories(ctx, "kaa") // fallback to uz-Latn
	if err != nil || catsKaa[0].Name != "Belgilar" || !catsKaa[0].FallbackUsed {
		t.Fatalf("kaa fallback: %+v err=%v", catsKaa, err)
	}
	vars, err := q.ListVariants(ctx)
	if err != nil || len(vars) != 1 || vars[0].Number != 1 || vars[0].QuestionCount != 1 {
		t.Fatalf("vars=%+v err=%v", vars, err)
	}
	vqs, err := q.ListVariantQuestions(ctx, sqlc.ListVariantQuestionsParams{VariantID: vID, Locale: "uz-Latn"})
	if err != nil || len(vqs) != 1 || vqs[0].Text != "Savol matni" {
		t.Fatalf("vqs=%+v err=%v", vqs, err)
	}
	ans, err := q.ListAnswersByQuestionIDs(ctx, sqlc.ListAnswersByQuestionIDsParams{
		QuestionIds: []uuid.UUID{qID}, Locale: "uz-Latn"})
	if err != nil || len(ans) != 4 {
		t.Fatalf("ans=%d err=%v", len(ans), err)
	}
}
