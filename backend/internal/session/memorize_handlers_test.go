package session_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCategoryMemorizeRequiresAuth(t *testing.T) {
	ts, _, _ := setupServer(t)
	status, _ := doReq(t, ts, http.MethodGet, "/categories/signs/memorize?locale=uz-Latn", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", status)
	}
}

func TestCategoryMemorizeRequiresVIP(t *testing.T) {
	ts, tok, _ := setupServer(t)
	status, env := doReq(t, ts, http.MethodGet, "/categories/signs/memorize?locale=uz-Latn", tok, nil)
	if status != http.StatusPaymentRequired || env.Error == nil || env.Error.Code != "vip_required" {
		t.Fatalf("status=%d env=%+v want 402 vip_required", status, env)
	}
}

func TestCategoryMemorizeOverHTTP(t *testing.T) {
	ts, tok, q := setupServer(t)
	profile, err := q.GetProfileByPhone(context.Background(), "+998901234567")
	if err != nil {
		t.Fatal(err)
	}
	grantVIP(t, q, profile.ID)

	catID, err := q.GetCategoryIDByCode(context.Background(), "signs")
	if err != nil {
		t.Fatal(err)
	}
	total, err := q.CountValidQuestionsInCategory(context.Background(), catID)
	if err != nil {
		t.Fatal(err)
	}

	status, env := doReq(t, ts, http.MethodGet, "/categories/signs/memorize?locale=uz-Latn", tok, nil)
	if status != http.StatusOK {
		t.Fatalf("status=%d env=%+v", status, env)
	}
	var items []struct {
		ID              string `json:"id"`
		Position        int    `json:"position"`
		Answered        bool   `json:"answered"`
		CorrectAnswerID string `json:"correct_answer_id"`
		Answers         []struct {
			ID string `json:"id"`
		} `json:"answers"`
	}
	if err := json.Unmarshal(env.Data, &items); err != nil {
		t.Fatalf("json: %v data=%s", err, env.Data)
	}
	if len(items) != int(total) {
		t.Fatalf("len(items)=%d want %d", len(items), total)
	}
	for i, item := range items {
		if item.Position != i+1 {
			t.Fatalf("items[%d].Position=%d want %d", i, item.Position, i+1)
		}
		if !item.Answered {
			t.Fatalf("items[%d].Answered=false want true", i)
		}
		if item.CorrectAnswerID == "" {
			t.Fatalf("items[%d] missing correct_answer_id", i)
		}
		found := false
		for _, a := range item.Answers {
			if a.ID == item.CorrectAnswerID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("items[%d].CorrectAnswerID %s not among its own answers", i, item.CorrectAnswerID)
		}
	}
}

func TestCategoryMemorizeBogusCategoryCode(t *testing.T) {
	ts, tok, q := setupServer(t)
	profile, err := q.GetProfileByPhone(context.Background(), "+998901234567")
	if err != nil {
		t.Fatal(err)
	}
	grantVIP(t, q, profile.ID)

	status, env := doReq(t, ts, http.MethodGet, "/categories/does-not-exist/memorize?locale=uz-Latn", tok, nil)
	if status != http.StatusNotFound || env.Error == nil || env.Error.Code != "not_found" {
		t.Fatalf("status=%d env=%+v want 404 not_found", status, env)
	}
}

func TestCategoryMemorizeInvalidLocale(t *testing.T) {
	ts, tok, q := setupServer(t)
	profile, err := q.GetProfileByPhone(context.Background(), "+998901234567")
	if err != nil {
		t.Fatal(err)
	}
	grantVIP(t, q, profile.ID)

	status, env := doReq(t, ts, http.MethodGet, "/categories/signs/memorize?locale=nope", tok, nil)
	if status != http.StatusBadRequest || env.Error == nil || env.Error.Code != "invalid_locale" {
		t.Fatalf("status=%d env=%+v want 400 invalid_locale", status, env)
	}
}
