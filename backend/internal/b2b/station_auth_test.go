package b2b_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/b2b"
	"avtotest.uz/backend/internal/testdb"
	"avtotest.uz/backend/internal/testredis"
)

func TestStationAuthHappyPath(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	rdb := testredis.New(t)
	store := b2b.Store{Pool: pool}
	secret := []byte("test-secret-that-is-long-enough-000000")
	sa := b2b.StationAuth{Pool: pool, Redis: rdb, Secret: secret}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 3)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const hwid = "aa11bb22cc33dd44ee55ff6677889900aa11bb22cc33dd44ee55ff6677889900"
	res, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: pub, HWIDHash: hwid, Label: "PC-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	ch, err := sa.Challenge(ctx, res.StationID)
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Now().Unix()
	sig := ed25519.Sign(priv, b2b.SignedMessage(res.StationID, ch.Nonce, ts))

	tok, err := sa.Token(ctx, b2b.TokenInput{
		StationID: res.StationID, Nonce: ch.Nonce, TS: ts, Sig: sig, HWIDHash: hwid,
	})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := auth.ParseAccess(secret, tok.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.StationID != res.StationID || claims.ProfileID != res.ProfileID {
		t.Fatalf("claims=%+v, want station=%v profile=%v", claims, res.StationID, res.ProfileID)
	}

	// The nonce is single use.
	sig2 := ed25519.Sign(priv, b2b.SignedMessage(res.StationID, ch.Nonce, ts))
	if _, err := sa.Token(ctx, b2b.TokenInput{
		StationID: res.StationID, Nonce: ch.Nonce, TS: ts, Sig: sig2, HWIDHash: hwid,
	}); !errors.Is(err, b2b.ErrStationAuth) {
		t.Fatalf("replayed nonce err=%v, want ErrStationAuth", err)
	}
}

