package bot

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func newTestLinkService(t *testing.T) (*LinkService, *sqlc.Queries) {
	t.Helper()
	pool := testdb.New(t)
	q := sqlc.New(pool)
	return NewLinkService(pool, q), q
}

func createProfile(t *testing.T, q *sqlc.Queries, phone string) uuid.UUID {
	t.Helper()
	p, err := q.CreateProfile(context.Background(), sqlc.CreateProfileParams{Phone: phone})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return p.ID
}

func TestGenerateLinkToken_ReturnsUnexpiredToken(t *testing.T) {
	svc, q := newTestLinkService(t)
	ctx := context.Background()
	profileID := createProfile(t, q, "+998901110001")

	tok, err := svc.GenerateLinkToken(ctx, profileID)
	if err != nil {
		t.Fatalf("GenerateLinkToken: %v", err)
	}
	if tok.Token == "" {
		t.Fatal("want non-empty token")
	}
	if !tok.ExpiresAt.After(time.Now()) {
		t.Fatalf("ExpiresAt = %v, want in the future", tok.ExpiresAt)
	}
	if !tok.ExpiresAt.Before(time.Now().Add(LinkTokenTTL + time.Minute)) {
		t.Fatalf("ExpiresAt = %v, too far in the future", tok.ExpiresAt)
	}
}

func TestGenerateLinkToken_InvalidatesPriorUnusedToken(t *testing.T) {
	svc, q := newTestLinkService(t)
	ctx := context.Background()
	profileID := createProfile(t, q, "+998901110002")

	first, err := svc.GenerateLinkToken(ctx, profileID)
	if err != nil {
		t.Fatalf("first GenerateLinkToken: %v", err)
	}
	if _, err := svc.GenerateLinkToken(ctx, profileID); err != nil {
		t.Fatalf("second GenerateLinkToken: %v", err)
	}

	_, err = svc.RedeemLinkToken(ctx, first.Token, 111, "alice")
	if !errors.Is(err, ErrLinkTokenNotFound) {
		t.Fatalf("redeeming an invalidated token: err = %v, want ErrLinkTokenNotFound", err)
	}
}

func TestRedeemLinkToken_Success_NewBinding(t *testing.T) {
	svc, q := newTestLinkService(t)
	ctx := context.Background()
	profileID := createProfile(t, q, "+998901110003")

	tok, err := svc.GenerateLinkToken(ctx, profileID)
	if err != nil {
		t.Fatalf("GenerateLinkToken: %v", err)
	}

	res, err := svc.RedeemLinkToken(ctx, tok.Token, 555, "bob")
	if err != nil {
		t.Fatalf("RedeemLinkToken: %v", err)
	}
	if res.ProfileID != profileID {
		t.Fatalf("ProfileID = %v, want %v", res.ProfileID, profileID)
	}
	if res.AlreadyLinked {
		t.Fatal("AlreadyLinked = true, want false for a brand new binding")
	}

	account, err := q.GetTelegramAccountByTgUserID(ctx, 555)
	if err != nil {
		t.Fatalf("GetTelegramAccountByTgUserID: %v", err)
	}
	if account.ProfileID != profileID || account.Username != "bob" {
		t.Fatalf("unexpected binding: %+v", account)
	}
}

func TestRedeemLinkToken_ExpiredRejected(t *testing.T) {
	svc, q := newTestLinkService(t)
	ctx := context.Background()
	profileID := createProfile(t, q, "+998901110004")

	raw := "expired-token-raw-value"
	if _, err := q.CreateLinkToken(ctx, sqlc.CreateLinkTokenParams{
		ProfileID: profileID,
		TokenHash: hashToken(raw),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Minute), Valid: true},
	}); err != nil {
		t.Fatalf("seed expired token: %v", err)
	}

	_, err := svc.RedeemLinkToken(ctx, raw, 666, "carol")
	if !errors.Is(err, ErrLinkTokenExpired) {
		t.Fatalf("err = %v, want ErrLinkTokenExpired", err)
	}
	if _, err := q.GetTelegramAccountByTgUserID(ctx, 666); err == nil {
		t.Fatal("expired-token redeem must not create a binding")
	}
}

