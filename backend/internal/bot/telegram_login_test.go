package bot

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/db/sqlc"
)

const loginUA = "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Mobile Safari/537.36"

func startWebsiteLogin(t *testing.T, svc *auth.Service) auth.TelegramLoginStart {
	t.Helper()
	st, err := svc.StartTelegramLogin(context.Background(), "10.0.0.9", loginUA, "AvtoTestBot")
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func contactUpdate(tgUserID, contactUserID int64, phone string) Update {
	return Update{
		UpdateID: 4,
		Message: &Message{
			From:    &User{ID: tgUserID, FirstName: "Ali"},
			Chat:    Chat{ID: tgUserID, Type: "private"},
			Contact: &Contact{PhoneNumber: phone, UserID: contactUserID},
		},
	}
}

func loginStatus(t *testing.T, svc *auth.Service, st auth.TelegramLoginStart) string {
	t.Helper()
	s, err := svc.TelegramLoginStatus(context.Background(), st.Token, st.BrowserSecret, "")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (f *fakeTelegram) loginButtons(t *testing.T) (yes, no InlineKeyboardButton) {
	t.Helper()
	var m InlineKeyboardMarkup
	if err := json.Unmarshal([]byte(f.lastMarkup()), &m); err != nil {
		t.Fatalf("markup=%q: %v", f.lastMarkup(), err)
	}
	if len(m.InlineKeyboard) != 1 || len(m.InlineKeyboard[0]) != 2 {
		t.Fatalf("markup=%+v", m)
	}
	return m.InlineKeyboard[0][0], m.InlineKeyboard[0][1]
}

func TestTelegramLogin_FirstTimeSharesPhone(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	svc := attachAuth(t, b, q)
	st := startWebsiteLogin(t, svc)

	if err := b.HandleUpdate(ctx, privateUpdate("/start login_"+st.Token, 3101, "Ali", "uz")); err != nil {
		t.Fatal(err)
	}
	msg := fake.lastMessage()
	if !strings.Contains(msg, "🔐 Driver Go'ga kirish") || !strings.Contains(msg, "Chrome · Android orqali kirish so'raldi.") ||
		!strings.Contains(msg, "«📱 Raqamni yuborish»") {
		t.Fatalf("prompt=%q", msg)
	}
	var kb ReplyKeyboardMarkup
	if err := json.Unmarshal([]byte(fake.lastMarkup()), &kb); err != nil || len(kb.Keyboard) != 1 ||
		!kb.Keyboard[0][0].RequestContact || kb.Keyboard[0][0].Text != "📱 Raqamni yuborish" || !kb.OneTimeKeyboard {
		t.Fatalf("keyboard=%s", fake.lastMarkup())
	}
	if loginStatus(t, svc, st) != auth.TelegramLoginStatePending {
		t.Fatal("opening the link approves nothing")
	}

	if err := b.HandleUpdate(ctx, contactUpdate(3101, 3101, "998901310101")); err != nil {
		t.Fatal(err)
	}
	if got := fake.lastMessage(); got != "✅ Kirildi. Brauzerga qayting." {
		t.Fatalf("done=%q", got)
	}
	if !strings.Contains(fake.lastMarkup(), `"remove_keyboard":true`) {
		t.Fatalf("keyboard not removed: %s", fake.lastMarkup())
	}
	if loginStatus(t, svc, st) != auth.TelegramLoginStateApproved {
		t.Fatal("not approved")
	}
}

func TestTelegramLogin_ContactChecksInRussian(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	svc := attachAuth(t, b, q)
	st := startWebsiteLogin(t, svc)
	if err := b.HandleUpdate(ctx, privateUpdate("/start login_"+st.Token, 3102, "Ali", "ru")); err != nil {
		t.Fatal(err)
	}
	if msg := fake.lastMessage(); !strings.Contains(msg, "🔐 Вход в Driver Go") || !strings.Contains(msg, "Chrome · Android") {
		t.Fatalf("ru prompt=%q", msg)
	}
	ru := contactUpdate(3102, 999, "998901310202")
	ru.Message.From.LanguageCode = "ru"
	if err := b.HandleUpdate(ctx, ru); err != nil {
		t.Fatal(err)
	}
	if msg := fake.lastMessage(); !strings.Contains(msg, "свой номер") {
		t.Fatalf("forwarded contact reply=%q", msg)
	}
	foreign := contactUpdate(3102, 3102, "79161234567")
	foreign.Message.From.LanguageCode = "ru"
	if err := b.HandleUpdate(ctx, foreign); err != nil {
		t.Fatal(err)
	}
	if msg := fake.lastMessage(); !strings.Contains(msg, "+998") {
		t.Fatalf("foreign reply=%q", msg)
	}
	if loginStatus(t, svc, st) != auth.TelegramLoginStatePending {
		t.Fatal("bad contacts must not approve")
	}
}

func TestTelegramLogin_LinkedUserTapsKirish(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	svc := attachAuth(t, b, q)
	reg, err := svc.Register(ctx, auth.RegisterInput{Phone: "901310303", Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: reg.Profile.ID, TgUserID: 3103, PhoneVerified: true}); err != nil {
		t.Fatal(err)
	}
	st := startWebsiteLogin(t, svc)
	if err := b.HandleUpdate(ctx, privateUpdate("/start login_"+st.Token, 3103, "Ali", "uz")); err != nil {
		t.Fatal(err)
	}
	msg := fake.lastMessage()
	if !strings.Contains(msg, "Siz bo'lsangiz «✅ Kirish» ni bosing. Siz so'ramagan bo'lsangiz — e'tibor bermang.") ||
		!strings.Contains(msg, "+998 90 ••• •• 03") {
		t.Fatalf("confirm=%q", msg)
	}
	yes, no := fake.loginButtons(t)
	if yes.Text != "✅ Kirish" || no.Text != "✖️ Bekor qilish" {
		t.Fatalf("buttons %q %q", yes.Text, no.Text)
	}
	if strings.Contains(yes.CallbackData, st.Token) || len(yes.CallbackData) > 64 {
		t.Fatal("callback_data must carry the nonce only")
	}
	// Someone else (forwarded message) tapping: no-op.
	if err := b.HandleUpdate(ctx, callback(4444, yes.CallbackData)); err != nil {
		t.Fatal(err)
	}
	if loginStatus(t, svc, st) != auth.TelegramLoginStatePending {
		t.Fatal("foreign tap approved")
	}
	if err := b.HandleUpdate(ctx, callback(3103, yes.CallbackData)); err != nil {
		t.Fatal(err)
	}
	if got := fake.lastEdit(); got != "✅ Kirildi. Brauzerga qayting." {
		t.Fatalf("edit=%q", got)
	}
	if loginStatus(t, svc, st) != auth.TelegramLoginStateApproved {
		t.Fatal("not approved")
	}
	edits := len(fake.allEdits())
	if err := b.HandleUpdate(ctx, callback(3103, no.CallbackData)); err != nil {
		t.Fatal(err)
	}
	if len(fake.allEdits()) != edits || loginStatus(t, svc, st) != auth.TelegramLoginStateApproved {
		t.Fatal("a late «Bekor qilish» changed an approved login")
	}
}

func TestTelegramLogin_CancelAndGroupRefusal(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	svc := attachAuth(t, b, q)
	reg, err := svc.Register(ctx, auth.RegisterInput{Phone: "901310404", Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: reg.Profile.ID, TgUserID: 3104, PhoneVerified: true}); err != nil {
		t.Fatal(err)
	}
	st := startWebsiteLogin(t, svc)

	group := privateUpdate("/start login_"+st.Token, 3104, "Ali", "uz")
	group.Message.Chat = Chat{ID: -100, Type: "supergroup"}
	if err := b.HandleUpdate(ctx, group); err != nil {
		t.Fatal(err)
	}
	if loginStatus(t, svc, st) != auth.TelegramLoginStatePending || strings.Contains(fake.lastMarkup(), "tgl:") {
		t.Fatal("a group must never get the login question")
	}

	if err := b.HandleUpdate(ctx, privateUpdate("/start login_"+st.Token, 3104, "Ali", "uz")); err != nil {
		t.Fatal(err)
	}
	_, no := fake.loginButtons(t)
	if err := b.HandleUpdate(ctx, callback(3104, no.CallbackData)); err != nil {
		t.Fatal(err)
	}
	if got := fake.lastEdit(); !strings.Contains(got, "bekor qilindi") {
		t.Fatalf("cancel edit=%q", got)
	}
	if loginStatus(t, svc, st) != auth.TelegramLoginStateCancelled {
		t.Fatal("not cancelled")
	}
	// The link again: it is spent.
	if err := b.HandleUpdate(ctx, privateUpdate("/start login_"+st.Token, 3104, "Ali", "uz")); err != nil {
		t.Fatal(err)
	}
	if got := fake.lastMessage(); !strings.Contains(got, "eskirgan") {
		t.Fatalf("spent link reply=%q", got)
	}
}

func TestStartRefRemembersReferralAndShowsMenu(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	if err := b.HandleUpdate(ctx, privateUpdate("/start ref_REF-AB23CD", 3105, "Ali", "uz")); err != nil {
		t.Fatal(err)
	}
	code, err := q.GetTelegramBotUserPendingReferral(ctx, 3105)
	if err != nil || code != "REF-AB23CD" {
		t.Fatalf("pending referral = %q %v", code, err)
	}
	if len(fake.allMessages()) != 1 || strings.Contains(fake.lastMessage(), "Havola") {
		t.Fatalf("ref start should get the normal welcome, got %q", fake.allMessages())
	}
	// Garbage after ref_ is not stored and is not treated as a link token.
	if err := b.HandleUpdate(ctx, privateUpdate("/start ref_<script>", 3106, "Ali", "uz")); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetTelegramBotUserPendingReferral(ctx, 3106); err == nil {
		t.Fatal("invalid code stored")
	}
}