func TestStationAuthRejects(t *testing.T) {
	pool := testdb.New(t)
	testdb.Truncate(t, pool)
	rdb := testredis.New(t)
	store := b2b.Store{Pool: pool}
	sa := b2b.StationAuth{Pool: pool, Redis: rdb, Secret: []byte("test-secret-that-is-long-enough-000000")}
	ctx := context.Background()

	orgID := seatedOrg(t, pool, 3)
	code, err := store.OpenEnrollWindow(ctx, orgID, time.Hour, "test")
	if err != nil {
		t.Fatal(err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const hwid = "aa11bb22cc33dd44ee55ff6677889900aa11bb22cc33dd44ee55ff6677889900"
	res, err := store.EnrollStation(ctx, b2b.EnrollInput{
		Code: code.Code, PublicKey: pub, HWIDHash: hwid, Label: "PC-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	fresh := func(t *testing.T) (string, int64) {
		t.Helper()
		ch, err := sa.Challenge(ctx, res.StationID)
		if err != nil {
			t.Fatal(err)
		}
		return ch.Nonce, time.Now().Unix()
	}

	t.Run("wrong key", func(t *testing.T) {
		nonce, ts := fresh(t)
		_, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
		sig := ed25519.Sign(otherPriv, b2b.SignedMessage(res.StationID, nonce, ts))
		if _, err := sa.Token(ctx, b2b.TokenInput{
			StationID: res.StationID, Nonce: nonce, TS: ts, Sig: sig, HWIDHash: hwid,
		}); !errors.Is(err, b2b.ErrStationAuth) {
			t.Fatalf("err=%v, want ErrStationAuth", err)
		}
	})

	t.Run("hwid mismatch", func(t *testing.T) {
		nonce, ts := fresh(t)
		sig := ed25519.Sign(priv, b2b.SignedMessage(res.StationID, nonce, ts))
		if _, err := sa.Token(ctx, b2b.TokenInput{
			StationID: res.StationID, Nonce: nonce, TS: ts, Sig: sig,
			HWIDHash: "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
		}); !errors.Is(err, b2b.ErrStationAuth) {
			t.Fatalf("err=%v, want ErrStationAuth", err)
		}
	})

	t.Run("unknown nonce", func(t *testing.T) {
		ts := time.Now().Unix()
		sig := ed25519.Sign(priv, b2b.SignedMessage(res.StationID, "not-a-nonce", ts))
		if _, err := sa.Token(ctx, b2b.TokenInput{
			StationID: res.StationID, Nonce: "not-a-nonce", TS: ts, Sig: sig, HWIDHash: hwid,
		}); !errors.Is(err, b2b.ErrStationAuth) {
			t.Fatalf("err=%v, want ErrStationAuth", err)
		}
	})

	t.Run("stale timestamp", func(t *testing.T) {
		nonce, _ := fresh(t)
		ts := time.Now().Add(-10 * time.Minute).Unix()
		sig := ed25519.Sign(priv, b2b.SignedMessage(res.StationID, nonce, ts))
		if _, err := sa.Token(ctx, b2b.TokenInput{
			StationID: res.StationID, Nonce: nonce, TS: ts, Sig: sig, HWIDHash: hwid,
		}); !errors.Is(err, b2b.ErrStationAuth) {
			t.Fatalf("err=%v, want ErrStationAuth", err)
		}
	})

	t.Run("nonce bound to a different station", func(t *testing.T) {
		// A second station under the same org, so the only variable is which
		// station the nonce (and its signed bytes) were issued for.
		otherPub, otherPriv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		otherHWID := testHWID("station-auth-cross-station-other")
		otherRes, err := store.EnrollStation(ctx, b2b.EnrollInput{
			Code: code.Code, PublicKey: otherPub, HWIDHash: otherHWID, Label: "PC-2",
		})
		if err != nil {
			t.Fatal(err)
		}

		// Nonce issued for the original station...
		nonce, ts := fresh(t)
		// ...but the credential is otherwise fully valid for the other
		// station: signed by its own key, over its own station id, with its
		// own hwid. The only thing wrong is that the nonce was issued for a
		// different station, so only the Redis nonce-key binding (which
		// embeds the station id) can be what rejects this.
		sig := ed25519.Sign(otherPriv, b2b.SignedMessage(otherRes.StationID, nonce, ts))
		if _, err := sa.Token(ctx, b2b.TokenInput{
			StationID: otherRes.StationID, Nonce: nonce, TS: ts, Sig: sig, HWIDHash: otherHWID,
		}); !errors.Is(err, b2b.ErrStationAuth) {
			t.Fatalf("err=%v, want ErrStationAuth", err)
		}
	})

	t.Run("suspended org", func(t *testing.T) {
		nonce, ts := fresh(t)
		sig := ed25519.Sign(priv, b2b.SignedMessage(res.StationID, nonce, ts))
		if err := store.SetOrgStatus(ctx, orgID, "suspended"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := store.SetOrgStatus(ctx, orgID, "active"); err != nil {
				t.Errorf("restore org status: %v", err)
			}
		})
		if _, err := sa.Token(ctx, b2b.TokenInput{
			StationID: res.StationID, Nonce: nonce, TS: ts, Sig: sig, HWIDHash: hwid,
		}); !errors.Is(err, b2b.ErrStationAuth) {
			t.Fatalf("err=%v, want ErrStationAuth", err)
		}
	})

	t.Run("expired licence", func(t *testing.T) {
		nonce, ts := fresh(t)
		sig := ed25519.Sign(priv, b2b.SignedMessage(res.StationID, nonce, ts))

		// Capture the licence window as it stood before this subtest so it can
		// be restored afterward: later subtests (e.g. "revoked station") share
		// this org's licence row and must not see it as expired.
		var origStarts, origEnds time.Time
		if err := pool.QueryRow(ctx,
			`SELECT starts_at, ends_at FROM b2b_org_license WHERE org_id = $1`, orgID,
		).Scan(&origStarts, &origEnds); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := pool.Exec(ctx,
				`UPDATE b2b_org_license SET starts_at = $2, ends_at = $3 WHERE org_id = $1`,
				orgID, origStarts, origEnds); err != nil {
				t.Errorf("restore licence window: %v", err)
			}
		})

		// b2b_org_license has a CHECK (ends_at > starts_at), so starts_at must
		// move back too or this violates that constraint.
		if _, err := pool.Exec(ctx,
			`UPDATE b2b_org_license SET starts_at = now() - interval '2 days', ends_at = now() - interval '1 day' WHERE org_id = $1`, orgID); err != nil {
			t.Fatal(err)
		}
		if _, err := sa.Token(ctx, b2b.TokenInput{
			StationID: res.StationID, Nonce: nonce, TS: ts, Sig: sig, HWIDHash: hwid,
		}); !errors.Is(err, b2b.ErrStationAuth) {
			t.Fatalf("err=%v, want ErrStationAuth", err)
		}
	})

	t.Run("revoked station", func(t *testing.T) {
		if err := store.RevokeStation(ctx, orgID, res.StationID); err != nil {
			t.Fatal(err)
		}
		nonce, ts := fresh(t)
		sig := ed25519.Sign(priv, b2b.SignedMessage(res.StationID, nonce, ts))
		if _, err := sa.Token(ctx, b2b.TokenInput{
			StationID: res.StationID, Nonce: nonce, TS: ts, Sig: sig, HWIDHash: hwid,
		}); !errors.Is(err, b2b.ErrStationAuth) {
			t.Fatalf("err=%v, want ErrStationAuth", err)
		}
	})
}
