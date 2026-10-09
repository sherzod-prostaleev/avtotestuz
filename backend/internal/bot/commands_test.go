package bot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSyncCommandsSetsPrivateAndGroupMenusInBothLanguages(t *testing.T) {
	var paths []string
	var bodies []map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		paths = append(paths, r.URL.Path)
		bodies = append(bodies, body)
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer ts.Close()

	if err := SyncCommands(context.Background(), NewClient(ts.URL, "tok", ts.Client())); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 4 {
		t.Fatalf("calls = %d, want 4", len(bodies))
	}
	type key struct{ scope, lang string }
	got := map[key][]string{}
	descs := map[key][]string{}
	for i, b := range bodies {
		if paths[i] != "/bottok/setMyCommands" {
			t.Fatalf("path = %q", paths[i])
		}
		scope := b["scope"].(map[string]any)["type"].(string)
		lang, _ := b["language_code"].(string)
		for _, c := range b["commands"].([]any) {
			m := c.(map[string]any)
			k := key{scope, lang}
			got[k] = append(got[k], m["command"].(string))
			d := m["description"].(string)
			descs[k] = append(descs[k], d)
			if n := utf8.RuneCountInString(d); n == 0 || n > 30 {
				t.Errorf("%v /%s description %q is %d chars, want 1..30", k, m["command"], d, n)
			}
		}
	}
	want := map[key]string{
		{"all_private_chats", ""}:   "start quiz status eslatma help",
		{"all_private_chats", "ru"}: "start quiz status eslatma help",
		{"all_group_chats", ""}:     "quiz next stop",
		{"all_group_chats", "ru"}:   "quiz next stop",
	}
	for k, w := range want {
		if strings.Join(got[k], " ") != w {
			t.Errorf("%v commands = %v, want %s", k, got[k], w)
		}
	}
	if descs[key{"all_private_chats", ""}][0] != "Bosh menyu" || descs[key{"all_private_chats", "ru"}][0] != "Главное меню" {
		t.Errorf("start descriptions = %v / %v", descs[key{"all_private_chats", ""}], descs[key{"all_private_chats", "ru"}])
	}
}

func TestSyncCommandsReturnsFirstAPIError(t *testing.T) {
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"boom"}`))
	}))
	defer ts.Close()
	if err := SyncCommands(context.Background(), NewClient(ts.URL, "tok", ts.Client())); err == nil {
		t.Fatal("want an error when Telegram rejects setMyCommands")
	}
	// One rejected set must not stop the others from being attempted.
	if calls != 4 {
		t.Errorf("calls = %d, want all 4 attempted", calls)
	}
}
