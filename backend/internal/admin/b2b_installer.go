package admin

import (
	"errors"
	"net/http"
	"os"
	"time"

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
	row, err := h.b2bStore().ActiveInstallerKey(r.Context(), orgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "installer key query failed")
		return
	}
	if row == nil {
		httpx.Data(w, http.StatusOK, nil)
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
	actor := adminActorLabel(r)
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
		writeInstallerErr(w, err)
		return
	}
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
		httpx.Error(w, http.StatusNotFound, "not_found", "org not found")
		return
	}

	base, err := os.Open(h.StationBinaryPath)
	if err != nil {
		httpx.Error(w, http.StatusServiceUnavailable, "installer_unavailable",
			"station binary is not present in this build")
		return
	}
	defer func() { _ = base.Close() }()

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
	default:
		httpx.Error(w, http.StatusInternalServerError, "internal", "installer key failed")
	}
}
