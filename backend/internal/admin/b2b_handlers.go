package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/httpx"
)

func (h *Handler) listB2BOrgs(w http.ResponseWriter, r *http.Request) {
	out, err := h.Svc.Store.ListB2BOrgs(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "b2b orgs query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type createB2BOrgBody struct {
	Name string `json:"name"`
}

func (h *Handler) createB2BOrg(w http.ResponseWriter, r *http.Request) {
	var body createB2BOrgBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	out, err := h.Svc.Store.CreateB2BOrg(r.Context(), body.Name)
	if err != nil {
		if strings.Contains(err.Error(), "name required") {
			httpx.Error(w, http.StatusBadRequest, "name_required", "name is required")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to create org")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "b2b.orgs.create", "b2b_org", out.ID.String(),
		nil, map[string]any{"name": out.Name}, clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getB2BOrg(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	out, err := h.Svc.Store.GetB2BOrgDetail(r.Context(), id)
	if err != nil {
		if IsNoRows(err) {
			httpx.Error(w, http.StatusNotFound, "not_found", "org not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "b2b org query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) hardDeleteB2BOrg(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body hardDeleteBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	claims, ok := FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}
	err := h.Svc.Store.HardDeleteB2BOrg(r.Context(), orgID, body.Confirm, MutationAudit{
		AdminUserID: claims.AdminUserID,
		IP:          clientIP(r),
		UA:          r.UserAgent(),
		RequestID:   middleware.GetReqID(r.Context()),
	})
	if err != nil {
		switch {
		case IsNoRows(err):
			httpx.Error(w, http.StatusNotFound, "not_found", "org not found")
		case errors.Is(err, ErrDeleteConfirmation):
			httpx.Error(w, http.StatusBadRequest, "confirmation_mismatch", "confirmation must match organization name")
		default:
			httpx.Error(w, http.StatusInternalServerError, "internal", "organization hard delete failed")
		}
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"id": orgID, "deleted": true})
}

type addB2BMemberBody struct {
	ProfileID string `json:"profile_id"`
	Role      string `json:"role"`
}

func (h *Handler) addB2BMember(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body addB2BMemberBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	profileID, err := uuid.Parse(strings.TrimSpace(body.ProfileID))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_profile_id", "profile_id must be uuid")
		return
	}
	out, err := h.Svc.Store.AddB2BMember(r.Context(), orgID, profileID, body.Role)
	if err != nil {
		if IsNoRows(err) {
			httpx.Error(w, http.StatusNotFound, "not_found", "org not found")
			return
		}
		if strings.Contains(err.Error(), "profile not found") {
			httpx.Error(w, http.StatusNotFound, "profile_not_found", "profile not found")
			return
		}
		if strings.Contains(err.Error(), "invalid role") {
			httpx.Error(w, http.StatusBadRequest, "invalid_role", "role must be owner, teacher, or student")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to add member")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "b2b.members.add", "b2b_org", orgID.String(),
		nil, map[string]any{"profile_id": profileID.String(), "role": out.Role},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, out)
}

type createB2BLicenseBody struct {
	Seats     int    `json:"seats"`
	HomeSeats int    `json:"home_seats"`
	Days      int    `json:"days"`
	Note      string `json:"note"`
}

func (h *Handler) createB2BLicense(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body createB2BLicenseBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	out, err := h.Svc.Store.CreateB2BLicense(r.Context(), orgID, body.Seats, body.Days, body.HomeSeats, body.Note, "admin:"+adminID.String())
	if err != nil {
		if IsNoRows(err) {
			httpx.Error(w, http.StatusNotFound, "not_found", "org not found")
			return
		}
		if strings.Contains(err.Error(), "positive") || strings.Contains(err.Error(), "home_seats") {
			httpx.Error(w, http.StatusBadRequest, "invalid_license", err.Error())
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to create license")
		return
	}
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "b2b.licenses.create", "b2b_org_license", out.ID.String(),
		nil, map[string]any{"org_id": orgID.String(), "seats": out.Seats, "home_seats": out.HomeSeats, "ends_at": out.EndsAt},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, out)
}

type grantB2BBody struct {
	Days int    `json:"days"`
	Note string `json:"note"`
}

func (h *Handler) grantB2BMember(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	profileID, ok := parseUUIDParam(w, r, "profileID")
	if !ok {
		return
	}
	var body grantB2BBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	if body.Days <= 0 || body.Days > 3660 {
		httpx.Error(w, http.StatusBadRequest, "invalid_days", "days must be 1..3660")
		return
	}
	member, err := h.Svc.Store.IsB2BMember(r.Context(), orgID, profileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "member check failed")
		return
	}
	if !member {
		httpx.Error(w, http.StatusBadRequest, "not_member", "profile is not an org member")
		return
	}
	// Personal grant is the Home Seat SKU (not classroom stations).
	homeSeats, err := h.b2bStore().ActiveHomeSeats(r.Context(), orgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "license check failed")
		return
	}
	if homeSeats <= 0 {
		httpx.Error(w, http.StatusBadRequest, "no_home_seats", "org has no active home seat quota")
		return
	}
	var orgStatus string
	if err := h.Svc.Store.Pool.QueryRow(r.Context(), `SELECT status FROM b2b_org WHERE id=$1`, orgID).Scan(&orgStatus); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "org check failed")
		return
	}
	if orgStatus != "active" {
		httpx.Error(w, http.StatusConflict, "org_suspended", "org is suspended")
		return
	}
	already, err := h.Svc.Store.HasActiveB2BGrant(r.Context(), orgID, profileID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "grant check failed")
		return
	}
	if !already {
		used, err := h.Svc.Store.CountActiveB2BGrants(r.Context(), orgID)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "internal", "home seat usage failed")
			return
		}
		if used >= homeSeats {
			httpx.Error(w, http.StatusConflict, "home_seats_exhausted", "active home grants already fill home seat quota")
			return
		}
	}
	if noteBody := strings.TrimSpace(body.Note); noteBody == "" {
		body.Note = "home seat"
	}
	if h.Pool == nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "pool required")
		return
	}
	note := strings.TrimSpace(body.Note)
	if note == "" {
		note = "home seat"
	}
	note = "b2b_org=" + orgID.String() + "; kind=home; " + note

	ctx := r.Context()
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "begin tx failed")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	svc := billing.Service{Q: sqlc.New(tx)}
	until, err := svc.GrantDays(ctx, profileID, body.Days, "b2b", note, uuid.NullUUID{})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "grant failed")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "commit failed")
		return
	}

	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "b2b.entitlements.grant", "entitlement", profileID.String(),
		nil, map[string]any{
			"org_id": orgID.String(),
			"days":   body.Days,
			"until":  until.UTC().Format(time.RFC3339),
			"source": "b2b",
		},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, map[string]any{
		"profile_id": profileID,
		"org_id":     orgID,
		"source":     "b2b",
		"until":      until.UTC(),
		"days":       body.Days,
	})
}

