package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/session"
)

// An exam-like session whose clock has run out can never be resumed: the next
// answer submitted to it is refused and finishes it instead. Until something
// finishes it, though, it stays 'in_progress' forever -- it shows in the
// learner's history as an exam still running, and it keeps its rows in every
// table that grows with the session count.
//
// ExpireTimedOutSessions is what finishes them. The tests that matter most
// here are the ones about what it must NOT touch: practice, variant, review
// and mistakes sessions are left open on purpose, because reopening them at
// the question the class stopped at is a feature, and an expiry sweep that
// closed those would silently delete that.

func startExam(t *testing.T, q *sqlc.Queries, svc *session.Service, profileID uuid.UUID) session.SessionView {
	t.Helper()
	grantVIP(t, q, profileID)
	view, err := svc.StartSession(context.Background(), profileID, session.StartRequest{
		Mode: "exam", Locale: "uz-Latn",
	})
	if err != nil {
		t.Fatalf("StartSession exam: %v", err)
	}
	return view
}

func statusOf(t *testing.T, q *sqlc.Queries, id uuid.UUID) sqlc.ExamSession {
	t.Helper()
	row, err := q.GetExamSession(context.Background(), id)
	if err != nil {
		t.Fatalf("GetExamSession: %v", err)
	}
	return row
}

func TestExpireFinishesAnExamWhoseTimeRanOut(t *testing.T) {
	_, q, svc, profileID := seedOrdered(t)
	ctx := context.Background()
	view := startExam(t, q, svc, profileID)

	svc.Now = func() time.Time {
		return view.StartedAt.Add(time.Duration(session.ExamTimeLimitSec)*time.Second + time.Hour)
	}
	n, err := svc.ExpireTimedOutSessions(ctx, 15*time.Minute, 100)
	if err != nil {
		t.Fatalf("ExpireTimedOutSessions: %v", err)
	}
	if n != 1 {
		t.Fatalf("finished %d sessions, want 1", n)
	}

	row := statusOf(t, q, view.ID)
	if row.Status != "failed" {
		t.Fatalf("status=%q want failed (no answers, out of time)", row.Status)
	}
	if row.StoppedReason.String != "time_up" {
		t.Fatalf("stopped_reason=%q want time_up", row.StoppedReason.String)
	}
	if !row.FinishedAt.Valid {
		t.Fatal("finished_at was left null")
	}
	if !row.Score.Valid || row.Score.Int32 != 0 {
		t.Fatalf("score=%+v want 0", row.Score)
	}
}

func TestExpireLeavesAnExamStillInsideItsTimeLimit(t *testing.T) {
	_, q, svc, profileID := seedOrdered(t)
	ctx := context.Background()
	view := startExam(t, q, svc, profileID)

	svc.Now = func() time.Time { return view.StartedAt.Add(time.Minute) }
	n, err := svc.ExpireTimedOutSessions(ctx, 15*time.Minute, 100)
	if err != nil {
		t.Fatalf("ExpireTimedOutSessions: %v", err)
	}
	if n != 0 {
		t.Fatalf("finished %d live sessions, want 0", n)
	}
	if row := statusOf(t, q, view.ID); row.Status != "in_progress" {
		t.Fatalf("status=%q want in_progress", row.Status)
	}
}

// The one that guards the resume feature.
func TestExpireNeverTouchesASessionWithoutATimeLimit(t *testing.T) {
	_, q, svc, profileID := seedOrdered(t)
	ctx := context.Background()
	variant := startVariantSession(t, q, svc, profileID)
	categoryID := categoryIDByCode(t, q, "signs")
	practice, err := svc.StartSession(ctx, profileID, session.StartRequest{
		Mode: "practice", CategoryID: categoryID, Locale: "uz-Latn", Count: 5,
	})
	if err != nil {
		t.Fatalf("StartSession practice: %v", err)
	}

	// A year later, and with no grace at all, they are still nobody's to close.
	svc.Now = func() time.Time { return variant.StartedAt.Add(365 * 24 * time.Hour) }
	n, err := svc.ExpireTimedOutSessions(ctx, 0, 100)
	if err != nil {
		t.Fatalf("ExpireTimedOutSessions: %v", err)
	}
	if n != 0 {
		t.Fatalf("finished %d untimed sessions, want 0", n)
	}
	for name, id := range map[string]uuid.UUID{"variant": variant.ID, "practice": practice.ID} {
		if row := statusOf(t, q, id); row.Status != "in_progress" {
			t.Fatalf("%s session status=%q want in_progress", name, row.Status)
		}
	}
}

