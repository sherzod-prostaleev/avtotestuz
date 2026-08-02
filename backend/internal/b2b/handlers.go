package b2b

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/devicefp"
	"avtotest.uz/backend/internal/httpx"
)

// Handler exposes learner teacher-portal routes under /me/teacher/* and /me/invites*.
type Handler struct {
	Pool *pgxpool.Pool
}

func (h *Handler) store() Store { return Store{Pool: h.Pool} }

// AuthedRoutes mounts teacher portal endpoints (caller applies auth.Required).
func (h *Handler) AuthedRoutes(r chi.Router) {
	r.Get("/me/teacher/orgs", h.listOrgs)
	r.Get("/me/teacher/orgs/{id}", h.getOrg)
	r.Post("/me/teacher/orgs/{id}/invites", h.inviteMember)
	r.Get("/me/teacher/orgs/{id}/invites", h.listInvites)
	r.Delete("/me/teacher/orgs/{id}/members/{profileID}", h.removeMember)
	r.Patch("/me/teacher/orgs/{id}/members/{profileID}", h.changeRole)
	r.Get("/me/teacher/orgs/{id}/licenses", h.listLicenses)
	r.Get("/me/teacher/orgs/{id}/stats", h.orgStats)
	r.Get("/me/teacher/orgs/{id}/export.csv", h.exportCSV)
	r.Get("/me/teacher/orgs/{id}/stations", h.listStations)
	r.Post("/me/teacher/orgs/{id}/station-codes", h.createStationCode)
	r.Delete("/me/teacher/orgs/{id}/stations/{stationID}", h.revokeStation)
	r.Patch("/me/teacher/orgs/{id}/stations/{stationID}", h.renameStation)

	r.Post("/me/stations/activate", h.activateStation)

	r.Get("/me/invites", h.myInvites)
	r.Post("/me/invites/accept", h.acceptInvite)
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
		writeStoreErr(w, err, "org query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type inviteBody struct {
	Phone string `json:"phone"`
	Role  string `json:"role"`
}

func (h *Handler) inviteMember(w http.ResponseWriter, r *http.Request) {
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
	var body inviteBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	out, err := h.store().InviteByPhone(r.Context(), claims.ProfileID, orgID, body.Phone, body.Role)
	if err != nil {
		writeStoreErr(w, err, "invite failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) listInvites(w http.ResponseWriter, r *http.Request) {
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
	out, err := h.store().ListPendingInvites(r.Context(), claims.ProfileID, orgID)
	if err != nil {
		writeStoreErr(w, err, "invites query failed")
		return
	}
	if out == nil {
		out = []InviteRow{}
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) removeMember(w http.ResponseWriter, r *http.Request) {
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
	targetID, err := uuid.Parse(chi.URLParam(r, "profileID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid profile id")
		return
	}
	if err := h.store().RemoveMember(r.Context(), claims.ProfileID, orgID, targetID); err != nil {
		writeStoreErr(w, err, "remove member failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"removed": true})
}

type changeRoleBody struct {
	Role string `json:"role"`
}

func (h *Handler) changeRole(w http.ResponseWriter, r *http.Request) {
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
	targetID, err := uuid.Parse(chi.URLParam(r, "profileID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid profile id")
		return
	}
	var body changeRoleBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	out, err := h.store().ChangeRole(r.Context(), claims.ProfileID, orgID, targetID, body.Role)
	if err != nil {
		writeStoreErr(w, err, "change role failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) listLicenses(w http.ResponseWriter, r *http.Request) {
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
	if _, err := h.store().teacherRole(r.Context(), claims.ProfileID, orgID); err != nil {
		writeStoreErr(w, err, "licenses query failed")
		return
	}
	licenses, seats, err := h.store().listLicenses(r.Context(), orgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "licenses query failed")
		return
	}
	used, err := h.store().CountActiveStations(r.Context(), orgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "seat usage failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"licenses":     licenses,
		"active_seats": seats,
		"seats_used":   used,
	})
}

type stationCodeBody struct {
	Label    string `json:"label"`
	TTLHours int    `json:"ttl_hours"`
}

func (h *Handler) createStationCode(w http.ResponseWriter, r *http.Request) {
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
	var body stationCodeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && r.ContentLength != 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	ttl := time.Duration(body.TTLHours) * time.Hour
	if body.TTLHours <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	out, err := h.store().CreateActivateCodeAsTeacher(r.Context(), claims.ProfileID, orgID, body.Label, ttl)
	if err != nil {
		writeStoreErr(w, err, "create station code failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) listStations(w http.ResponseWriter, r *http.Request) {
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
	out, err := h.store().ListStationsAsTeacher(r.Context(), claims.ProfileID, orgID)
	if err != nil {
		writeStoreErr(w, err, "stations query failed")
		return
	}
	if out == nil {
		out = []StationRow{}
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) revokeStation(w http.ResponseWriter, r *http.Request) {
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
	stationID, err := uuid.Parse(chi.URLParam(r, "stationID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid station id")
		return
	}
	if err := h.store().RevokeStationAsTeacher(r.Context(), claims.ProfileID, orgID, stationID); err != nil {
		writeStoreErr(w, err, "revoke station failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"revoked": true})
}

type renameStationBody struct {
	Label string `json:"label"`
}

func (h *Handler) renameStation(w http.ResponseWriter, r *http.Request) {
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
	stationID, err := uuid.Parse(chi.URLParam(r, "stationID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid station id")
		return
	}
	var body renameStationBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	out, err := h.store().RenameStationAsTeacher(r.Context(), claims.ProfileID, orgID, stationID, body.Label)
	if err != nil {
		writeStoreErr(w, err, "rename station failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type activateStationBody struct {
	Code        string `json:"code"`
	Fingerprint string `json:"fingerprint"`
	Label       string `json:"label"`
}

func (h *Handler) activateStation(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	var body activateStationBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	fp := devicefp.Normalize(body.Fingerprint)
	if fp == "" {
		fp = devicefp.FromContext(r.Context())
	}
	out, err := h.store().ActivateStation(r.Context(), body.Code, fp, body.Label, "profile:"+claims.ProfileID.String())
	if err != nil {
		writeStoreErr(w, err, "activate station failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) orgStats(w http.ResponseWriter, r *http.Request) {
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
	if _, err := h.store().teacherRole(r.Context(), claims.ProfileID, orgID); err != nil {
		writeStoreErr(w, err, "stats query failed")
		return
	}
	out, err := h.store().OrgStats(r.Context(), orgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "stats query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) exportCSV(w http.ResponseWriter, r *http.Request) {
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
	if _, err := h.store().teacherRole(r.Context(), claims.ProfileID, orgID); err != nil {
		writeStoreErr(w, err, "export failed")
		return
	}
	csv, err := h.store().ExportMembersCSV(r.Context(), orgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "export failed")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="org-members.csv"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(csv)
}

func (h *Handler) myInvites(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	out, err := h.store().ListMyPendingInviteViews(r.Context(), claims.ProfileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "invites query failed")
		return
	}
	if out == nil {
		out = []PendingInviteView{}
	}
	httpx.Data(w, http.StatusOK, out)
}

type acceptInviteBody struct {
	Token string `json:"token"`
}

func (h *Handler) acceptInvite(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing auth")
		return
	}
	var body acceptInviteBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	out, err := h.store().AcceptInvite(r.Context(), claims.ProfileID, body.Token)
	if err != nil {
		writeStoreErr(w, err, "accept invite failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func writeStoreErr(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, ErrForbidden):
		httpx.Error(w, http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, ErrSeatsExhausted):
		httpx.Error(w, http.StatusConflict, "seats_exhausted", "active stations already fill license seats")
	case errors.Is(err, ErrOrgSuspended):
		httpx.Error(w, http.StatusConflict, "org_suspended", "org is suspended")
	case errors.Is(err, ErrNoLicense):
		httpx.Error(w, http.StatusBadRequest, "no_license", "org has no active license seats")
	case errors.Is(err, ErrConflict):
		msg := "conflict"
		if strings.Contains(err.Error(), "last owner") {
			msg = "cannot remove or demote the last owner"
		} else if strings.Contains(err.Error(), "already used") {
			msg = "activation code already used"
		}
		httpx.Error(w, http.StatusConflict, "conflict", msg)
	case errors.Is(err, ErrInvalid):
		httpx.Error(w, http.StatusBadRequest, "invalid", err.Error())
	default:
		httpx.Error(w, http.StatusInternalServerError, "internal", fallback)
	}
}
