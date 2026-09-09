package learning_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/learning"
)

// calibrateBucketZero writes n finished exams whose stored readiness lands in
// bucket 0, which is the bucket a profile that has answered nothing falls in.
// Thirty is empiricalMinSamples, the point where estimatePass stops using the
// model and starts using measured rows -- so crossing it is a change Stats
// reports, and therefore a change a cache can be caught still hiding.
func calibrateBucketZero(t *testing.T, pool *pgxpool.Pool, profileID uuid.UUID, n int) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO exam_session (profile_id, mode, locale, total, status, readiness_pct_at_finish)
		SELECT $1, 'exam', 'uz-Latn', 20, 'passed', 0 FROM generate_series(1, $2)`,
		profileID, n)
	if err != nil {
		t.Fatalf("calibrate: %v", err)
	}
}

func TestStatsWithoutACacheSeesNewCalibrationImmediately(t *testing.T) {
	_, svc, profileID, _, pool := seedWithPool(t)
	ctx := context.Background()

	stats, err := svc.Stats(ctx, profileID)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.PassEstimate.Source != "model" {
		t.Fatalf("source=%q want model before calibration", stats.PassEstimate.Source)
	}

	calibrateBucketZero(t, pool, profileID, 40)

	stats, err = svc.Stats(ctx, profileID)
	if err != nil {
		t.Fatalf("Stats after calibration: %v", err)
	}
	if stats.PassEstimate.Source != "empirical" {
		t.Fatalf("source=%q want empirical: an uncached service must read live rows", stats.PassEstimate.Source)
	}
}

func TestStatsServesTheHistogramItAlreadyLoaded(t *testing.T) {
	q, svc, profileID, _, pool := seedWithPool(t)
	ctx := context.Background()
	svc.PassRates = learning.NewPassRateCache(time.Hour)

	stats, err := svc.Stats(ctx, profileID)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.PassEstimate.Source != "model" {
		t.Fatalf("source=%q want model before calibration", stats.PassEstimate.Source)
	}

	calibrateBucketZero(t, pool, profileID, 40)

	stats, err = svc.Stats(ctx, profileID)
	if err != nil {
		t.Fatalf("Stats after calibration: %v", err)
	}
	if stats.PassEstimate.Source != "model" {
		t.Fatalf("source=%q: the cached histogram was not used", stats.PassEstimate.Source)
	}

	// The rows really are there -- it is the cache holding the old answer, not
	// a calibration that failed to write.
	live := learning.NewService(q)
	fresh, err := live.Stats(ctx, profileID)
	if err != nil {
		t.Fatalf("Stats uncached: %v", err)
	}
	if fresh.PassEstimate.Source != "empirical" {
		t.Fatalf("uncached source=%q want empirical", fresh.PassEstimate.Source)
	}
}
