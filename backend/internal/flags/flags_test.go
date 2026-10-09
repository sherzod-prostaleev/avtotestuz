package flags

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"avtotest.uz/backend/internal/testdb"
)

func TestBoolAndPublic(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)

	ctx := context.Background()
	v, err := Bool(ctx, pool, KeyArenaEnabled, true)
	if err != nil || !v {
		t.Fatalf("seeded arena_enabled want true, got %v err=%v", v, err)
	}

	if _, err := pool.Exec(ctx, `
		UPDATE feature_flag SET value_json = 'false'::jsonb WHERE key = $1`, KeyArenaEnabled); err != nil {
		t.Fatal(err)
	}
	v, err = Bool(ctx, pool, KeyArenaEnabled, true)
	if err != nil || v {
		t.Fatalf("arena_enabled want false, got %v err=%v", v, err)
	}

	snap, err := Public(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if snap.ArenaEnabled {
		t.Fatal("public snapshot should reflect false arena")
	}

	_, _ = pool.Exec(ctx, `UPDATE feature_flag SET value_json = 'true'::jsonb WHERE key = $1`, KeyArenaEnabled)
}

func TestPublicHandler(t *testing.T) {
	pool := testdb.New(t)
	h := &Handler{Pool: pool}
	r := chi.NewRouter()
	r.Route("/api/v1", h.PublicRoutes)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/flags", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var env struct {
		Data PublicSnapshot `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.Data.ArenaEnabled {
		t.Fatal("expected seeded arena_enabled true")
	} // telegram_login (migration 0081) is seeded ON and is part of the public
	// snapshot: the website hides «Telegram orqali kirish» when it is off.
	if !env.Data.TelegramLogin || !strings.Contains(w.Body.String(), `"telegram_login":true`) {
		t.Fatalf("expected seeded telegram_login true in %s", w.Body.String())
	}
	if _, err := pool.Exec(context.Background(), `UPDATE feature_flag SET value_json = 'false'::jsonb WHERE key = $1`, KeyTelegramLogin); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `UPDATE feature_flag SET value_json = 'true'::jsonb WHERE key = $1`, KeyTelegramLogin)
	})
	snap, err := Public(context.Background(), pool)
	if err != nil || snap.TelegramLogin {
		t.Fatalf("public snapshot must follow the switch: %+v %v", snap, err)
	}
}
