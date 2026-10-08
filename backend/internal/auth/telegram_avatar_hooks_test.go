package auth

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// recordingAvatars is the avatar service as auth sees it. It checks, at call
// time, that the link change it is told about is already committed: the
// real service reads the database from another goroutine, so a hook fired
// inside the transaction would act on the old state.
type recordingAvatars struct {
	t    *testing.T
	pool *pgxpool.Pool

	mu       sync.Mutex
	linked   []uuid.UUID
	unlinked []uuid.UUID
}

func (r *recordingAvatars) verified(id uuid.UUID) bool {
	var ok bool
	if err := r.pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM telegram_account WHERE profile_id=$1 AND phone_verified_at IS NOT NULL)`, id).Scan(&ok); err != nil {
		r.t.Error(err)
	}
	return ok
}

func (r *recordingAvatars) TelegramLinked(id uuid.UUID) {
	if !r.verified(id) {
		r.t.Errorf("TelegramLinked(%s) before a verified link is visible", id)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.linked = append(r.linked, id)
}

func (r *recordingAvatars) TelegramUnlinked(id uuid.UUID) {
	if r.verified(id) {
		r.t.Errorf("TelegramUnlinked(%s) while its verified link is still visible", id)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unlinked = append(r.unlinked, id)
}

func withAvatars(t *testing.T, svc *Service) *recordingAvatars {
	t.Helper()
	rec := &recordingAvatars{t: t, pool: svc.Pool}
	svc.Avatars = rec
	return rec
}

func TestPhoneVerifiedLinkTriggersAvatarFetch(t *testing.T) {
	svc, ctx := newWebAppService(t)
	rec := withAvatars(t, svc)

	const phone, pw = "+998901170001", "avatar-pass-1"
	raw, contact := proof(t, 7001, phone)
	reg, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: pw, Name: "A", TgInitData: raw, TgContact: contact})
	if err != nil || !reg.TelegramLinked {
		t.Fatalf("register: %+v %v", reg, err)
	}
	if !slices.Equal(rec.linked, []uuid.UUID{reg.Profile.ID}) {
		t.Fatalf("linked = %v, want the new profile once", rec.linked)
	}

	// A sign-in with no Telegram proof is not a link change.
	if _, err := svc.Login(ctx, LoginInput{Phone: phone, Password: pw}); err != nil {
		t.Fatal(err)
	}
	// Nor is one whose proof fails (contact of another number).
	raw2, wrong := proof(t, 7001, "+998901179999")
	if _, err := svc.Login(ctx, LoginInput{Phone: phone, Password: pw, TgInitData: raw2, TgContact: wrong}); err != nil {
		t.Fatal(err)
	}
	if len(rec.linked) != 1 || len(rec.unlinked) != 0 {
		t.Fatalf("linked=%v unlinked=%v after unproven sign-ins", rec.linked, rec.unlinked)
	}

	// Re-proving in the Mini App (link-webapp) refreshes.
	raw3, contact3 := proof(t, 7001, phone)
	if ok, err := svc.LinkTelegramWebApp(ctx, reg.Profile.ID, raw3, contact3); err != nil || !ok {
		t.Fatalf("link-webapp: %v %v", ok, err)
	}
	if len(rec.linked) != 2 {
		t.Fatalf("linked = %v after link-webapp", rec.linked)
	}
}

// Moving a Telegram account to the profile whose phone it proved takes the
// photo off the profile it left.
func TestMovedLinkDropsAvatarOfPreviousProfile(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const phoneA, phoneB, pw = "+998901170002", "+998901170003", "avatar-pass-2"
	rawA, contactA := proof(t, 7002, phoneA)
	a, err := svc.Register(ctx, RegisterInput{Phone: phoneA, Password: pw, Name: "A", TgInitData: rawA, TgContact: contactA})
	if err != nil || !a.TelegramLinked {
		t.Fatalf("register A: %+v %v", a, err)
	}
	b, err := svc.Register(ctx, RegisterInput{Phone: phoneB, Password: pw, Name: "B"})
	if err != nil {
		t.Fatal(err)
	}
	rec := withAvatars(t, svc)

	rawB, contactB := proof(t, 7002, phoneB)
	login, err := svc.Login(ctx, LoginInput{Phone: phoneB, Password: pw, TgInitData: rawB, TgContact: contactB})
	if err != nil || !login.TelegramLinked {
		t.Fatalf("login B: %+v %v", login, err)
	}
	if !slices.Equal(rec.linked, []uuid.UUID{b.Profile.ID}) || !slices.Equal(rec.unlinked, []uuid.UUID{a.Profile.ID}) {
		t.Fatalf("linked=%v unlinked=%v, want B linked and A unlinked", rec.linked, rec.unlinked)
	}
}

// The bot reset: «Ha, men» leaves a phone-verified link (fetch), and the
// completed reset may drop links (reconcile).
func TestPasswordResetLinkChangesReachAvatars(t *testing.T) {
	svc, _ := resetTestService(t)
	rec := withAvatars(t, svc)
	ctx := context.Background()
	raw, nonce := contactMatchedReset(t, svc, "901170004", 7004, "+998901170004")

	res, err := svc.AnswerTelegramPasswordResetConfirm(ctx, 7004, nonce, true)
	if err != nil || res.Outcome != TelegramResetVerified {
		t.Fatalf("confirm: %+v %v", res, err)
	}
	var profileID uuid.UUID
	if err := svc.Pool.QueryRow(ctx, `SELECT profile_id FROM telegram_account WHERE tg_user_id=7004`).Scan(&profileID); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rec.linked, []uuid.UUID{profileID}) {
		t.Fatalf("linked = %v, want %s", rec.linked, profileID)
	}

	// The confirmer's verified link survives the reset, so nothing is
	// unlinked; a planted legacy link on the profile would not, and the
	// reconcile after commit is what drops its photo.
	if err := svc.CompletePasswordReset(ctx, raw, "newpass123", "4.4.4.4"); err != nil {
		t.Fatal(err)
	}
	if len(rec.unlinked) != 0 {
		t.Fatalf("unlinked = %v while the verified link survived", rec.unlinked)
	}
}

func TestPasswordResetDroppingLinkReconcilesAvatar(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	raw, nonce := contactMatchedReset(t, svc, "901170005", 7005, "+998901170005")
	if res, err := svc.AnswerTelegramPasswordResetConfirm(ctx, 7005, nonce, true); err != nil || res.Outcome != TelegramResetVerified {
		t.Fatalf("confirm: %+v %v", res, err)
	}
	var profileID uuid.UUID
	if err := svc.Pool.QueryRow(ctx, `SELECT profile_id FROM telegram_account WHERE tg_user_id=7005`).Scan(&profileID); err != nil {
		t.Fatal(err)
	}
	// After the confirmation, the link is re-pointed by a legacy token to
	// another Telegram user: the reset cannot attribute it and drops it.
	legacyLink(t, svc, profileID, 7006)
	rec := withAvatars(t, svc)
	if err := svc.CompletePasswordReset(ctx, raw, "newpass123", "4.4.4.4"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rec.unlinked, []uuid.UUID{profileID}) {
		t.Fatalf("unlinked = %v, want %s", rec.unlinked, profileID)
	}
}
