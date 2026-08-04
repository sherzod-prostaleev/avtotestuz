package stationctx_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/stationctx"
)

func TestRoundTrip(t *testing.T) {
	id := uuid.New()
	ctx := stationctx.WithContext(context.Background(), id)
	got, ok := stationctx.FromContext(ctx)
	if !ok || got != id {
		t.Fatalf("got=%v ok=%v want=%v", got, ok, id)
	}
}

func TestEmptyContext(t *testing.T) {
	if _, ok := stationctx.FromContext(context.Background()); ok {
		t.Fatal("bare context must carry no station")
	}
}

func TestNilUUIDIsNotStored(t *testing.T) {
	ctx := stationctx.WithContext(context.Background(), uuid.Nil)
	if _, ok := stationctx.FromContext(ctx); ok {
		t.Fatal("uuid.Nil must not register as a station")
	}
}
