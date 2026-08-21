package agent_test

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"avtotest.uz/station/internal/agent"
	"avtotest.uz/station/internal/keystore"
)

func TestEnrollThenTokenSignsTheChallenge(t *testing.T) {
	dir := t.TempDir()
	const hwid = "aa11bb22cc33dd44ee55ff6677889900aa11bb22cc33dd44ee55ff6677889900"
	const stationID = "11111111-2222-3333-4444-555555555555"
	const nonce = "test-nonce"

	var pub ed25519.PublicKey
	var verified bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/b2b/stations/enroll":
			var body struct {
				PublicKey string `json:"public_key"`
				HWIDHash  string `json:"hwid_hash"`
				Code      string `json:"code"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.HWIDHash != hwid || body.Code != "AVTO-TEST-CODE" {
				t.Errorf("bad enroll body: %+v", body)
			}
			raw, err := base64.StdEncoding.DecodeString(body.PublicKey)
			if err != nil {
				t.Errorf("public_key not base64: %v", err)
			}
			pub = ed25519.PublicKey(raw)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"station_id": stationID, "org_id": stationID, "label": "PC-1"},
			})
		case "/api/v1/b2b/stations/challenge":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"nonce": nonce, "expires_in": 60},
			})
		case "/api/v1/b2b/stations/token":
			var body struct {
				StationID string `json:"station_id"`
				Nonce     string `json:"nonce"`
				TS        int64  `json:"ts"`
				Sig       string `json:"sig"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			sig, err := base64.StdEncoding.DecodeString(body.Sig)
			if err != nil {
				t.Errorf("sig not base64: %v", err)
			}
			msg := []byte("avtotest-station-v1|" + body.StationID + "|" + body.Nonce + "|" + strconv.FormatInt(body.TS, 10))
			verified = ed25519.Verify(pub, msg, sig)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"access_token": "tok-abc", "expires_in": 900},
			})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	ks, err := keystore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := &agent.Agent{
		APIBase:  srv.URL,
		StateDir: dir,
		Keys:     ks,
		HWID:     hwid,
		Version:  "test",
	}

	ctx := context.Background()
	if err := a.Enroll(ctx, "AVTO-TEST-CODE", "PC-1", "Test Avtomaktab"); err != nil {
		t.Fatal(err)
	}
	tok, err := a.Token(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if tok != "tok-abc" {
		t.Fatalf("token=%q, want tok-abc", tok)
	}
	if !verified {
		t.Fatal("server could not verify the agent's signature")
	}

	// A second call inside the TTL reuses the cached token: no new challenge.
	if _, err := a.Token(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestAgentSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	ks, err := keystore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	priv, err := ks.Load()
	if err != nil {
		t.Fatal(err)
	}
	ks2, err := keystore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	priv2, err := ks2.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !priv.Equal(priv2) {
		t.Fatal("keystore must return the same key across restarts")
	}
}
