package httpx

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestData(t *testing.T) {
	rec := httptest.NewRecorder()
	Data(rec, 200, map[string]string{"status": "ok"})
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("code=%d ct=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var body struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Data["status"] != "ok" {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}
}

func TestError(t *testing.T) {
	rec := httptest.NewRecorder()
	Error(rec, 404, "not_found", "no such thing")
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 404 || body.Error.Code != "not_found" || body.Error.Message != "no such thing" {
		t.Fatalf("got %d %+v", rec.Code, body)
	}
}