func TestRedeemLinkToken_ReuseRejected(t *testing.T) {
	svc, q := newTestLinkService(t)
	ctx := context.Background()
	profileID := createProfile(t, q, "+998901110005")

	tok, err := svc.GenerateLinkToken(ctx, profileID)
	if err != nil {
		t.Fatalf("GenerateLinkToken: %v", err)
	}
	if _, err := svc.RedeemLinkToken(ctx, tok.Token, 777, "dave"); err != nil {
		t.Fatalf("first redeem: %v", err)
	}

	_, err = svc.RedeemLinkToken(ctx, tok.Token, 888, "eve")
	if !errors.Is(err, ErrLinkTokenAlreadyUsed) {
		t.Fatalf("second redeem err = %v, want ErrLinkTokenAlreadyUsed", err)
	}

	// The first, successful binding must be untouched by the rejected reuse.
	account, err := q.GetTelegramAccountByTgUserID(ctx, 777)
	if err != nil {
		t.Fatalf("GetTelegramAccountByTgUserID(777): %v", err)
	}
	if account.ProfileID != profileID {
		t.Fatalf("original binding changed: %+v", account)
	}
	if _, err := q.GetTelegramAccountByTgUserID(ctx, 888); err == nil {
		t.Fatal("reused token must not create a second binding")
	}
}

func TestRedeemLinkToken_UnknownTokenRejected(t *testing.T) {
	svc, _ := newTestLinkService(t)
	ctx := context.Background()

	_, err := svc.RedeemLinkToken(ctx, "this-token-was-never-issued", 999, "mallory")
	if !errors.Is(err, ErrLinkTokenNotFound) {
		t.Fatalf("err = %v, want ErrLinkTokenNotFound", err)
	}
}

func TestRedeemLinkToken_SelfRelinkIsIdempotent(t *testing.T) {
	svc, q := newTestLinkService(t)
	ctx := context.Background()
	profileID := createProfile(t, q, "+998901110006")

	first, err := svc.GenerateLinkToken(ctx, profileID)
	if err != nil {
		t.Fatalf("first GenerateLinkToken: %v", err)
	}
	if _, err := svc.RedeemLinkToken(ctx, first.Token, 1234, "frank"); err != nil {
		t.Fatalf("first redeem: %v", err)
	}

	// Same profile, same Telegram account, a fresh token (e.g. the user
	// re-opened a stale deep link, or generated a new one by mistake) — must
	// succeed, not be treated as a hijack of its own binding.
	second, err := svc.GenerateLinkToken(ctx, profileID)
	if err != nil {
		t.Fatalf("second GenerateLinkToken: %v", err)
	}
	res, err := svc.RedeemLinkToken(ctx, second.Token, 1234, "frank")
	if err != nil {
		t.Fatalf("self-relink redeem: %v", err)
	}
	if !res.AlreadyLinked {
		t.Error("AlreadyLinked = false, want true for a self-relink")
	}
	if res.ProfileID != profileID {
		t.Fatalf("ProfileID = %v, want %v", res.ProfileID, profileID)
	}
}

func TestRedeemLinkToken_ProfileRelinksToNewTelegramAccount(t *testing.T) {
	svc, q := newTestLinkService(t)
	ctx := context.Background()
	profileID := createProfile(t, q, "+998901110007")

	firstTok, _ := svc.GenerateLinkToken(ctx, profileID)
	if _, err := svc.RedeemLinkToken(ctx, firstTok.Token, 2001, "old-account"); err != nil {
		t.Fatalf("initial redeem: %v", err)
	}

	secondTok, _ := svc.GenerateLinkToken(ctx, profileID)
	if _, err := svc.RedeemLinkToken(ctx, secondTok.Token, 2002, "new-account"); err != nil {
		t.Fatalf("relink redeem: %v", err)
	}

	if _, err := q.GetTelegramAccountByTgUserID(ctx, 2001); err == nil {
		t.Error("old tg_user_id binding should have been replaced, not left dangling")
	}
	account, err := q.GetTelegramAccountByTgUserID(ctx, 2002)
	if err != nil {
		t.Fatalf("GetTelegramAccountByTgUserID(2002): %v", err)
	}
	if account.ProfileID != profileID {
		t.Fatalf("ProfileID = %v, want %v", account.ProfileID, profileID)
	}
}

