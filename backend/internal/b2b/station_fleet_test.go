package b2b_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/stationctx"
	"avtotest.uz/backend/internal/testdb"
	"avtotest.uz/backend/internal/testredis"
)

// fleetPC is one classroom machine's identity for the duration of the test.
type fleetPC struct {
	label     string
	pub       ed25519.PublicKey
	priv      ed25519.PrivateKey
	stationID uuid.UUID
}

// TestAClonedClassroomOfFiftyFiveComesUpWhole is the end-to-end rehearsal of
// Avtomotohavaskorlar BUXORO, run against the real HTTP handlers, the real
// Redis rate limiter and the real seat accounting.
//
// The school's 55 PCs were imaged from a handful of master disks, so they all
// present the same MachineGuid and therefore the same hwid_hash. This test
// takes the worst case -- every single PC a clone of one image -- and asserts
// the three things that failed on 2026-08-26:
//
//   - all 55 enroll, none refused;
//   - all 55 are still active afterwards, i.e. no PC was revoked by the PC
//     installed after it;
//   - all 55 report VIP, which is what a station actually needs. A revoked
//     station falls back to the free tier's 30 practice questions a day for
//     the whole classroom, which is what the school saw as "premium stopped
//     working on some computers".
//
// The enrollments run concurrently because that is how a lab behaves when the
// room is switched on: they contend on the same org row lock and the same
// enrollment code counter.
func TestAClonedClassroomOfFiftyFiveComesUpWhole(t *testing.T) {
	const seats = 55

	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	rdb := testredis.New(t)
	ctx := context.Background()
	secret := []byte("test-secret-that-is-long-enough-000000")

	store := b2b.Store{Pool: pool}
	orgID := seatedOrg(t, pool, seats)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}

	h := &b2b.Handler{Pool: pool, Redis: rdb, Secret: secret, Lim: auth.Limiter{R: rdb}}
	r := chi.NewRouter()
	r.Route("/api/v1", h.PublicRoutes)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	post := func(path string, body any) (int, map[string]any, error) {
		buf, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		resp, err := http.Post(srv.URL+path, "application/json", bytes.NewReader(buf))
		if err != nil {
			return 0, nil, err
		}
		defer func() { _ = resp.Body.Close() }()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return resp.StatusCode, nil, err
		}
		var out map[string]any
		_ = json.Unmarshal(raw, &out)
		if out == nil {
			return resp.StatusCode, nil, fmt.Errorf("non-JSON body: %s", raw)
		}
		return resp.StatusCode, out, nil
	}

	// One master image for the whole room: the exact condition that made 37 of
	// the school's stations revoke each other.
	clonedHWID := testHWID("HPStoreBukhara-master-image")

	fleet := make([]*fleetPC, seats)
	for i := range fleet {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		fleet[i] = &fleetPC{label: "HPStoreBukhara", pub: pub, priv: priv}
	}

	// --- the room is switched on -------------------------------------------
	var wg sync.WaitGroup
	errs := make([]error, seats)
	for i, pc := range fleet {
		wg.Add(1)
		go func(i int, pc *fleetPC) {
			defer wg.Done()
			status, body, err := post("/api/v1/b2b/stations/enroll", map[string]any{
				"code":          code.Code,
				"public_key":    base64.StdEncoding.EncodeToString(pc.pub),
				"hwid_hash":     clonedHWID,
				"label":         pc.label,
				"agent_version": "1.3.0",
			})
			if err != nil {
				errs[i] = err
				return
			}
			if status != http.StatusOK {
				errs[i] = fmt.Errorf("enroll status=%d body=%v", status, body)
				return
			}
			data, _ := body["data"].(map[string]any)
			raw, _ := data["station_id"].(string)
			id, parseErr := uuid.Parse(raw)
			if parseErr != nil {
				errs[i] = fmt.Errorf("station_id %q: %w", raw, parseErr)
				return
			}
			pc.stationID = id
		}(i, pc)
	}
	wg.Wait()

	var failed int
	for i, err := range errs {
		if err != nil {
			failed++
			if failed <= 3 {
				t.Errorf("PC %d/%d failed to enroll: %v", i+1, seats, err)
			}
		}
	}
	if failed > 0 {
		t.Fatalf("%d of %d PCs could not enroll", failed, seats)
	}

	// Distinct rows, not one row handed out 55 times.
	seen := make(map[uuid.UUID]bool, seats)
	for i, pc := range fleet {
		if seen[pc.stationID] {
			t.Fatalf("PC %d reused station id %s", i+1, pc.stationID)
		}
		seen[pc.stationID] = true
	}

	// --- nobody was evicted by a classmate ---------------------------------
	active, err := store.CountActiveStations(ctx, orgID)
	if err != nil {
		t.Fatal(err)
	}
	if active != seats {
		var revoked int
		if err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM b2b_station WHERE org_id = $1 AND status = 'revoked'`,
			orgID).Scan(&revoked); err != nil {
			t.Fatal(err)
		}
		t.Fatalf("active=%d revoked=%d, want all %d active: a PC was revoked by the one installed after it",
			active, revoked, seats)
	}

	// --- every PC can authenticate, and every PC is VIP ---------------------
	bill := billing.Service{Q: sqlc.New(pool), StationVIP: store}
	for i, pc := range fleet {
		status, body, err := post("/api/v1/b2b/stations/challenge", map[string]any{
			"station_id": pc.stationID.String(),
		})
		if err != nil || status != http.StatusOK {
			t.Fatalf("PC %d/%d challenge: status=%d err=%v body=%v", i+1, seats, status, err, body)
		}
		data, _ := body["data"].(map[string]any)
		nonce, _ := data["nonce"].(string)
		if nonce == "" {
			t.Fatalf("PC %d/%d: no nonce in %v", i+1, seats, body)
		}

		ts := time.Now().Unix()
		sig := ed25519.Sign(pc.priv, b2b.SignedMessage(pc.stationID, nonce, ts))
		status, body, err = post("/api/v1/b2b/stations/token", map[string]any{
			"station_id":    pc.stationID.String(),
			"nonce":         nonce,
			"ts":            ts,
			"sig":           base64.StdEncoding.EncodeToString(sig),
			"hwid_hash":     clonedHWID,
			"agent_version": "1.3.0",
		})
		if err != nil || status != http.StatusOK {
			t.Fatalf("PC %d/%d token: status=%d err=%v body=%v", i+1, seats, status, err, body)
		}
		data, _ = body["data"].(map[string]any)
		if tok, _ := data["access_token"].(string); tok == "" {
			t.Fatalf("PC %d/%d: no access_token in %v", i+1, seats, body)
		}

		var profileID uuid.UUID
		if err := pool.QueryRow(ctx,
			`SELECT station_profile_id FROM b2b_station WHERE id = $1`, pc.stationID).Scan(&profileID); err != nil {
			t.Fatal(err)
		}
		vip, until, err := bill.Status(stationctx.WithContext(ctx, pc.stationID), profileID)
		if err != nil {
			t.Fatalf("PC %d/%d VIP lookup: %v", i+1, seats, err)
		}
		if !vip || until == nil {
			t.Fatalf("PC %d/%d has no VIP (active=%v until=%v) -- this classroom is back on the free tier's 30 questions a day",
				i+1, seats, vip, until)
		}
	}

	// --- the 56th PC is refused, and refusing it costs nobody their seat ----
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	status, body, err := post("/api/v1/b2b/stations/enroll", map[string]any{
		"code":          code.Code,
		"public_key":    base64.StdEncoding.EncodeToString(pub),
		"hwid_hash":     clonedHWID,
		"label":         "HPStoreBukhara",
		"agent_version": "1.3.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if status == http.StatusOK {
		t.Fatalf("a 56th PC was admitted onto a %d-seat licence: %v", seats, body)
	}
	active, err = store.CountActiveStations(ctx, orgID)
	if err != nil {
		t.Fatal(err)
	}
	if active != seats {
		t.Fatalf("active=%d after the refusal, want %d: refusing a new PC must not evict a live one", active, seats)
	}
}
