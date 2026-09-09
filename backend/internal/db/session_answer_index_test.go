package db_test

import (
	"context"
	"strings"
	"testing"

	"avtotest.uz/backend/internal/testdb"
)

// The index behind CountPracticeAnswersToday is invisible in Go: nothing
// references it by name, so dropping it or losing migration 0072 breaks
// nothing that compiles and nothing that any other test asserts. What it
// breaks is the plan -- the query silently returns to scanning every answer
// ever recorded, which was a second and a half in production and grows with
// the table forever. This asserts the migrated schema still carries it, and
// carries it over the columns that make the index-only probe possible: a
// single-column (session_id) index would satisfy a name check while handing
// the planner back the scan.
func TestMigratedSchemaIndexesSessionAnswerBySessionAndAnsweredAt(t *testing.T) {
	pool := testdb.New(t)

	var def string
	err := pool.QueryRow(context.Background(),
		`SELECT indexdef FROM pg_indexes
		 WHERE tablename = 'session_answer'
		   AND indexname = 'session_answer_session_answered_idx'`).Scan(&def)
	if err != nil {
		t.Fatalf("session_answer_session_answered_idx is missing from the migrated schema: %v", err)
	}
	if want := "(session_id, answered_at)"; !strings.Contains(def, want) {
		t.Fatalf("index is %q, want it keyed by %s", def, want)
	}
}
