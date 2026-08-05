// Package account exposes the authenticated user's own profile and
// entitlement status under /me.
package account

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/httpx"
	"avtotest.uz/backend/internal/i18n"
)

type Handler struct {
	Q       *sqlc.Queries
	Billing billing.Service
}

func (h *Handler) Routes(r chi.Router) {
	r.Get("/me", h.getMe)
	r.Patch("/me", h.patchMe)
	r.Get("/me/entitlement", h.getEntitlement)
	r.Get("/me/payments", h.listMyPayments)
}

type profileDTO struct {
	ID           string  `json:"id"`
	Phone        string  `json:"phone"`
	Name         string  `json:"name"`
	Region       string  `json:"region"`
	District     string  `json:"district"`
	BirthDate    *string `json:"birth_date"`
	LocalePref   string  `json:"locale_pref"`
	ThemePref    string  `json:"theme_pref"`
	ReferralCode string  `json:"referral_code"`
	Role         string  `json:"role"`
	Kind         string  `json:"kind"`
	CreatedAt    string  `json:"created_at"`
}

func toProfileDTO(p sqlc.Profile) profileDTO {
	var bd *string
	if p.BirthDate.Valid {
		s := p.BirthDate.Time.Format("2006-01-02")
		bd = &s
	}
	return profileDTO{
		ID:           p.ID.String(),
		Phone:        p.Phone,
		Name:         p.Name,
		Region:       p.Region,
		District:     p.District,
		BirthDate:    bd,
		LocalePref:   p.LocalePref,
		ThemePref:    p.ThemePref,
		ReferralCode: p.ReferralCode.String,
		Role:         p.Role,
		Kind:         p.Kind,
		CreatedAt:    p.CreatedAt.Time.Format(time.RFC3339),
	}
}

type vipDTO struct {
	Active    bool                   `json:"active"`
	Until     *string                `json:"until"`
	Proration *billing.ProrationInfo `json:"proration,omitempty"`
}

func toVIPDTO(active bool, until *time.Time) vipDTO {
	v := vipDTO{Active: active}
	if until != nil {
		s := until.Format(time.RFC3339)
		v.Until = &s
	}
	return v
}

func (h *Handler) getEntitlement(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	active, until, err := h.Billing.Status(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "entitlement query failed")
		return
	}
	out := toVIPDTO(active, until)
	if pr, err := h.Billing.LatestPurchaseProration(r.Context(), claims.ProfileID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "entitlement query failed")
		return
	} else if pr != nil {
		out.Proration = pr
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	profile, err := h.Q.GetProfileByID(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "profile query failed")
		return
	}
	active, until, err := h.Billing.Status(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "entitlement query failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"profile": toProfileDTO(profile),
		"vip":     toVIPDTO(active, until),
	})
}

type patchMeBody struct {
	Name       *string         `json:"name"`
	Region     *string         `json:"region"`
	District   *string         `json:"district"`
	BirthDate  json.RawMessage `json:"birth_date"`
	LocalePref *string         `json:"locale_pref"`
	ThemePref  *string         `json:"theme_pref"`
}

func (h *Handler) patchMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	var body patchMeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	current, err := h.Q.GetProfileByID(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "profile query failed")
		return
	}

	params := sqlc.UpdateProfileMeParams{
		ID:         claims.ProfileID,
		Name:       current.Name,
		Region:     current.Region,
		District:   current.District,
		BirthDate:  current.BirthDate,
		LocalePref: current.LocalePref,
		ThemePref:  current.ThemePref,
	}
	if body.Name != nil {
		params.Name = *body.Name
	}
	if body.Region != nil {
		params.Region = *body.Region
	}
	if body.District != nil {
		params.District = *body.District
	}
	if body.LocalePref != nil {
		params.LocalePref = *body.LocalePref
	}
	if body.ThemePref != nil {
		params.ThemePref = *body.ThemePref
	}
	if len(body.BirthDate) > 0 {
		if string(body.BirthDate) == "null" {
			params.BirthDate = pgtype.Date{}
		} else {
			var s string
			if err := json.Unmarshal(body.BirthDate, &s); err != nil {
				httpx.Error(w, http.StatusBadRequest, "invalid_birth_date", "birth_date must be YYYY-MM-DD or null")
				return
			}
			d, err := time.Parse("2006-01-02", s)
			if err != nil {
				httpx.Error(w, http.StatusBadRequest, "invalid_birth_date", "birth_date must be YYYY-MM-DD or null")
				return
			}
			params.BirthDate = pgtype.Date{Time: d, Valid: true}
		}
	}

	updated, err := h.Q.UpdateProfileMe(r.Context(), params)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "profile update failed")
		return
	}
	httpx.Data(w, http.StatusOK, toProfileDTO(updated))
}

type paymentHistoryDTO struct {
	ID         string     `json:"id"`
	TariffCode string     `json:"tariff_code"`
	TariffName string     `json:"tariff_name"`
	TariffDays int        `json:"tariff_days"`
	AmountUzs  int64      `json:"amount_uzs"`
	Provider   string     `json:"provider"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	PaidAt     *time.Time `json:"paid_at"`
}

func (h *Handler) listMyPayments(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	loc, ok := i18n.Parse(r)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_locale", "locale must be one of uz-Latn, uz-Cyrl, ru, kaa")
		return
	}
	limit := 20
	if s := r.URL.Query().Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid_request", "limit must be an integer")
			return
		}
		if n > 0 {
			limit = n
		}
	}
	rows, err := h.Q.ListMyPayments(r.Context(), sqlc.ListMyPaymentsParams{
		ProfileID: claims.ProfileID,
		Locale:    loc,
		Limit:     int32(limit),
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "payment history query failed")
		return
	}
	out := make([]paymentHistoryDTO, len(rows))
	for i, row := range rows {
		dto := paymentHistoryDTO{
			ID:         row.ID.String(),
			TariffCode: row.TariffCode,
			TariffName: row.TariffName,
			TariffDays: int(row.TariffDays),
			AmountUzs:  row.AmountUzs,
			Provider:   row.Provider,
			Status:     row.Status,
			CreatedAt:  row.CreatedAt.Time,
		}
		if row.PaidAt.Valid {
			t := row.PaidAt.Time
			dto.PaidAt = &t
		}
		out[i] = dto
	}
	httpx.Data(w, http.StatusOK, out)
}