func (h *Handler) b2bStore() b2b.Store { return b2b.Store{Pool: h.Pool} }

func writeB2BStoreErr(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, b2b.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, b2b.ErrForbidden):
		httpx.Error(w, http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, b2b.ErrSeatsExhausted):
		httpx.Error(w, http.StatusConflict, "seats_exhausted", "active stations already fill license seats")
	case errors.Is(err, b2b.ErrOrgSuspended):
		httpx.Error(w, http.StatusConflict, "org_suspended", "org is suspended")
	case errors.Is(err, b2b.ErrNoLicense):
		httpx.Error(w, http.StatusBadRequest, "no_license", "org has no active license seats")
	case errors.Is(err, b2b.ErrConflict):
		msg := "conflict"
		if strings.Contains(err.Error(), "last owner") {
			msg = "cannot remove or demote the last owner"
		} else if strings.Contains(err.Error(), "already used") {
			msg = "activation code already used"
		}
		httpx.Error(w, http.StatusConflict, "conflict", msg)
	case errors.Is(err, b2b.ErrInvalid):
		httpx.Error(w, http.StatusBadRequest, "invalid", err.Error())
	default:
		httpx.Error(w, http.StatusInternalServerError, "internal", fallback)
	}
}

func (h *Handler) getB2BOrgStats(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var exists bool
	if err := h.Svc.Store.Pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM b2b_org WHERE id=$1)`, orgID).Scan(&exists); err != nil || !exists {
		if !exists {
			httpx.Error(w, http.StatusNotFound, "not_found", "org not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "org check failed")
		return
	}
	out, err := h.b2bStore().OrgStats(r.Context(), orgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "stats query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) exportB2BOrgCSV(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var exists bool
	if err := h.Svc.Store.Pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM b2b_org WHERE id=$1)`, orgID).Scan(&exists); err != nil || !exists {
		if !exists {
			httpx.Error(w, http.StatusNotFound, "not_found", "org not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "org check failed")
		return
	}
	csv, err := h.b2bStore().ExportMembersCSV(r.Context(), orgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "export failed")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="org-members.csv"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(csv)
}

type inviteB2BBody struct {
	Phone string `json:"phone"`
	Role  string `json:"role"`
}

func (h *Handler) inviteB2BMember(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body inviteB2BBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	createdBy, err := h.b2bStore().FirstOwnerProfile(r.Context(), orgID)
	if err != nil && !errors.Is(err, b2b.ErrNotFound) {
		writeB2BStoreErr(w, err, "invite failed")
		return
	}
	// No owner yet: created_by stays null (allowed by mig 0036).
	out, err := h.b2bStore().EnrollOrInvite(r.Context(), orgID, body.Phone, body.Role, createdBy)
	if err != nil {
		writeB2BStoreErr(w, err, "invite failed")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "b2b.members.invite", "b2b_org", orgID.String(),
		nil, map[string]any{"phone": body.Phone, "role": body.Role, "status": out.Status},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) removeB2BMember(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	profileID, ok := parseUUIDParam(w, r, "profileID")
	if !ok {
		return
	}
	if err := h.b2bStore().AdminRemoveMember(r.Context(), orgID, profileID); err != nil {
		writeB2BStoreErr(w, err, "remove member failed")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "b2b.members.remove", "b2b_org", orgID.String(),
		nil, map[string]any{"profile_id": profileID.String()},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, map[string]any{"removed": true})
}

type changeB2BRoleBody struct {
	Role string `json:"role"`
}

type patchB2BOrgBody struct {
	Status string `json:"status"`
}

func (h *Handler) patchB2BOrg(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body patchB2BOrgBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	if err := h.b2bStore().SetOrgStatus(r.Context(), orgID, body.Status); err != nil {
		writeB2BStoreErr(w, err, "update org failed")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "b2b.orgs.status", "b2b_org", orgID.String(),
		nil, map[string]any{"status": body.Status},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, map[string]any{"id": orgID, "status": body.Status})
}

func (h *Handler) listB2BStations(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	out, err := h.b2bStore().ListStations(r.Context(), orgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "stations query failed")
		return
	}
	if out == nil {
		out = []b2b.StationRow{}
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) revokeB2BStation(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	stationID, ok := parseUUIDParam(w, r, "stationID")
	if !ok {
		return
	}
	if err := h.b2bStore().RevokeStation(r.Context(), orgID, stationID); err != nil {
		writeB2BStoreErr(w, err, "revoke station failed")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "b2b.stations.revoke", "b2b_station", stationID.String(),
		nil, map[string]any{"org_id": orgID.String()},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, map[string]any{"revoked": true})
}

type partnerPromoBody struct {
	Code      string `json:"code"`
	Kind      string `json:"kind"`
	Value     int    `json:"value"`
	ValidDays int    `json:"valid_days"`
}

func (h *Handler) createB2BPartnerPromo(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body partnerPromoBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	out, err := h.b2bStore().CreatePartnerPromo(r.Context(), orgID, body.Code, body.Kind, body.Value, body.ValidDays)
	if err != nil {
		writeB2BStoreErr(w, err, "create partner promo failed")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "b2b.partner_promo.create", "promo_code", out.ID.String(),
		nil, map[string]any{"org_id": orgID.String(), "code": out.Code},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) listB2BPartnerPromos(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.b2bStore().EnsureOrgExists(r.Context(), orgID); err != nil {
		writeB2BStoreErr(w, err, "org check failed")
		return
	}
	out, err := h.b2bStore().ListPartnerPromos(r.Context(), orgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "partner promos query failed")
		return
	}
	if out == nil {
		out = []b2b.PartnerPromo{}
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) changeB2BMemberRole(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	profileID, ok := parseUUIDParam(w, r, "profileID")
	if !ok {
		return
	}
	var body changeB2BRoleBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	out, err := h.b2bStore().AdminChangeRole(r.Context(), orgID, profileID, body.Role)
	if err != nil {
		writeB2BStoreErr(w, err, "change role failed")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "b2b.members.role", "b2b_org", orgID.String(),
		nil, map[string]any{"profile_id": profileID.String(), "role": out.Role},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, out)
}
