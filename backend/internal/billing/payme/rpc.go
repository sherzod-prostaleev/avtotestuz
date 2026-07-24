// Package payme implements the Payme (Paycom) Merchant API webhook: a
// single JSON-RPC 2.0 POST endpoint that always answers HTTP 200, carrying
// success or failure in the JSON body per the JSON-RPC 2.0 envelope.
package payme

import (
	"encoding/json"
	"net/http"
)

// rpcRequest is a JSON-RPC 2.0 request as sent by Payme. ID may be a JSON
// number, string, or null — json.RawMessage defers decoding so callers
// don't have to guess the type; it is echoed back verbatim in the response.
// Params is left raw here; per-method param structs are decoded by the
// individual method handlers added in later tasks.
type rpcRequest struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// rpcResponse is the JSON-RPC 2.0 envelope written back to Payme. Exactly
// one of Result/Error is set per the spec.
type rpcResponse struct {
	Result any             `json:"result,omitempty"`
	Error  *rpcError       `json:"error,omitempty"`
	ID     json.RawMessage `json:"id"`
}

// writeResult writes a successful {result, id} response. Always HTTP 200.
func writeResult(w http.ResponseWriter, id json.RawMessage, result any) {
	writeJSON(w, rpcResponse{Result: result, ID: id})
}

// writeError writes a {error, id} response. Always HTTP 200 — Payme expects
// every error, including transport-level ones, inside the JSON-RPC body.
func writeError(w http.ResponseWriter, id json.RawMessage, err *rpcError) {
	writeJSON(w, rpcResponse{Error: err, ID: id})
}

func writeJSON(w http.ResponseWriter, resp rpcResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
