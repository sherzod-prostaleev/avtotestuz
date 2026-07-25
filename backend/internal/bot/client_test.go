package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientSendMessageSuccess(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok123", srv.Client())
	if err := c.SendMessage(context.Background(), 42, "salom"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if gotPath != "/bottok123/sendMessage" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["chat_id"] != float64(42) || gotBody["text"] != "salom" {
		t.Errorf("body = %+v", gotBody)
	}
}

func TestClientSendMessageAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"description":"bot was blocked by the user"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok123", srv.Client())
	err := c.SendMessage(context.Background(), 42, "salom")
	if err == nil {
		t.Fatal("want error when ok:false")
	}
}

func TestClientSendMessageTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok123", srv.Client())
	err := c.SendMessage(context.Background(), 42, "salom")
	if err == nil {
		t.Fatal("want error on a non-JSON/5xx response")
	}
}

func TestClientGetUpdatesParsesResult(t *testing.T) {
	var gotOffset float64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotOffset = body["offset"].(float64)
		_, _ = w.Write([]byte(`{"ok":true,"result":[
			{"update_id":100,"message":{"message_id":1,"text":"/start","from":{"id":7,"username":"u"},"chat":{"id":7}}}
		]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok123", srv.Client())
	updates, err := c.GetUpdates(context.Background(), 99, 30)
	if err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if gotOffset != 99 {
		t.Errorf("offset sent = %v, want 99", gotOffset)
	}
	if len(updates) != 1 || updates[0].UpdateID != 100 {
		t.Fatalf("updates = %+v", updates)
	}
	if updates[0].Message == nil || updates[0].Message.Text != "/start" || updates[0].Message.From.ID != 7 {
		t.Fatalf("message = %+v", updates[0].Message)
	}
}

func TestClientSetWebhookSendsSecretToken(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok123", srv.Client())
	if err := c.SetWebhook(context.Background(), "https://example.test/hook", "s3cr3t"); err != nil {
		t.Fatalf("SetWebhook: %v", err)
	}
	if gotBody["url"] != "https://example.test/hook" || gotBody["secret_token"] != "s3cr3t" {
		t.Errorf("body = %+v", gotBody)
	}
}
