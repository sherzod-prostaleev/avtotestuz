package account_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
)

func hasPasswordOf(t *testing.T, env respEnvelope) bool {
	t.Helper()
	var me struct {
		Profile struct {
			HasPassword *bool `json:"has_password"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(env.Data, &me); err != nil || me.Profile.HasPassword == nil {
		t.Fatalf("GET /me has no has_password: %s %v", env.Data, err)
	}
	return *me.Profile.HasPassword
}

// An account created through Telegram has no password; the learner can set a
// first one with just the new password, once.
func TestSetFirstPassword(t *testing.T) {
	ts, svc, _, pool := setupPasswordServer(t)
	ctx := context.Background()
	var id uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO profile (phone, name) VALUES ('+998901210001', 'Tg') RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	access, err := auth.IssueAccess([]byte(testSecret), id, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	_, env := doReq(t, ts, http.MethodGet, "/me", access, nil)
	if hasPasswordOf(t, env) {
		t.Fatal("passwordless account reported has_password")
	}

	post := func(newPass, confirm string) (int, respEnvelope) {
		body, _ := json.Marshal(map[string]string{"new_password": newPass, "confirm_password": confirm})
		return doReq(t, ts, http.MethodPost, "/me/password/set", access, body)
	}
	if status, env := post("short", "short"); status != http.StatusBadRequest || env.Error.Code != "weak_password" {
		t.Fatalf("weak = %d %+v", status, env.Error)
	}
	if status, env := post("goodpass12", "goodpass13"); status != http.StatusBadRequest || env.Error.Code != "password_mismatch" {
		t.Fatalf("mismatch = %d %+v", status, env.Error)
	}
	if status, env := post("goodpass12", "goodpass12"); status != http.StatusOK {
		t.Fatalf("set = %d %+v", status, env.Error)
	}
	_, env = doReq(t, ts, http.MethodGet, "/me", access, nil)
	if !hasPasswordOf(t, env) {
		t.Fatal("has_password still false")
	}
	if _, err := svc.Login(ctx, auth.LoginInput{Phone: "901210001", Password: "goodpass12"}); err != nil {
		t.Fatalf("login with the new password: %v", err)
	}
	// Once a password exists, only the change flow (current password) may replace it.
	if status, env := post("otherpass12", "otherpass12"); status != http.StatusConflict || env.Error.Code != "password_already_set" {
		t.Fatalf("second set = %d %+v", status, env.Error)
	}
}

func TestSetFirstPasswordRefusedWhenPasswordExists(t *testing.T) {
	ts, svc, _, _ := setupPasswordServer(t)
	reg := registerLearner(t, svc, "901210002", "oldpass12")
	body, _ := json.Marshal(map[string]string{"new_password": "newpass12", "confirm_password": "newpass12"})
	status, env := doReq(t, ts, http.MethodPost, "/me/password/set", reg.Access, body)
	if status != http.StatusConflict || env.Error.Code != "password_already_set" {
		t.Fatalf("status=%d %+v", status, env.Error)
	}
	if _, err := svc.Login(context.Background(), auth.LoginInput{Phone: "901210002", Password: "oldpass12"}); err != nil {
		t.Fatal("old password must still work")
	}
	_, env = doReq(t, ts, http.MethodGet, "/me", reg.Access, nil)
	if !hasPasswordOf(t, env) {
		t.Fatal("password account must report has_password")
	}
}
