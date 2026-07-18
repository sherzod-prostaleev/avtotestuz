package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTelegramSenderSuccess(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewTelegramSender(srv.URL, "tok123", srv.Client())
	if err := s.Send(context.Background(), "+998901234567", "123456"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotPath != "/sendVerificationMessage" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "Bearer tok123" {
		t.Errorf("auth header = %q", gotAuth)
	}
	if gotBody["phone_number"] != "+998901234567" || gotBody["code"] != "123456" {
		t.Errorf("body = %+v", gotBody)
	}
	if s.Channel() != "telegram" {
		t.Errorf("channel = %q", s.Channel())
	}
}

func TestTelegramSenderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := NewTelegramSender(srv.URL, "tok123", srv.Client())
	if err := s.Send(context.Background(), "+998901234567", "123456"); err == nil {
		t.Fatal("want error on non-200 response")
	}
}
