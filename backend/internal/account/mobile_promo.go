package account

import (
	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/httpx"
	"net/http"
)

func (h *Handler) getMobilePromo(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, 401, "unauthorized", "missing auth")
		return
	}
	profile, err := h.Q.GetProfileByID(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, 500, "internal", "profile query failed")
		return
	}
	if profile.Kind != "station" {
		httpx.Error(w, 403, "station_only", "classroom station required")
		return
	}
	p, err := (b2b.Store{Pool: h.Billing.Pool}).StationMobilePromo(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, 500, "internal", "mobile promotion query failed")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.Data(w, 200, p)
}
