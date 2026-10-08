package auth

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// Audit-2 minor findings (M1–M5, M8).

// M1: a contact for a reset that is still pending for this Telegram user but
// has expired is "nothing waiting" (silence), not "link invalid" — the same
// contact is what the Mini App's phone share posts into the bot chat.
func TestPasswordResetContactForExpiredResetIsNone(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	if _, err := svc.Register(ctx, RegisterInput{Phone: "901150001", Password: "oldpass12", Name: "E"}); err != nil {
		t.Fatal(err)
	}
	start, err := svc.StartPasswordReset(ctx, "901150001", "6.6.6.6", "AvtoTestBot")
	if err != nil {
		t.Fatal(err)
	}
	begin, err := svc.BeginTelegramPasswordReset(ctx, parseResetRaw(t, start.BotURL), 8101)
	if err != nil || begin.Outcome != TelegramResetNeedContact {
		t.Fatalf("begin=%+v %v", begin, err)
	}
	if _, err := svc.Pool.Exec(ctx, `UPDATE password_reset_token SET expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	res, err := svc.ConfirmTelegramPasswordResetContact(ctx, 8101, 8101, "+998901150001")
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != TelegramResetNone {
		t.Fatalf("expired reset contact=%s want none", res.Outcome)
	}
}

// M2: two /start pwr_ for different resets by the same Telegram user at once
// race on the pending unique index; the loser retries instead of failing.
func TestBeginTelegramPasswordResetConcurrentForSameTelegramUser(t *testing.T) {
	svc, _ := resetTestService(t)
	ctx := context.Background()
	const tgID = 8201
	phones := []string{"901150002", "901150003"}
	for _, p := range phones {
		if _, err := svc.Register(ctx, RegisterInput{Phone: p, Password: "oldpass12", Name: "C"}); err != nil {
			t.Fatal(err)
		}
	}
	for round := 0; round < 15; round++ {
		raws := make([]string, len(phones))
		for i, p := range phones {
			normalized, _ := NormalizePhone(p)
			if err := svc.Lim.R.Del(ctx, "reset:cooldown:"+normalized, "reset:phone:"+normalized).Err(); err != nil {
				t.Fatal(err)
			}
			start, err := svc.StartPasswordReset(ctx, p, "", "AvtoTestBot")
			if err != nil {
				t.Fatal(err)
			}
			raws[i] = parseResetRaw(t, start.BotURL)
		}
		var wg sync.WaitGroup
		gate := make(chan struct{})
		errs := make([]error, len(raws))
		outcomes := make([]string, len(raws))
		for i := range raws {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-gate
				res, err := svc.BeginTelegramPasswordReset(ctx, raws[i], tgID)
				errs[i], outcomes[i] = err, res.Outcome
			}(i)
		}
		close(gate)
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Fatalf("round %d begin %d: %v", round, i, err)
			}
			if outcomes[i] != TelegramResetNeedContact {
				t.Fatalf("round %d begin %d outcome=%s", round, i, outcomes[i])
			}
		}
		var pending int
		if err := svc.Pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM password_reset_token WHERE pending_tg_user_id=$1 AND used_at IS NULL`, tgID).Scan(&pending); err != nil {
			t.Fatal(err)
		}
		if pending != 1 {
			t.Fatalf("round %d: %d resets pending for one Telegram user", round, pending)
		}
	}
}

// M3: the per-IP Mini App bucket is a brake on forged payloads only. A
// classroom or carrier NAT full of real learners must not run into it.
func TestTelegramWebAppLoginIPLimitIgnoresValidSignIns(t *testing.T) {
	svc, ctx := newWebAppService(t)
	const ip = "203.0.113.77"
	for i := 0; i < 320; i++ {
		raw := signInitData(t, testBotToken, webAppFields(int64(90000+i), time.Now()))
		if _, err := svc.TelegramWebAppLogin(ctx, raw, ip); err != nil {
			t.Fatalf("valid sign-in %d from a shared IP: %v", i, err)
		}
	}
	// Garbage from the same IP is still throttled after its own budget.
	for i := 0; i < 300; i++ {
		if _, err := svc.TelegramWebAppLogin(ctx, "hash=00", ip); err == nil {
			t.Fatalf("garbage %d accepted", i)
		}
	}
	if _, err := svc.TelegramWebAppLogin(ctx, "hash=00", ip); err != ErrRateLimited {
		t.Fatalf("garbage past the budget: err=%v want rate limited", err)
	}
}

func observedService(t *testing.T, level zapcore.Level) (*Service, context.Context, *observer.ObservedLogs) {
	t.Helper()
	svc, ctx := newWebAppService(t)
	core, logs := observer.New(level)
	svc.Log = zap.New(core)
	return svc, ctx, logs
}

