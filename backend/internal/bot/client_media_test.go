package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientSendPhotoWithMarkup(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":9}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", srv.Client())
	markup := &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{{
		{Text: "A", CallbackData: "a:0"},
	}}}
	id, err := c.SendPhoto(context.Background(), -1, "http://media/x.png", "Savol?", markup)
	if err != nil {
		t.Fatal(err)
	}
	if id != 9 {
		t.Fatalf("id=%d", id)
	}
	if got["photo"] != "http://media/x.png" || got["caption"] != "Savol?" {
		t.Fatalf("body=%+v", got)
	}
	if got["reply_markup"] == nil {
		t.Fatal("expected reply_markup")
	}
}

func TestClientAnswerCallbackQuery(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", srv.Client())
	if err := c.AnswerCallbackQuery(context.Background(), "cb1", "ok", false); err != nil {
		t.Fatal(err)
	}
	if got["callback_query_id"] != "cb1" {
		t.Fatalf("body=%+v", got)
	}
}
