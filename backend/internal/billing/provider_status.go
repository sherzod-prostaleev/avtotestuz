package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"avtotest.uz/backend/internal/flags"
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

var knownProviders = []string{"payme", "click", "manual"}

func isKnownProvider(provider string) bool {
	for _, p := range knownProviders {
		if p == provider {
			return true
		}
	}
	return false
}

func flagKeyForProvider(provider string) (string, bool) {
	switch provider {
	case "payme":
		return flags.KeyCheckoutPayme, true
	case "click":
		return flags.KeyCheckoutClick, true
	case "manual":
		return flags.KeyCheckoutManual, true
	default:
		return "", false
	}
}

// ListProviderStatuses returns Payme/Click/Manual enablement. Missing rows are
// treated as enabled. Feature flags AND with the kill-switch.
func (s Service) ListProviderStatuses(ctx context.Context) ([]ProviderStatus, error) {
	if s.Pool == nil {
		return []ProviderStatus{
			{Provider: "payme", Enabled: true},
			{Provider: "click", Enabled: true},
			{Provider: "manual", Enabled: true},
		}, nil
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT provider, enabled
		FROM payment_provider_status
		WHERE provider = ANY($1)
		ORDER BY provider`, knownProviders)
	if err != nil {
		return nil, fmt.Errorf("list payment providers: %w", err)
	}
	defer rows.Close()

	out := make([]ProviderStatus, 0, len(knownProviders))
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
	for _, name := range knownProviders {
		if !seen[name] {
			out = append(out, ProviderStatus{Provider: name, Enabled: true})
		}
	}

	paymeFlag, err := flags.Bool(ctx, s.Pool, flags.KeyCheckoutPayme, true)
	if err != nil {
		return nil, err
	}
	clickFlag, err := flags.Bool(ctx, s.Pool, flags.KeyCheckoutClick, true)
	if err != nil {
		return nil, err
	}
	manualFlag, err := flags.Bool(ctx, s.Pool, flags.KeyCheckoutManual, true)
	if err != nil {
		return nil, err
	}
	for i := range out {
		switch out[i].Provider {
		case "payme":
			out[i].Enabled = out[i].Enabled && paymeFlag
		case "click":
			out[i].Enabled = out[i].Enabled && clickFlag
		case "manual":
			out[i].Enabled = out[i].Enabled && manualFlag
		}
	}
	return out, nil
}

// EnsureProviderEnabled fails with ErrProviderDisabled when the kill-switch is off
// or the matching checkout_* feature flag is false.
func (s Service) EnsureProviderEnabled(ctx context.Context, provider string) error {
	if !isKnownProvider(provider) {
		return nil
	}
	if s.Pool == nil {
		return nil
	}
	flagKey, ok := flagKeyForProvider(provider)
	if !ok {
		return nil
	}
	flagOn, err := flags.Bool(ctx, s.Pool, flagKey, true)
	if err != nil {
		return err
	}
	if !flagOn {
		return ErrProviderDisabled
	}
	var enabled bool
	err = s.Pool.QueryRow(ctx, `
		SELECT enabled FROM payment_provider_status WHERE provider = $1`, provider).Scan(&enabled)
	if err != nil {
		return nil
	}
	if !enabled {
		return ErrProviderDisabled
	}
	return nil
}

// SetProviderEnabled flips a kill-switch. updatedBy is an ops actor label.
func (s Service) SetProviderEnabled(ctx context.Context, provider string, enabled bool, updatedBy string) (ProviderStatus, error) {
	if !isKnownProvider(provider) {
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
