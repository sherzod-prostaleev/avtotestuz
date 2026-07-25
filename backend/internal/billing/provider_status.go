package billing

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrProviderDisabled means new checkouts for this provider are turned off
// via payment_provider_status (ops kill-switch). In-flight webhooks are
// unaffected.
var ErrProviderDisabled = errors.New("payment provider temporarily disabled")

// ProviderStatus is the public shape for GET /billing/providers.
type ProviderStatus struct {
	Provider string `json:"provider"`
	Enabled  bool   `json:"enabled"`
}

// ListProviderStatuses returns Payme/Click enablement. Missing rows (pre-migration
// race) are treated as enabled so a half-applied deploy does not hard-stop sales.
func (s Service) ListProviderStatuses(ctx context.Context) ([]ProviderStatus, error) {
	if s.Pool == nil {
		return []ProviderStatus{
			{Provider: "payme", Enabled: true},
			{Provider: "click", Enabled: true},
		}, nil
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT provider, enabled
		FROM payment_provider_status
		WHERE provider IN ('payme', 'click')
		ORDER BY provider`)
	if err != nil {
		return nil, fmt.Errorf("list payment providers: %w", err)
	}
	defer rows.Close()

	out := make([]ProviderStatus, 0, 2)
	seen := map[string]bool{}
	for rows.Next() {
		var p ProviderStatus
		if err := rows.Scan(&p.Provider, &p.Enabled); err != nil {
			return nil, fmt.Errorf("scan payment provider: %w", err)
		}
		out = append(out, p)
		seen[p.Provider] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, name := range []string{"payme", "click"} {
		if !seen[name] {
			out = append(out, ProviderStatus{Provider: name, Enabled: true})
		}
	}
	return out, nil
}

// EnsureProviderEnabled fails with ErrProviderDisabled when the kill-switch is off.
func (s Service) EnsureProviderEnabled(ctx context.Context, provider string) error {
	if provider != "payme" && provider != "click" {
		return nil
	}
	if s.Pool == nil {
		return nil
	}
	var enabled bool
	err := s.Pool.QueryRow(ctx, `
		SELECT enabled FROM payment_provider_status WHERE provider = $1`, provider).Scan(&enabled)
	if err != nil {
		// No row / table not ready → fail open for paid checkouts only after
		// migration; tests that truncate never wipe this config table.
		return nil
	}
	if !enabled {
		return ErrProviderDisabled
	}
	return nil
}

// SetProviderEnabled flips a kill-switch. updatedBy is an ops actor label (token
// fingerprint or "ops"), not a learner profile id.
func (s Service) SetProviderEnabled(ctx context.Context, provider string, enabled bool, updatedBy string) (ProviderStatus, error) {
	if provider != "payme" && provider != "click" {
		return ProviderStatus{}, fmt.Errorf("unknown provider %q", provider)
	}
	if s.Pool == nil {
		return ProviderStatus{}, fmt.Errorf("billing: SetProviderEnabled requires Pool")
	}
	if updatedBy == "" {
		updatedBy = "ops"
	}
	var out ProviderStatus
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO payment_provider_status (provider, enabled, updated_at, updated_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (provider) DO UPDATE
		  SET enabled = EXCLUDED.enabled,
		      updated_at = EXCLUDED.updated_at,
		      updated_by = EXCLUDED.updated_by
		RETURNING provider, enabled`,
		provider, enabled, time.Now().UTC(), updatedBy,
	).Scan(&out.Provider, &out.Enabled)
	if err != nil {
		return ProviderStatus{}, fmt.Errorf("set payment provider: %w", err)
	}
	return out, nil
}
