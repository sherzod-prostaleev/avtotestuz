package ops

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/httpx"
)

// PaymentRow is a billing ops list row (M3 precursor).
type PaymentRow struct {
	ID        uuid.UUID  `json:"id"`
	ProfileID uuid.UUID  `json:"profile_id"`
	Provider  string     `json:"provider"`
	Status    string     `json:"status"`
	AmountUzs int64      `json:"amount_uzs"`
	CreatedAt time.Time  `json:"created_at"`
	PaidAt    *time.Time `json:"paid_at,omitempty"`
}

// ListPayments returns recent payments newest-first.
func (h *Handler) ListPayments(ctx context.Context, status string, limit int) ([]PaymentRow, error) {
	if h.Pool == nil {
		return nil, fmt.Errorf("ops payments requires pool")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	status = strings.TrimSpace(strings.ToLower(status))
	rows, err := h.Pool.Query(ctx, `
		SELECT id, profile_id, provider, status, amount_uzs, created_at, paid_at
		FROM payment
		WHERE ($1 = '' OR status = $1)
		ORDER BY created_at DESC
		LIMIT $2`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PaymentRow
	for rows.Next() {
		var (
			p      PaymentRow
			paidAt *time.Time
		)
		if err := rows.Scan(&p.ID, &p.ProfileID, &p.Provider, &p.Status, &p.AmountUzs, &p.CreatedAt, &paidAt); err != nil {
			return nil, err
		}
		p.CreatedAt = p.CreatedAt.UTC()
		if paidAt != nil {
			t := paidAt.UTC()
			p.PaidAt = &t
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (h *Handler) listPayments(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	status := r.URL.Query().Get("status")
	out, err := h.ListPayments(r.Context(), status, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "payments query failed")
		return
	}
	if out == nil {
		out = []PaymentRow{}
	}
	httpx.Data(w, http.StatusOK, out)
}
