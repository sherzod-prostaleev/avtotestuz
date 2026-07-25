package ops

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"avtotest.uz/backend/internal/httpx"
)

// LimitRow is a read-only limit_config snapshot for ops (M3 precursor).
// Values drive free vs VIP gates; editing stays for full Admin.
type LimitRow struct {
	Key       string    `json:"key"`
	FreeValue int32     `json:"free_value"`
	VipValue  int32     `json:"vip_value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListLimits returns all limit_config rows ordered by key.
func (h *Handler) ListLimits(ctx context.Context) ([]LimitRow, error) {
	if h.Pool == nil {
		return nil, fmt.Errorf("ops limits requires pool")
	}
	rows, err := h.Pool.Query(ctx, `
		SELECT key, free_value, vip_value, updated_at
		FROM limit_config
		ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LimitRow
	for rows.Next() {
		var row LimitRow
		if err := rows.Scan(&row.Key, &row.FreeValue, &row.VipValue, &row.UpdatedAt); err != nil {
			return nil, err
		}
		row.UpdatedAt = row.UpdatedAt.UTC()
		out = append(out, row)
	}
	return out, rows.Err()
}

func (h *Handler) listLimits(w http.ResponseWriter, r *http.Request) {
	out, err := h.ListLimits(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "limits query failed")
		return
	}
	if out == nil {
		out = []LimitRow{}
	}
	httpx.Data(w, http.StatusOK, out)
}
