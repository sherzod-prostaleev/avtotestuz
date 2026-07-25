package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// FeatureFlagRow is a settings.flags list/detail row.
type FeatureFlagRow struct {
	Key         string          `json:"key"`
	Type        string          `json:"type"`
	Value       json.RawMessage `json:"value"`
	Description string          `json:"description"`
	UpdatedBy   string          `json:"updated_by"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// ListFeatureFlags returns all flags ordered by key.
func (s Store) ListFeatureFlags(ctx context.Context) ([]FeatureFlagRow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT key, type, value_json, description, updated_by, updated_at
		FROM feature_flag
		ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("list feature flags: %w", err)
	}
	defer rows.Close()
	out := make([]FeatureFlagRow, 0)
	for rows.Next() {
		var row FeatureFlagRow
		if err := rows.Scan(&row.Key, &row.Type, &row.Value, &row.Description, &row.UpdatedBy, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// GetFeatureFlag returns one flag by key.
func (s Store) GetFeatureFlag(ctx context.Context, key string) (FeatureFlagRow, error) {
	var row FeatureFlagRow
	err := s.Pool.QueryRow(ctx, `
		SELECT key, type, value_json, description, updated_by, updated_at
		FROM feature_flag WHERE key = $1`, key).Scan(
		&row.Key, &row.Type, &row.Value, &row.Description, &row.UpdatedBy, &row.UpdatedAt,
	)
	if err != nil {
		return row, err
	}
	return row, nil
}

// SetFeatureFlagValue updates value_json for an existing key after type validation.
func (s Store) SetFeatureFlagValue(ctx context.Context, key string, value json.RawMessage, updatedBy string) (FeatureFlagRow, FeatureFlagRow, error) {
	before, err := s.GetFeatureFlag(ctx, key)
	if err != nil {
		return FeatureFlagRow{}, FeatureFlagRow{}, err
	}
	if err := validateFlagValue(before.Type, value); err != nil {
		return before, FeatureFlagRow{}, err
	}
	if updatedBy == "" {
		updatedBy = "admin"
	}
	var after FeatureFlagRow
	err = s.Pool.QueryRow(ctx, `
		UPDATE feature_flag
		SET value_json = $2::jsonb, updated_by = $3, updated_at = now()
		WHERE key = $1
		RETURNING key, type, value_json, description, updated_by, updated_at`,
		key, value, updatedBy,
	).Scan(&after.Key, &after.Type, &after.Value, &after.Description, &after.UpdatedBy, &after.UpdatedAt)
	if err != nil {
		return before, FeatureFlagRow{}, fmt.Errorf("update feature flag: %w", err)
	}
	return before, after, nil
}

func validateFlagValue(typ string, raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("invalid value")
	}
	switch typ {
	case "boolean":
		var v bool
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("boolean flag requires JSON boolean")
		}
		return nil
	case "percentage":
		var v float64
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("percentage flag requires JSON number")
		}
		if v < 0 || v > 100 {
			return fmt.Errorf("percentage must be 0..100")
		}
		return nil
	case "allowlist":
		var v []string
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("allowlist flag requires JSON string array")
		}
		return nil
	default:
		return fmt.Errorf("unknown flag type")
	}
}
