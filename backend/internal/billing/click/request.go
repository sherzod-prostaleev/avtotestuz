package click

import (
	"encoding/json"
	"net/http"
)

type clickRequest struct {
	ClickTransID      string
	ServiceID         string
	ClickPaydocID     string
	MerchantTransID   string
	Amount            string
	Action            string
	Error             string
	ErrorNote         string
	SignTime          string
	SignString        string
	MerchantPrepareID string
}

// parseClickRequest reads Click's webhook fields from either a
// form-urlencoded body (Click's real-world behavior) or a JSON body
// (defensive fallback). r.ParseForm only consumes the body when
// Content-Type is application/x-www-form-urlencoded, so it's always
// safe to call first.
func parseClickRequest(r *http.Request) (clickRequest, bool) {
	if err := r.ParseForm(); err == nil && len(r.PostForm) > 0 {
		return clickRequest{
			ClickTransID:      r.PostFormValue("click_trans_id"),
			ServiceID:         r.PostFormValue("service_id"),
			ClickPaydocID:     r.PostFormValue("click_paydoc_id"),
			MerchantTransID:   r.PostFormValue("merchant_trans_id"),
			Amount:            r.PostFormValue("amount"),
			Action:            r.PostFormValue("action"),
			Error:             r.PostFormValue("error"),
			ErrorNote:         r.PostFormValue("error_note"),
			SignTime:          r.PostFormValue("sign_time"),
			SignString:        r.PostFormValue("sign_string"),
			MerchantPrepareID: r.PostFormValue("merchant_prepare_id"),
		}, true
	}
	var wire struct {
		ClickTransID      string `json:"click_trans_id"`
		ServiceID         string `json:"service_id"`
		ClickPaydocID     string `json:"click_paydoc_id"`
		MerchantTransID   string `json:"merchant_trans_id"`
		Amount            string `json:"amount"`
		Action            string `json:"action"`
		Error             string `json:"error"`
		ErrorNote         string `json:"error_note"`
		SignTime          string `json:"sign_time"`
		SignString        string `json:"sign_string"`
		MerchantPrepareID string `json:"merchant_prepare_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&wire); err != nil {
		return clickRequest{}, false
	}
	return clickRequest(wire), true
}
