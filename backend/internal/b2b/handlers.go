package b2b

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/httpx"
)

// Handler exposes learner teacher-portal routes under /me/teacher/*.
type Handler struct {
	Pool *pgxpool.Pool
}

func (h *Handler) store() Store { return Store{Pool: h.Pool} }

// AuthedRoutes mounts teacher portal endpoints (caller applies auth.Required).
func (h *Handler) AuthedRoutes(r chi.Router) {
	r.Get("/me/teacher/orgs", h.listOrgs)
	r.Get("/me/teacher/orgs/{id}", h.getOrg)
}

func (h *Handler) listOrgs(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	out, err := h.store().ListTeacherOrgs(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "orgs query failed")
		return
	}
	if out == nil {
		out = []OrgSummary{}
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getOrg(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	orgID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid org id")
		return
	}
	out, err := h.store().GetTeacherOrgDetail(r.Context(), claims.ProfileID, orgID)
	if err != nil {
		if err == pgx.ErrNoRows {
			httpx.Error(w, http.StatusNotFound, "not_found", "org not found or not teacher/owner")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "org query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}
