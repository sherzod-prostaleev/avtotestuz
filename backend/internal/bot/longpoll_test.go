package bot

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/progress"
	"avtotest.uz/backend/internal/testdb"
)

// fakeFetcher scripts a sequence of GetUpdates responses so the long-poll
// loop can be tested without a real Telegram API.
type fakeFetcher struct {
	mu        sync.Mutex
	responses []fetcherResponse
	calls     []int64 // offsets the loop asked for, in order
}

type fetcherResponse struct {
	updates []Update
	err     error
}

func (f *fakeFetcher) GetUpdates(_ context.Context, offset int64, _ int) ([]Update, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, offset)
	if len(f.responses) == 0 {
		return nil, errors.New("fakeFetcher exhausted")
	}
	r := f.responses[0]
	f.responses = f.responses[1:]
	return r.updates, r.err
}

func updateWithID(updateID int64, text string, tgUserID int64, username string) Update {
	u := update(text, tgUserID, username)
	u.UpdateID = updateID
	return u
}

func TestRunLongPoll_AdvancesOffsetAndDispatches(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	_, client := newFakeTelegram(t)
	b := &Bot{
		Link:     NewLinkService(pool, q),
		Billing:  billing.Service{Q: q},
		Progress: progress.NewService(q),
		TG:       client,
	}

	fetcher := &fakeFetcher{
		responses: []fetcherResponse{
			{updates: []Update{updateWithID(1, "/start", 1, "a")}},
			{updates: []Update{updateWithID(2, "/status", 2, "b")}},
			{err: errors.New("transient network error")},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		RunLongPoll(ctx, fetcher, b, nil)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunLongPoll did not return after context cancellation")
	}

	fetcher.mu.Lock()
	calls := append([]int64(nil), fetcher.calls...)
	fetcher.mu.Unlock()

	if len(calls) < 3 {
		t.Fatalf("calls = %v, want at least 3 (two updates + one error)", calls)
	}
	if calls[0] != 0 {
		t.Errorf("first offset = %d, want 0", calls[0])
	}
	if calls[1] != 2 {
		t.Errorf("second offset = %d, want 2 (after update_id=1)", calls[1])
	}
	if calls[2] != 3 {
		t.Errorf("third offset = %d, want 3 (after update_id=2)", calls[2])
	}
}

func TestRunLongPoll_StopsOnContextCancellation(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	_, client := newFakeTelegram(t)
	b := &Bot{Link: NewLinkService(pool, q), Billing: billing.Service{Q: q}, Progress: progress.NewService(q), TG: client}

	ctx, cancel := context.WithCancel(context.Background())
	fetcher := &fakeFetcher{responses: []fetcherResponse{{err: context.Canceled}}}
	cancel()

	done := make(chan struct{})
	go func() {
		RunLongPoll(ctx, fetcher, b, nil)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunLongPoll did not return promptly after cancellation")
	}
}
