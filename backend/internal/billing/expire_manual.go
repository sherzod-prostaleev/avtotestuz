package billing

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// ExpireStaleManualPayments cancels unpaid Humo/manual checkouts older than
// ManualUnpaidTTL. Rows are kept; paid / Payme / Click payments are untouched.
func (s Service) ExpireStaleManualPayments(ctx context.Context) (int, error) {
	if s.Q == nil {
		return 0, nil
	}
	ids, err := s.Q.ExpireStaleManualPayments(ctx)
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}

// RunManualExpireWorker sweeps abandoned Humo checkouts until ctx is cancelled.
func RunManualExpireWorker(ctx context.Context, svc Service, log *zap.Logger) {
	if svc.Q == nil {
		return
	}
	if log == nil {
		log = zap.NewNop()
	}
	run := func() {
		n, err := svc.ExpireStaleManualPayments(ctx)
		if err != nil && ctx.Err() == nil {
			log.Warn("manual unpaid expire", zap.Error(err))
			return
		}
		if n > 0 {
			log.Info("manual unpaid expired", zap.Int("count", n), zap.Duration("ttl", ManualUnpaidTTL))
		}
	}
	run()
	t := time.NewTicker(expireWorkerInterval)
	defer t.Stop()
	log.Info("manual unpaid expire worker started", zap.Duration("ttl", ManualUnpaidTTL), zap.Duration("interval", expireWorkerInterval))
	for {
		select {
		case <-ctx.Done():
			log.Info("manual unpaid expire worker stopped")
			return
		case <-t.C:
			run()
		}
	}
}