// M4: signing in with a typed phone (launch data, no shared contact) is the
// normal path, not a proof failure: Debug, not Warn. A real failure stays Warn.
func TestLinkSkippedLogLevel(t *testing.T) {
	svc, ctx, logs := observedService(t, zap.DebugLevel)
	const phone, pw = "+998901150004", "level-pass-1"
	if _, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: pw, Name: "L"}); err != nil {
		t.Fatal(err)
	}
	raw := signInitData(t, testBotToken, webAppFields(8401, time.Now()))
	if _, err := svc.Login(ctx, LoginInput{Phone: phone, Password: pw, TgInitData: raw}); err != nil {
		t.Fatal(err)
	}
	skipped := logs.FilterMessage("auth.telegram_link_skipped").AllUntimed()
	if len(skipped) != 1 || skipped[0].Level != zap.DebugLevel {
		t.Fatalf("no-contact skip entries=%+v want one at Debug", skipped)
	}
	logs.TakeAll() // a filtered view is a copy; clear the source

	_, otherContact := proof(t, 8402, phone) // contact of another Telegram user
	if _, err := svc.Login(ctx, LoginInput{Phone: phone, Password: pw, TgInitData: raw, TgContact: otherContact}); err != nil {
		t.Fatal(err)
	}
	skipped = logs.FilterMessage("auth.telegram_link_skipped").AllUntimed()
	if len(skipped) != 1 || skipped[0].Level != zap.WarnLevel {
		t.Fatalf("proof-failure skip entries=%+v want one at Warn", skipped)
	}
}

// M5: the bot reset's contact path replacing the profile's link to another
// Telegram account is logged like the Mini App's replacement.
func TestBotResetContactPathLogsLinkReplaced(t *testing.T) {
	svc, ctx, logs := observedService(t, zap.InfoLevel)
	const phone = "+998901150005"
	reg, err := svc.Register(ctx, RegisterInput{Phone: phone, Password: "replace-pass-1", Name: "P"})
	if err != nil {
		t.Fatal(err)
	}
	legacyLink(t, svc, reg.Profile.ID, 8501)
	start, err := svc.StartPasswordReset(ctx, phone, "7.7.7.7", "AvtoTestBot")
	if err != nil {
		t.Fatal(err)
	}
	raw := parseResetRaw(t, start.BotURL)
	if b, err := svc.BeginTelegramPasswordReset(ctx, raw, 8502); err != nil || b.Outcome != TelegramResetNeedContact {
		t.Fatalf("begin=%+v %v", b, err)
	}
	c, err := svc.ConfirmTelegramPasswordResetContact(ctx, 8502, 8502, phone)
	if err != nil || c.Outcome != TelegramResetNeedConfirm {
		t.Fatalf("contact=%+v %v", c, err)
	}
	if res, err := svc.AnswerTelegramPasswordResetConfirm(ctx, 8502, c.ConfirmNonce, true); err != nil || res.Outcome != TelegramResetVerified {
		t.Fatalf("yes=%+v %v", res, err)
	}
	entries := logs.FilterMessage("auth.telegram_link_replaced").AllUntimed()
	if len(entries) != 1 || entries[0].ContextMap()["profile_id"] != reg.Profile.ID.String() {
		t.Fatalf("replaced entries=%+v", entries)
	}
	if tg, _ := tgLinkOf(t, svc, reg.Profile.ID); tg != 8502 {
		t.Fatalf("link=%d want 8502", tg)
	}
}

// M8, revised: a Telegram user who reached the «Ha, men» question through
// the contact path but meanwhile got linked to ANOTHER profile has still
// proven this profile's phone. Answering "stale" left the reset pending with
// no way forward; instead the tap moves the link here, exactly like the Mini
// App's phone share does (linkTelegramInTx), and verifies the reset.
func TestPasswordResetConfirmMovesLinkFromAnotherProfile(t *testing.T) {
	svc, ctx, logs := observedService(t, zap.InfoLevel)
	raw, nonce := contactMatchedReset(t, svc, "901150006", 8601, "+998901150006")
	other, err := svc.Register(ctx, RegisterInput{Phone: "901150007", Password: "otherpass1", Name: "O"})
	if err != nil {
		t.Fatal(err)
	}
	legacyLink(t, svc, other.Profile.ID, 8601)

	res, err := svc.AnswerTelegramPasswordResetConfirm(ctx, 8601, nonce, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != TelegramResetVerified {
		t.Fatalf("yes from a user linked elsewhere=%s want verified", res.Outcome)
	}
	assertResetState(t, svc, raw, ResetStateVerified)
	if _, ok := tgLinkOf(t, svc, other.Profile.ID); ok {
		t.Fatal("other profile kept the link")
	}
	if !phoneVerified(t, svc, 8601) {
		t.Fatal("moved link is not phone-verified")
	}
	entries := logs.FilterMessage("auth.telegram_link_moved").AllUntimed()
	if len(entries) != 1 || entries[0].ContextMap()["from_profile_id"] != other.Profile.ID.String() {
		t.Fatalf("moved entries=%+v", entries)
	}
}
