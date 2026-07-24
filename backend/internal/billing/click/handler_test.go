package click

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestServeHTTP_Transport exercises the transport layer of the webhook:
// method check, required-fields check, sign check, and unknown-action
// dispatch. Unlike Payme, every response is still HTTP 200 once the
// request reaches the JSON body stage — Click carries failures in the
// {"error": ...} body, never in the status code — except the method
// check, which is a genuine HTTP-level rejection (405).
func TestServeHTTP_Transport(t *testing.T) {
	const serviceID = "S"
	const secretKey = "secret"

	h := &Handler{ServiceID: serviceID, SecretKey: secretKey}

	postForm := func(vals url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/billing/click", strings.NewReader(vals.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	t.Run("GET is rejected with 405", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/billing/click", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("POST missing sign_string returns errBadRequest", func(t *testing.T) {
		vals := url.Values{
			"click_trans_id":    {"123"},
			"service_id":        {serviceID},
			"merchant_trans_id": {"abc"},
			"amount":            {"5000"},
			"action":            {"0"},
			"sign_time":         {"1700000000"},
			// sign_string intentionally omitted
		}
		rec := postForm(vals)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		var resp clickResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.Error != errBadRequest {
			t.Fatalf("error = %d, want %d", resp.Error, errBadRequest)
		}
	})

	t.Run("POST with wrong sign_string returns errSignFailed", func(t *testing.T) {
		vals := url.Values{
			"click_trans_id":    {"123"},
			"service_id":        {serviceID},
			"merchant_trans_id": {"abc"},
			"amount":            {"5000"},
			"action":            {"0"},
			"sign_time":         {"1700000000"},
			"sign_string":       {"not-the-right-sign"},
		}
		rec := postForm(vals)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		var resp clickResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.Error != errSignFailed {
			t.Fatalf("error = %d, want %d", resp.Error, errSignFailed)
		}
	})

	t.Run("POST with valid sign but unknown action returns errAction", func(t *testing.T) {
		req := clickRequest{
			ClickTransID:    "123",
			ServiceID:       serviceID,
			MerchantTransID: "abc",
			Amount:          "5000",
			Action:          "9",
			SignTime:        "1700000000",
		}
		req.SignString = computeSign(req, serviceID, secretKey)

		vals := url.Values{
			"click_trans_id":    {req.ClickTransID},
			"service_id":        {req.ServiceID},
			"merchant_trans_id": {req.MerchantTransID},
			"amount":            {req.Amount},
			"action":            {req.Action},
			"sign_time":         {req.SignTime},
			"sign_string":       {req.SignString},
		}
		rec := postForm(vals)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		var resp clickResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.Error != errAction {
			t.Fatalf("error = %d, want %d", resp.Error, errAction)
		}
	})
}
