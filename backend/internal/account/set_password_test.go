package account_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/db/sqlc"
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

func setPasswordBody(pass string) []byte {
	body, _ := json.Marshal(map[string]string{"new_password": pass, "confirm_password": pass})
	return body
}

// telegramAccount is a learner made through Telegram: no password yet.
func telegramAccount(t *testing.T, pool *pgxpool.Pool, phone string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `INSERT INTO profile (phone, name) VALUES ($1, 'Tg') RETURNING id`, phone).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// session signs the profile in once more: an access token plus a stored
// refresh token, as any sign-in leaves behind.
func session(t *testing.T, _ *auth.Service, q *sqlc.Queries, id uuid.UUID) auth.Tokens {
	t.Helper()
	access, err := auth.IssueAccess([]byte(testSecret), id, "user", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	refresh, err := auth.NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := q.CreateRefreshToken(context.Background(), sqlc.CreateRefreshTokenParams{
		ProfileID: id, TokenHash: auth.HashToken(refresh),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	return auth.Tokens{Access: access, Refresh: refresh}
}

func doSetPassword(t *testing.T, ts *httptest.Server, url, access, refresh string, body []byte) (int, respEnvelope) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url+"/me/password/set", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	if refresh != "" {
		req.Header.Set("X-Avtotest-Refresh-Token", refresh)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var env respEnvelope
	_ = json.NewDecoder(resp.Body).Decode(&env)
	return resp.StatusCode, env
}

// Audit I4: a first password is a new way into the account. Whoever sets it
// keeps their own session; every other session of the profile ends, and the
// linked Telegram chat is told — a phished session must not coexist with the
// owner unnoticed and then outlive a logout as a password.
func TestSetFirstPasswordEndsOtherSessionsAndNotifies(t *testing.T) {
	ts, svc, q, pool := setupPasswordServer(t)
	ctx := context.Background()
	id := telegramAccount(t, pool, "+998901210010")
	mine := session(t, svc, q, id)
	other := session(t, svc, q, id)
	stranger := registerLearner(t, svc, "901210011", "strangerpw1")

	// A refused attempt changes nothing.
	if status, _ := doSetPassword(t, ts, ts.URL, mine.Access, mine.Refresh, setPasswordBody("short")); status != http.StatusBadRequest {
		t.Fatalf("weak status=%d", status)
	}
	if len(lastPasswordNotices.all()) != 0 {
		t.Fatal("notified for a refused attempt")
	}

	status, env := doSetPassword(t, ts, ts.URL, mine.Access, mine.Refresh, setPasswordBody("goodpass12"))
	if status != http.StatusOK {
		t.Fatalf("set = %d %+v", status, env.Error)
	}
	if got := lastPasswordNotices.all(); len(got) != 1 || got[0] != id {
		t.Fatalf("notices = %v, want one for %s", got, id)
	}
	if _, err := svc.Refresh(ctx, other.Refresh); err == nil {
		t.Fatal("another session of the profile survived the first password")
	}
	// Ended, not "revoked": a revoked token presented later counts as reuse
	// and would sign the keeper out as well.
	if _, err := svc.Refresh(ctx, mine.Refresh); err != nil {
		t.Fatalf("the caller's own session was ended: %v", err)
	}
	if _, err := svc.Refresh(ctx, stranger.Refresh); err != nil {
		t.Fatalf("another profile's session was touched: %v", err)
	}
}

// Without the caller's refresh token (a client that does not send it) every
// session ends: safe, at the cost of one sign-in.
func TestSetFirstPasswordWithoutRefreshTokenEndsEverySession(t *testing.T) {
	ts, svc, q, pool := setupPasswordServer(t)
	ctx := context.Background()
	id := telegramAccount(t, pool, "+998901210020")
	mine := session(t, svc, q, id)
	// Somebody else's refresh token keeps nothing of this profile.
	stranger := registerLearner(t, svc, "901210021", "strangerpw1")
	if status, env := doSetPassword(t, ts, ts.URL, mine.Access, stranger.Refresh, setPasswordBody("goodpass12")); status != http.StatusOK {
		t.Fatalf("set = %d %+v", status, env.Error)
	}
	if _, err := svc.Refresh(ctx, mine.Refresh); err == nil {
		t.Fatal("a session survived without its refresh token being named")
	}
	if _, err := svc.Refresh(ctx, stranger.Refresh); err != nil {
		t.Fatalf("the stranger's session was touched: %v", err)
	}
}

func TestSetFirstPasswordIsRateLimitedPerProfile(t *testing.T) {
	ts, svc, q, pool := setupPasswordServer(t)
	id := telegramAccount(t, pool, "+998901210030")
	mine := session(t, svc, q, id)
	for i := 0; i < 5; i++ {
		if status, env := doSetPassword(t, ts, ts.URL, mine.Access, mine.Refresh, setPasswordBody("short")); status != http.StatusBadRequest {
			t.Fatalf("attempt %d = %d %+v", i, status, env.Error)
		}
	}
	status, env := doSetPassword(t, ts, ts.URL, mine.Access, mine.Refresh, setPasswordBody("goodpass12"))
	if status != http.StatusTooManyRequests || env.Error == nil || env.Error.Code != "rate_limited" {
		t.Fatalf("6th attempt = %d %+v", status, env.Error)
	}
	// Another learner is not affected.
	other := telegramAccount(t, pool, "+998901210031")
	if status, _ := doSetPassword(t, ts, ts.URL, session(t, svc, q, other).Access, "", setPasswordBody("goodpass12")); status != http.StatusOK {
		t.Fatalf("other profile status=%d", status)
	}
}

// A B2B station's token must not give its shadow profile a password — refused
// by the handler and, independently, by the query.
func TestSetFirstPasswordRefusesAStation(t *testing.T) {
	ts, _, q, pool := setupPasswordServer(t)
	ctx := context.Background()
	var profileID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO profile (phone, kind) VALUES ($1, 'station') RETURNING id`, "st:"+uuid.NewString()).Scan(&profileID); err != nil {
		t.Fatal(err)
	}
	access, err := auth.IssueStationAccess([]byte(testSecret), uuid.New(), profileID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	status, env := doSetPassword(t, ts, ts.URL, access, "", setPasswordBody("goodpass12"))
	if status != http.StatusForbidden || env.Error == nil || env.Error.Code != "forbidden" {
		t.Fatalf("station set = %d %+v", status, env.Error)
	}
	if _, err := q.SetProfilePasswordIfUnset(ctx, sqlc.SetProfilePasswordIfUnsetParams{
		ID: profileID, PasswordHash: pgtype.Text{String: "$2a$10$notarealhashnotarealhashnotarealhashnotarealhashnotare", Valid: true},
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("query on a station row: err = %v, want no rows", err)
	}
	var hasPassword bool
	if err := pool.QueryRow(ctx, `SELECT password_hash IS NOT NULL FROM profile WHERE id = $1`, profileID).Scan(&hasPassword); err != nil || hasPassword {
		t.Fatalf("station row has a password: %v %v", hasPassword, err)
	}
	if len(lastPasswordNotices.all()) != 0 {
		t.Fatal("notified for a station")
	}
}
