package admin

import (
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/httpx"
)

type installerKeyDTO struct {
	Code      string    `json:"code"`
	MaxUses   int       `json:"max_uses"`
	UsedCount int       `json:"used_count"`
	ExpiresAt time.Time `json:"expires_at"`
}

func toInstallerKeyDTO(row b2b.EnrollCodeRow) installerKeyDTO {
	return installerKeyDTO{
		Code: row.Code, MaxUses: row.MaxUses,
		UsedCount: row.UsedCount, ExpiresAt: row.ExpiresAt,
	}
}

// allowedInstallerLocales mirrors the frontend's locale list. An unknown value
// would produce an installer whose kiosk URL 404s on every classroom PC, so it
// is rejected here rather than discovered in a school.
var allowedInstallerLocales = map[string]bool{"uz-Latn": true, "uz-Cyrl": true, "ru": true}

// adminActorLabel follows the "admin:<id>" convention already used for
// created_by columns elsewhere in this package (see createB2BLicense).
func adminActorLabel(r *http.Request) string {
	claims, _ := FromContext(r.Context())
	return "admin:" + claims.AdminUserID.String()
}

func (h *Handler) getB2BInstaller(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.b2bStore().EnsureOrgExists(r.Context(), orgID); err != nil {
		if errors.Is(err, b2b.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", "org not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "org check failed")
		return
	}
	row, err := h.b2bStore().ActiveInstallerKey(r.Context(), orgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "installer key query failed")
		return
	}
	if row == nil {
		// A typed nil, not a bare nil: the interface value httpx.Data receives
		// then holds a non-nil type with a nil pointer, which the JSON encoder
		// serialises as {"data":null} rather than dropping the key via
		// envelope's `omitempty` (see TestGetInstallerNoKeyBodyIsExplicitNull).
		httpx.Data(w, http.StatusOK, (*installerKeyDTO)(nil))
		return
	}
	httpx.Data(w, http.StatusOK, toInstallerKeyDTO(*row))
}

func (h *Handler) openB2BInstaller(w http.ResponseWriter, r *http.Request) {
	h.installerKeyWrite(w, r, false)
}

func (h *Handler) rotateB2BInstaller(w http.ResponseWriter, r *http.Request) {
	h.installerKeyWrite(w, r, true)
}

func (h *Handler) installerKeyWrite(w http.ResponseWriter, r *http.Request, rotate bool) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	actor := adminActorLabel(r)
	action := "b2b.installer.open"
	if rotate {
		action = "b2b.installer.rotate"
	}
	var (
		row b2b.EnrollCodeRow
		err error
	)
	if rotate {
		row, err = h.b2bStore().RotateInstallerKey(r.Context(), orgID, actor)
	} else {
		row, err = h.b2bStore().OpenInstallerKey(r.Context(), orgID, actor)
	}
	if err != nil {
		if rotate && errors.Is(err, b2b.ErrInstallerKeyRotatedNoSeats) {
			// The rotate call's emergency stop already ran -- the old key is
			// dead -- even though there's no replacement to hand back. That
			// is a real mutation an operator needs to be able to find later
			// (e.g. "who killed the key, and when"), so it's audited like any
			// other successful write, not swallowed into the generic error
			// path below. The key itself was never minted, so there's
			// nothing secret in this entry.
			_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, action, "b2b_org_enroll_code", orgID.String(),
				nil, map[string]any{"org_id": orgID.String(), "result": "revoked_no_replacement"},
				clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
			)
		}
		writeInstallerErr(w, err)
		return
	}
	// Audit the exposure, not the key: an installer code is a bearer
	// credential that enrols classroom PCs and spends a school's seats, and
	// "who fetched or minted it" is the whole compensating control for a
	// leak (see downloadB2BInstaller's audit call for the download side of
	// the same concern). Following the WriteAudit shape used elsewhere in
	// this package (see createB2BLicense) -- never the code itself.
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, action, "b2b_org_enroll_code", row.ID.String(),
		nil, map[string]any{"org_id": orgID.String(), "max_uses": row.MaxUses, "expires_at": row.ExpiresAt},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, toInstallerKeyDTO(row))
}

func (h *Handler) downloadB2BInstaller(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	locale := r.URL.Query().Get("locale")
	if locale == "" {
		locale = "uz-Latn"
	}
	if !allowedInstallerLocales[locale] {
		httpx.Error(w, http.StatusBadRequest, "invalid_locale", "locale must be uz-Latn, uz-Cyrl or ru")
		return
	}

	row, err := h.b2bStore().ActiveInstallerKey(r.Context(), orgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "installer key query failed")
		return
	}
	if row == nil {
		// A GET must not mint a key as a side effect.
		httpx.Error(w, http.StatusConflict, "no_installer_key", "open an installer key first")
		return
	}

	var orgName string
	if err := h.Pool.QueryRow(r.Context(),
		`SELECT name FROM b2b_org WHERE id = $1`, orgID).Scan(&orgName); err != nil {
		if IsNoRows(err) {
			httpx.Error(w, http.StatusNotFound, "not_found", "org not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "org query failed")
		return
	}

	base, err := os.Open(h.StationBinaryPath)
	if err != nil {
		httpx.Error(w, http.StatusServiceUnavailable, "installer_unavailable",
			"station binary is not present in this build")
		return
	}
	defer func() { _ = base.Close() }()

	// This is the actual moment a bearer credential leaves the admin panel as
	// a working, pre-armed binary. Audited before streaming starts (not
	// after: AppendConfig writes straight to w, so there is no later point to
	// hook in without buffering the whole download) -- same shape as
	// installerKeyWrite's audit call, never the code itself.
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "b2b.installer.download", "b2b_org_enroll_code", row.ID.String(),
		nil, map[string]any{"org_id": orgID.String(), "locale": locale},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition",
		`attachment; filename="`+b2b.InstallerFilename(orgName, orgID)+`"`)
	// Streamed, so no Content-Length: the trailer's size is not known until the
	// base has been copied, and buffering a 7 MB binary per download to learn it
	// buys nothing.
	if err := b2b.AppendConfig(w, base, b2b.InstallerConfig{
		Code:     row.Code,
		API:      h.PublicBaseURL,
		Frontend: h.PublicBaseURL,
		Org:      orgName,
		Locale:   locale,
	}); err != nil {
		// The status is already sent; the truncated download is the signal.
		return
	}
}

func writeInstallerErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, b2b.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "org not found")
	case errors.Is(err, b2b.ErrOrgSuspended):
		httpx.Error(w, http.StatusConflict, "org_suspended", "org is suspended")
	case errors.Is(err, b2b.ErrNoLicense):
		httpx.Error(w, http.StatusConflict, "no_license", "org has no active licence")
	case errors.Is(err, b2b.ErrSeatsExhausted):
		httpx.Error(w, http.StatusConflict, "seats_exhausted", "all licensed seats are in use")
	case errors.Is(err, b2b.ErrInstallerKeyRotatedNoSeats):
		// Distinct from seats_exhausted: the rotate call's emergency stop
		// already ran and the old key is dead, there just wasn't a free seat
		// to mint a replacement into. Collapsing this into seats_exhausted
		// would read to the caller as "rotate did nothing", which is false.
		httpx.Error(w, http.StatusConflict, "rotated_no_seats",
			"installer key revoked; no free seats for a replacement")
	default:
		httpx.Error(w, http.StatusInternalServerError, "internal", "installer key failed")
	}
}
