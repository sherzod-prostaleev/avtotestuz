package bot

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
)

const (
	longPollTimeoutSec   = 30
	longPollErrorBackoff = 2 * time.Second
)

// updatesFetcher is the subset of *Client RunLongPoll needs — an interface
// so tests can drive the loop with a fake instead of a real HTTP server.
type updatesFetcher interface {
	GetUpdates(ctx context.Context, offset int64, timeoutSec int) ([]Update, error)
}

// RunLongPoll is the dev-mode alternative to a webhook: it repeatedly calls
// getUpdates and dispatches whatever comes back, advancing the offset past
// the highest update_id seen so Telegram doesn't redeliver it. Intended for
// local development behind NAT / without a public HTTPS URL — see design
// §5.1 for why this is rejected at config-validation time when ENV=prod.
func RunLongPoll(ctx context.Context, tg updatesFetcher, b *Bot, log *zap.Logger) {
	if log == nil {
		log = zap.NewNop()
	}
	var offset int64
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		updates, err := tg.GetUpdates(ctx, offset, longPollTimeoutSec)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			log.Warn("bot longpoll: getUpdates failed", zap.Error(err))
			select {
			case <-ctx.Done():
				return
			case <-time.After(longPollErrorBackoff):
			}
			continue
		}

		for _, u := range updates {
			if err := b.HandleUpdate(ctx, u); err != nil {
				log.Error("bot longpoll: dispatch failed", zap.Error(err), zap.Int64("update_id", u.UpdateID))
			}
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
		}
	}
}
