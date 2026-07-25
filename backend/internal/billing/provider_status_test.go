package billing

import (
	"context"
	"errors"
	"testing"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestProviderKillSwitch(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	svc := Service{Q: sqlc.New(pool), Pool: pool}
	ctx := context.Background()

	list, err := svc.ListProviderStatuses(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 2 {
		t.Fatalf("providers = %d, want at least 2", len(list))
	}

	if _, err := svc.SetProviderEnabled(ctx, "payme", false, "test"); err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsureProviderEnabled(ctx, "payme"); !errors.Is(err, ErrProviderDisabled) {
		t.Fatalf("EnsureProviderEnabled = %v, want ErrProviderDisabled", err)
	}
	if err := svc.EnsureProviderEnabled(ctx, "click"); err != nil {
		t.Fatalf("click should stay enabled: %v", err)
	}

	if _, err := svc.SetProviderEnabled(ctx, "payme", true, "test"); err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsureProviderEnabled(ctx, "payme"); err != nil {
		t.Fatalf("payme re-enabled: %v", err)
	}
}
