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

// UserRow is a PII-light directory row for ops (M3 precursor).
type UserRow struct {
	ID          uuid.UUID `json:"id"`
	PhoneMasked string    `json:"phone_masked"`
	Name        string    `json:"name"`
	LocalePref  string    `json:"locale_pref"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// ListUsers returns recent profiles, optionally filtered by phone substring / id prefix.
func (h *Handler) ListUsers(ctx context.Context, q string, limit int) ([]UserRow, error) {
	if h.Pool == nil {
		return nil, fmt.Errorf("ops users requires pool")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q = strings.TrimSpace(q)
	rows, err := h.Pool.Query(ctx, `
		SELECT id, phone, name, locale_pref, status, created_at
		FROM profile
		WHERE ($1 = '' OR phone ILIKE '%' || $1 || '%' OR id::text ILIKE $1 || '%')
		ORDER BY created_at DESC
		LIMIT $2`, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserRow
	for rows.Next() {
		var (
			id      uuid.UUID
			phone   string
			name    string
			locale  string
			status  string
			created time.Time
		)
		if err := rows.Scan(&id, &phone, &name, &locale, &status, &created); err != nil {
			return nil, err
		}
		out = append(out, UserRow{
			ID:          id,
			PhoneMasked: maskPhone(phone),
			Name:        name,
			LocalePref:  locale,
			Status:      status,
			CreatedAt:   created.UTC(),
		})
	}
	return out, rows.Err()
}

func maskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	n := len(phone)
	if n <= 4 {
		return "***"
	}
	if n <= 8 {
		return phone[:2] + "***" + phone[n-2:]
	}
	return phone[:4] + "***" + phone[n-4:]
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	q := r.URL.Query().Get("q")
	out, err := h.ListUsers(r.Context(), q, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "users query failed")
		return
	}
	if out == nil {
		out = []UserRow{}
	}
	httpx.Data(w, http.StatusOK, out)
}
