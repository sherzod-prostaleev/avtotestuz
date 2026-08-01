package push

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

const (
	KindBroadcast       = "support_broadcast"
	defaultBroadcastCap = 500
	broadcastWorkers    = 8
)

// BroadcastOpts controls an admin support web-push broadcast.
type BroadcastOpts struct {
	Title  string
	Body   string
	URL    string
	DryRun bool
	// Limit caps distinct subscribed profiles (default 500).
	Limit int
}

// BroadcastResult summarizes a broadcast attempt.
type BroadcastResult struct {
	Recipients int  `json:"recipients"`
	Notified   int  `json:"notified"`
	Deliveries int  `json:"deliveries"`
	Errors     int  `json:"errors"`
	DryRun     bool `json:"dry_run"`
}

// ListBroadcastRecipients returns distinct active profiles with ≥1 push subscription.
func (s *Service) ListBroadcastRecipients(ctx context.Context, limit int) ([]uuid.UUID, error) {
	if s.Pool == nil {
		return nil, fmt.Errorf("broadcast requires a pool")
	}
	if limit <= 0 {
		limit = defaultBroadcastCap
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT DISTINCT p.id
		FROM profile p
		JOIN push_subscription ps ON ps.profile_id = p.id
		WHERE p.status = 'active'
		ORDER BY p.id
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// BroadcastSupport sends (or dry-runs) a web-push to subscribed active profiles.
func (s *Service) BroadcastSupport(ctx context.Context, opts BroadcastOpts) (BroadcastResult, error) {
	if !s.Cfg.Configured() || s.Sender == nil {
		return BroadcastResult{}, ErrUnconfigured
	}
	ids, err := s.ListBroadcastRecipients(ctx, opts.Limit)
	if err != nil {
		return BroadcastResult{}, err
	}
	res := BroadcastResult{Recipients: len(ids), DryRun: opts.DryRun}
	if opts.DryRun {
		return res, nil
	}
	payload := NotifyPayload{
		Title: opts.Title,
		Body:  opts.Body,
		URL:   opts.URL,
		Data:  map[string]any{"kind": KindBroadcast},
	}
	type notifyResult struct {
		sent int
		err  error
	}
	workers := broadcastWorkers
	if len(ids) < workers {
		workers = len(ids)
	}
	jobs := make(chan uuid.UUID)
	results := make(chan notifyResult, len(ids))
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				sent, notifyErr := s.Notify(ctx, id, KindBroadcast, payload)
				results <- notifyResult{sent: sent, err: notifyErr}
			}
		}()
	}

sendLoop:
	for _, id := range ids {
		select {
		case jobs <- id:
		case <-ctx.Done():
			break sendLoop
		}
	}
	close(jobs)
	wg.Wait()
	close(results)

	for result := range results {
		if result.err != nil {
			if result.err != ErrNoSubs {
				res.Errors++
			}
			continue
		}
		res.Notified++
		res.Deliveries += result.sent
	}
	if err := ctx.Err(); err != nil {
		return res, err
	}
	return res, nil
}
