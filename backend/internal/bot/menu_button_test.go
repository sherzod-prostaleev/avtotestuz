package bot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSyncMenuButtonSendsWebAppOrDefault(t *testing.T) {
	var got []map[string]any
	var paths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(b, &payload)
		got = append(got, payload)
		paths = append(paths, r.URL.Path)
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer ts.Close()
	c := NewClient(ts.URL, "tok", ts.Client())

	if err := SyncMenuButton(context.Background(), c, "https://drivergo.uz/uz-Latn/tg"); err != nil {
		t.Fatal(err)
	}
	if err := SyncMenuButton(context.Background(), c, ""); err != nil {
		t.Fatal(err)
	}
	if paths[0] != "/bottok/setChatMenuButton" {
		t.Fatalf("path = %q", paths[0])
	}
	first := got[0]["menu_button"].(map[string]any)
	if first["type"] != "web_app" || first["text"] != "Ochish" ||
		first["web_app"].(map[string]any)["url"] != "https://drivergo.uz/uz-Latn/tg" {
		t.Fatalf("first = %v", first)
	}
	if got[1]["menu_button"].(map[string]any)["type"] != "default" {
		t.Fatalf("second = %v", got[1])
	}
}

func TestSyncMenuButtonReturnsAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"description":"boom"}`))
	}))
	defer ts.Close()
	if err := SyncMenuButton(context.Background(), NewClient(ts.URL, "tok", ts.Client()), ""); err == nil {
		t.Fatal("want error when Telegram rejects the call")
	}
}
