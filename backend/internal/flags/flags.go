// Package flags reads runtime feature_flag rows for product gates.
package flags

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	KeyMaintenanceMode   = "maintenance_mode"
	KeyArenaEnabled      = "arena_enabled"
	KeyWebPushDigest     = "web_push_digest"
	KeyCheckoutPayme     = "checkout_payme"
	KeyCheckoutClick     = "checkout_click"
	KeyTelegramQuiz      = "telegram_quiz"
	KeyTelegramDMDigest  = "telegram_dm_digest"
)

// Bool returns a boolean feature flag. Missing rows / wrong type → defaultVal
// (fail-open for known product surfaces unless callers choose otherwise).
func Bool(ctx context.Context, pool *pgxpool.Pool, key string, defaultVal bool) (bool, error) {
	if pool == nil {
		return defaultVal, nil
	}
	var typ string
	var raw []byte
	err := pool.QueryRow(ctx, `
		SELECT type, value_json FROM feature_flag WHERE key = $1`, key).Scan(&typ, &raw)
	if err != nil {
		if err == pgx.ErrNoRows {
			return defaultVal, nil
		}
		return defaultVal, fmt.Errorf("feature flag %s: %w", key, err)
	}
	if typ != "boolean" {
		return defaultVal, nil
	}
	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		return defaultVal, nil
	}
	return v, nil
}

// PublicSnapshot is the learner-safe flag subset (no secrets).
type PublicSnapshot struct {
	MaintenanceMode bool `json:"maintenance_mode"`
	ArenaEnabled    bool `json:"arena_enabled"`
	CheckoutPayme   bool `json:"checkout_payme"`
	CheckoutClick   bool `json:"checkout_click"`
}

// Public returns learner-visible boolean gates (defaults match seed).
func Public(ctx context.Context, pool *pgxpool.Pool) (PublicSnapshot, error) {
	out := PublicSnapshot{
		MaintenanceMode: false,
		ArenaEnabled:    true,
		CheckoutPayme:   true,
		CheckoutClick:   true,
	}
	var err error
	if out.MaintenanceMode, err = Bool(ctx, pool, KeyMaintenanceMode, false); err != nil {
		return out, err
	}
	if out.ArenaEnabled, err = Bool(ctx, pool, KeyArenaEnabled, true); err != nil {
		return out, err
	}
	if out.CheckoutPayme, err = Bool(ctx, pool, KeyCheckoutPayme, true); err != nil {
		return out, err
	}
	if out.CheckoutClick, err = Bool(ctx, pool, KeyCheckoutClick, true); err != nil {
		return out, err
	}
	return out, nil
}
