package b2b

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/httpx"
)

// Handler exposes the public station enroll/challenge/token routes (see
// station_handlers.go). The admin panel is the only control surface for
// everything else — orgs, licences and stations are managed entirely through
// backend/internal/admin.
type Handler struct {
	Pool      *pgxpool.Pool
	Redis     *redis.Client
	Secret    []byte
	Lim       auth.Limiter
	ClientIPs auth.ClientIPResolver
	// StationBinaryPath and StationVersionPath describe the Windows agent
	// baked into this image, which installed classroom PCs poll to update
	// themselves. Both are written by backend/Dockerfile's station stage.
	StationBinaryPath  string
	StationVersionPath string
}

func (h *Handler) store() Store { return Store{Pool: h.Pool} }

func writeStoreErr(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, ErrForbidden):
		httpx.Error(w, http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, ErrSeatsExhausted):
		httpx.Error(w, http.StatusConflict, "seats_exhausted", "active stations already fill license seats")
	case errors.Is(err, ErrCodeExhausted):
		httpx.Error(w, http.StatusConflict, "code_exhausted", "enrollment code has already been used up; ask your school admin to revoke a station or rotate the key")
	case errors.Is(err, ErrOrgSuspended):
		httpx.Error(w, http.StatusConflict, "org_suspended", "org is suspended")
	case errors.Is(err, ErrNoLicense):
		httpx.Error(w, http.StatusBadRequest, "no_license", "org has no active license seats")
	case errors.Is(err, ErrStationAuth):
		httpx.Error(w, http.StatusUnauthorized, "station_unauthorized", "station authentication failed")
	case errors.Is(err, ErrHWIDOtherOrg):
		httpx.Error(w, http.StatusConflict, "hwid_other_org",
			"this computer is already registered to a different driving school; revoke it there first")
	case errors.Is(err, ErrConflict):
		msg := "conflict"
		if strings.Contains(err.Error(), "already used") {
			msg = "activation code already used"
		}
		httpx.Error(w, http.StatusConflict, "conflict", msg)
	case errors.Is(err, ErrInvalid):
		httpx.Error(w, http.StatusBadRequest, "invalid", err.Error())
	default:
		httpx.Error(w, http.StatusInternalServerError, "internal", fallback)
	}
}
