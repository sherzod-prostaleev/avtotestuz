package payme

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
)

// Handler serves the Payme JSON-RPC webhook (POST /api/v1/billing/payme).
// Key is the Basic-auth password — the cashbox KEY for the active
// PAYME_ENV (PAYME_TEST_KEY or PAYME_KEY). An empty Key means Payme isn't
// configured, so every call is rejected with -32504.
type Handler struct {
	Q   *sqlc.Queries
	Svc billing.Service
	Key string
}

// ServeHTTP implements the JSON-RPC 2.0 transport contract: method check,
// Basic-auth, JSON parse, method dispatch. It always answers HTTP 200 —
// Payme carries failures in the JSON-RPC error object, never in the status
// code, so every early-return here writes a {error, id} body rather than
// calling http.Error.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, nil, errNotPost)
		return
	}

	if !h.checkAuth(r) {
		writeError(w, nil, errAuth)
		return
	}

	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, nil, errParse)
		return
	}

	switch req.Method {
	// Task 6 adds cases here: CheckTransaction, GetStatement.
	case "CheckPerformTransaction":
		var p checkPerformParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			writeError(w, req.ID, errParse)
			return
		}
		result, rpcErr := h.checkPerform(r.Context(), p)
		if rpcErr != nil {
			writeError(w, req.ID, rpcErr)
			return
		}
		writeResult(w, req.ID, result)
	case "CreateTransaction":
		var p createTransactionParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			writeError(w, req.ID, errParse)
			return
		}
		result, rpcErr := h.createTransaction(r.Context(), p)
		if rpcErr != nil {
			writeError(w, req.ID, rpcErr)
			return
		}
		writeResult(w, req.ID, result)
	case "PerformTransaction":
		var p performTransactionParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			writeError(w, req.ID, errParse)
			return
		}
		result, rpcErr := h.performTransaction(r.Context(), p)
		if rpcErr != nil {
			writeError(w, req.ID, rpcErr)
			return
		}
		writeResult(w, req.ID, result)
	case "CancelTransaction":
		var p cancelTransactionParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			writeError(w, req.ID, errParse)
			return
		}
		result, rpcErr := h.cancelTransaction(r.Context(), p)
		if rpcErr != nil {
			writeError(w, req.ID, rpcErr)
			return
		}
		writeResult(w, req.ID, result)
	default:
		writeError(w, req.ID, errUnknownMethod)
	}
}

// checkAuth validates the Basic-auth header against login "Paycom" and
// password Key, using constant-time comparison. An empty Key (Payme not
// configured for this environment) always fails, by design.
func (h *Handler) checkAuth(r *http.Request) bool {
	if h.Key == "" {
		return false
	}
	login, pass, ok := r.BasicAuth()
	if !ok {
		return false
	}
	loginOK := subtle.ConstantTimeCompare([]byte(login), []byte("Paycom")) == 1
	passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(h.Key)) == 1
	return loginOK && passOK
}
