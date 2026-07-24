package click

import (
	"encoding/json"
	"net/http"
)

type clickResponse struct {
	ClickTransID      string `json:"click_trans_id"`
	MerchantTransID   string `json:"merchant_trans_id"`
	MerchantPrepareID string `json:"merchant_prepare_id,omitempty"`
	MerchantConfirmID string `json:"merchant_confirm_id,omitempty"`
	Error             int    `json:"error"`
	ErrorNote         string `json:"error_note"`
}

func writeClickResponse(w http.ResponseWriter, resp clickResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func errorResponse(req clickRequest, code int) clickResponse {
	return clickResponse{
		ClickTransID:    req.ClickTransID,
		MerchantTransID: req.MerchantTransID,
		Error:           code,
		ErrorNote:       errorNotes[code],
	}
}