func TestRedeemLinkToken_RejectsHijackOfAnotherProfilesTelegramAccount(t *testing.T) {
	svc, q := newTestLinkService(t)
	ctx := context.Background()
	profileA := createProfile(t, q, "+998901110008")
	profileB := createProfile(t, q, "+998901110009")

	tokA, _ := svc.GenerateLinkToken(ctx, profileA)
	if _, err := svc.RedeemLinkToken(ctx, tokA.Token, 3000, "shared-telegram"); err != nil {
		t.Fatalf("profile A initial redeem: %v", err)
	}

	tokB, _ := svc.GenerateLinkToken(ctx, profileB)
	_, err := svc.RedeemLinkToken(ctx, tokB.Token, 3000, "shared-telegram")
	if !errors.Is(err, ErrTelegramAccountLinkedElsewhere) {
		t.Fatalf("err = %v, want ErrTelegramAccountLinkedElsewhere", err)
	}

	// Profile A's binding must survive the attempted hijack untouched.
	account, err := q.GetTelegramAccountByTgUserID(ctx, 3000)
	if err != nil {
		t.Fatalf("GetTelegramAccountByTgUserID: %v", err)
	}
	if account.ProfileID != profileA {
		t.Fatalf("binding hijacked: now points at %v, want %v", account.ProfileID, profileA)
	}

	// Profile B's token was neither consumed nor silently succeeded — it can
	// still be redeemed for a Telegram account that isn't already claimed.
	res, err := svc.RedeemLinkToken(ctx, tokB.Token, 3001, "profile-b-telegram")
	if err != nil {
		t.Fatalf("profile B redeem after rejected hijack: %v", err)
	}
	if res.ProfileID != profileB {
		t.Fatalf("ProfileID = %v, want %v", res.ProfileID, profileB)
	}
}

func TestRedeemLinkToken_ConcurrentDifferentTokensSameTelegramAccountOnlyOneWins(t *testing.T) {
	svc, q := newTestLinkService(t)
	ctx := context.Background()
	profileA := createProfile(t, q, "+998901110011")
	profileB := createProfile(t, q, "+998901110012")

	tokA, err := svc.GenerateLinkToken(ctx, profileA)
	if err != nil {
		t.Fatalf("GenerateLinkToken A: %v", err)
	}
	tokB, err := svc.GenerateLinkToken(ctx, profileB)
	if err != nil {
		t.Fatalf("GenerateLinkToken B: %v", err)
	}

	const sameTgUserID = 4242
	var wg sync.WaitGroup
	var resA, resB RedeemResult
	var errA, errB error
	wg.Add(2)
	go func() {
		defer wg.Done()
		resA, errA = svc.RedeemLinkToken(ctx, tokA.Token, sameTgUserID, "racer-a")
	}()
	go func() {
		defer wg.Done()
		resB, errB = svc.RedeemLinkToken(ctx, tokB.Token, sameTgUserID, "racer-b")
	}()
	wg.Wait()

	successes := 0
	var winner uuid.UUID
	for _, r := range []struct {
		res RedeemResult
		err error
	}{{resA, errA}, {resB, errB}} {
		if r.err == nil {
			successes++
			winner = r.res.ProfileID
			continue
		}
		if !errors.Is(r.err, ErrTelegramAccountLinkedElsewhere) {
			t.Errorf("unexpected error: %v", r.err)
		}
	}
	if successes != 1 {
		t.Fatalf("successes = %d, want exactly 1 (two different tokens racing to bind the same tg_user_id)", successes)
	}

	account, err := q.GetTelegramAccountByTgUserID(ctx, sameTgUserID)
	if err != nil {
		t.Fatalf("GetTelegramAccountByTgUserID: %v", err)
	}
	if account.ProfileID != winner {
		t.Fatalf("final binding = %v, want the winning profile %v", account.ProfileID, winner)
	}
}

func TestRedeemLinkToken_ConcurrentRedeemOnlyOneWins(t *testing.T) {
	svc, q := newTestLinkService(t)
	ctx := context.Background()
	profileID := createProfile(t, q, "+998901110010")

	tok, err := svc.GenerateLinkToken(ctx, profileID)
	if err != nil {
		t.Fatalf("GenerateLinkToken: %v", err)
	}

	const attempts = 8
	var wg sync.WaitGroup
	errs := make([]error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.RedeemLinkToken(ctx, tok.Token, int64(9000+i), "racer")
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, e := range errs {
		if e == nil {
			successes++
			continue
		}
		if !errors.Is(e, ErrLinkTokenAlreadyUsed) {
			t.Errorf("unexpected error: %v", e)
		}
	}
	if successes != 1 {
		t.Fatalf("successes = %d, want exactly 1 (of %d concurrent redeems of the same token)", successes, attempts)
	}
}
