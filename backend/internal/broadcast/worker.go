package broadcast

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// RunWorker polls the outbox until ctx is cancelled.
func RunWorker(ctx context.Context, svc *Service, log *zap.Logger) {
	if svc == nil || svc.Pool == nil {
		return
	}
	if log == nil {
		log = zap.NewNop()
	}
	t := time.NewTicker(workerPollInterval)
	defer t.Stop()
	log.Info("broadcast worker started")
	for {
		select {
		case <-ctx.Done():
			log.Info("broadcast worker stopped")
			return
		case <-t.C:
			if err := svc.ProcessOnce(ctx); err != nil && ctx.Err() == nil {
				log.Warn("broadcast worker tick", zap.Error(err))
			}
		}
	}
}
