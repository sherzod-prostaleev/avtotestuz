package session_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"avtotest.uz/backend/internal/db/sqlc"
)

// SubmitAnswer and FinishSession each open their transaction by locking the
// session row and then reading the same row back -- two round trips to one
// row, on the hottest endpoint the API has. GetExamSessionForUpdate is the
// single statement that replaces the pair.
//
// Both of its properties are asserted here because losing either is silent.
// A read that returned the columns without the lock would let two concurrent
// answers interleave past the duplicate-answer check; a lock that returned
// only the id would leave the caller reading a row it had not actually
// verified against the locked one.

func TestGetExamSessionForUpdateReturnsTheLockedRow(t *testing.T) {
	pool, q, svc, profileID := seedOrdered(t)
	ctx := context.Background()
	view := startVariantSession(t, q, svc, profileID)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row, err := sqlc.New(tx).GetExamSessionForUpdate(ctx, view.ID)
	if err != nil {
		t.Fatalf("GetExamSessionForUpdate: %v", err)
	}
	if row.ID != view.ID {
		t.Fatalf("id: got %v, want %v", row.ID, view.ID)
	}
	if row.ProfileID != profileID {
		t.Fatalf("profile_id: got %v, want %v", row.ProfileID, profileID)
	}
	if row.Mode != "variant" {
		t.Fatalf("mode: got %q, want %q", row.Mode, "variant")
	}
	if row.Status != "in_progress" {
		t.Fatalf("status: got %q, want %q", row.Status, "in_progress")
	}
	if int(row.Total) != view.Total {
		t.Fatalf("total: got %d, want %d", row.Total, view.Total)
	}
}

func TestGetExamSessionForUpdateBlocksAConcurrentLocker(t *testing.T) {
	pool, q, svc, profileID := seedOrdered(t)
	ctx := context.Background()
	view := startVariantSession(t, q, svc, profileID)

	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin holder: %v", err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	if _, err := sqlc.New(holder).GetExamSessionForUpdate(ctx, view.ID); err != nil {
		t.Fatalf("GetExamSessionForUpdate: %v", err)
	}

	// NOWAIT turns "would block" into an error instead of a hung test.
	rival, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin rival: %v", err)
	}
	defer func() { _ = rival.Rollback(ctx) }()
	var id any
	err = rival.QueryRow(ctx,
		`SELECT id FROM exam_session WHERE id = $1 FOR UPDATE NOWAIT`, view.ID).Scan(&id)
	if err == nil {
		t.Fatal("rival transaction locked the row while it was already locked")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
		t.Fatalf("want lock_not_available (55P03), got %v", err)
	}

	// Releasing the holder must release the row: a lock that outlived its
	// transaction would wedge the session for every later answer. The rival
	// is aborted by its own failed statement, so the retry needs a fresh
	// transaction rather than another command on that one.
	if err := rival.Rollback(ctx); err != nil {
		t.Fatalf("rollback rival: %v", err)
	}
	if err := holder.Rollback(ctx); err != nil {
		t.Fatalf("rollback holder: %v", err)
	}
	retry, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin retry: %v", err)
	}
	defer func() { _ = retry.Rollback(ctx) }()
	if err := retry.QueryRow(ctx,
		`SELECT id FROM exam_session WHERE id = $1 FOR UPDATE NOWAIT`, view.ID).Scan(&id); err != nil {
		t.Fatalf("row still locked after holder rolled back: %v", err)
	}
}
