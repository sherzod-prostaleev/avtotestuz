package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// LimitConfigRow is a limit_config row for Admin Settings.
type LimitConfigRow struct {
	Key       string    `json:"key"`
	FreeValue int32     `json:"free_value"`
	VipValue  int32     `json:"vip_value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListLimitConfigs returns all limit_config rows.
func (s Store) ListLimitConfigs(ctx context.Context) ([]LimitConfigRow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT key, free_value, vip_value, updated_at
		FROM limit_config
		ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("list limit_config: %w", err)
	}
	defer rows.Close()
	out := make([]LimitConfigRow, 0)
	for rows.Next() {
		var row LimitConfigRow
		if err := rows.Scan(&row.Key, &row.FreeValue, &row.VipValue, &row.UpdatedAt); err != nil {
			return nil, err
		}
		row.UpdatedAt = row.UpdatedAt.UTC()
		out = append(out, row)
	}
	return out, rows.Err()
}

// GetLimitConfig returns one row by key.
func (s Store) GetLimitConfig(ctx context.Context, key string) (LimitConfigRow, error) {
	var row LimitConfigRow
	err := s.Pool.QueryRow(ctx, `
		SELECT key, free_value, vip_value, updated_at
		FROM limit_config WHERE key = $1`, key).Scan(
		&row.Key, &row.FreeValue, &row.VipValue, &row.UpdatedAt,
	)
	if err != nil {
		return row, err
	}
	row.UpdatedAt = row.UpdatedAt.UTC()
	return row, nil
}

// SetLimitConfigValues updates free/vip for an existing key.
func (s Store) SetLimitConfigValues(ctx context.Context, key string, free, vip int32, adminUserID uuid.UUID) (LimitConfigRow, LimitConfigRow, error) {
	before, err := s.GetLimitConfig(ctx, key)
	if err != nil {
		return LimitConfigRow{}, LimitConfigRow{}, err
	}
	var after LimitConfigRow
	err = s.Pool.QueryRow(ctx, `
		UPDATE limit_config
		SET free_value = $2, vip_value = $3, updated_at = now(), updated_by = $4
		WHERE key = $1
		RETURNING key, free_value, vip_value, updated_at`,
		key, free, vip, adminUserID,
	).Scan(&after.Key, &after.FreeValue, &after.VipValue, &after.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return before, LimitConfigRow{}, err
		}
		return before, LimitConfigRow{}, fmt.Errorf("update limit_config: %w", err)
	}
	after.UpdatedAt = after.UpdatedAt.UTC()
	return before, after, nil
}
