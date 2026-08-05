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
