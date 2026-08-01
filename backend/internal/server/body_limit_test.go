package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLimitRequestBodyRejectsKnownOversizeBody(t *testing.T) {
	called := false
	h := limitRequestBody(8)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("123456789"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge || called {
		t.Fatalf("status=%d called=%v", w.Code, called)
	}
}

func TestLimitRequestBodyBoundsChunkedBody(t *testing.T) {
	h := limitRequestBody(8)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if _, ok := err.(*http.MaxBytesError); !ok {
			t.Fatalf("read error=%T %v, want *http.MaxBytesError", err, err)
		}
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", io.NopCloser(bytes.NewBufferString("123456789")))
	req.ContentLength = -1
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d", w.Code)
	}
}
