package click

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
)

// Handler serves the Click Shop API webhook (POST /api/v1/billing/click).
// Unlike Payme, there is no separate auth layer — the sign_string check
// (validSign) IS the authentication.
type Handler struct {
	Q         *sqlc.Queries
	Svc       billing.Service
	ServiceID string
	SecretKey string
	Pool      *pgxpool.Pool
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	req, ok := parseClickRequest(r)
	if !ok || !requiredFieldsPresent(req) {
		writeClickResponse(w, errorResponse(req, errBadRequest))
		return
	}
	if !validSign(req, h.ServiceID, h.SecretKey) {
		writeClickResponse(w, errorResponse(req, errSignFailed))
		return
	}
	switch req.Action {
	// Tasks 4-5 add cases here, each calling through writeClickResponse
	// exactly like this stub does, e.g.:
	//   case "0":
	//       writeClickResponse(w, h.prepare(r.Context(), req))
	//   case "1":
	//       writeClickResponse(w, h.complete(r.Context(), req))
	default:
		writeClickResponse(w, errorResponse(req, errAction))
	}
}

// requiredFieldsPresent mirrors Click's own required-field check: every
// core field must be set, and Complete (action=1) additionally requires
// merchant_prepare_id.
func requiredFieldsPresent(req clickRequest) bool {
	if req.ClickTransID == "" || req.ServiceID == "" || req.MerchantTransID == "" ||
		req.Amount == "" || req.Action == "" || req.SignTime == "" || req.SignString == "" {
		return false
	}
	if req.Action == "1" && req.MerchantPrepareID == "" {
		return false
	}
	return true
}