func TestExpireWaitsOutTheGracePeriod(t *testing.T) {
	_, q, svc, profileID := seedOrdered(t)
	ctx := context.Background()
	view := startExam(t, q, svc, profileID)

	// One minute past the limit: expired, but inside a 15-minute grace.
	svc.Now = func() time.Time {
		return view.StartedAt.Add(time.Duration(session.ExamTimeLimitSec)*time.Second + time.Minute)
	}
	n, err := svc.ExpireTimedOutSessions(ctx, 15*time.Minute, 100)
	if err != nil {
		t.Fatalf("ExpireTimedOutSessions: %v", err)
	}
	if n != 0 {
		t.Fatalf("finished %d sessions inside the grace window, want 0", n)
	}
	if row := statusOf(t, q, view.ID); row.Status != "in_progress" {
		t.Fatalf("status=%q want in_progress", row.Status)
	}

	n, err = svc.ExpireTimedOutSessions(ctx, 0, 100)
	if err != nil {
		t.Fatalf("ExpireTimedOutSessions without grace: %v", err)
	}
	if n != 1 {
		t.Fatalf("finished %d sessions without grace, want 1", n)
	}
}

func TestExpireIsIdempotent(t *testing.T) {
	_, q, svc, profileID := seedOrdered(t)
	ctx := context.Background()
	view := startExam(t, q, svc, profileID)
	svc.Now = func() time.Time {
		return view.StartedAt.Add(time.Duration(session.ExamTimeLimitSec)*time.Second + time.Hour)
	}

	if _, err := svc.ExpireTimedOutSessions(ctx, 0, 100); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	before := statusOf(t, q, view.ID)

	n, err := svc.ExpireTimedOutSessions(ctx, 0, 100)
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if n != 0 {
		t.Fatalf("second sweep finished %d sessions, want 0", n)
	}
	after := statusOf(t, q, view.ID)
	if after.FinishedAt.Time != before.FinishedAt.Time || after.Status != before.Status {
		t.Fatalf("second sweep rewrote the session: %+v -> %+v", before, after)
	}
}

func TestExpireStopsAtTheLimitItWasGiven(t *testing.T) {
	_, q, svc, profileID := seedOrdered(t)
	ctx := context.Background()
	first := startExam(t, q, svc, profileID)
	for i := 0; i < 2; i++ {
		if _, err := svc.StartSession(ctx, profileID, session.StartRequest{
			Mode: "exam", Locale: "uz-Latn",
		}); err != nil {
			t.Fatalf("StartSession exam %d: %v", i, err)
		}
	}
	svc.Now = func() time.Time {
		return first.StartedAt.Add(time.Duration(session.ExamTimeLimitSec)*time.Second + time.Hour)
	}

	n, err := svc.ExpireTimedOutSessions(ctx, 0, 2)
	if err != nil {
		t.Fatalf("bounded sweep: %v", err)
	}
	if n != 2 {
		t.Fatalf("bounded sweep finished %d sessions, want 2", n)
	}
	n, err = svc.ExpireTimedOutSessions(ctx, 0, 2)
	if err != nil {
		t.Fatalf("follow-up sweep: %v", err)
	}
	if n != 1 {
		t.Fatalf("follow-up sweep finished %d sessions, want the remaining 1", n)
	}
}
