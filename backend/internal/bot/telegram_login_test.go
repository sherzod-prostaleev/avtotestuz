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
			From:    &User{ID: tgUserID, FirstName: "Ali", LastName: "Valiyev"},
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
		!strings.Contains(msg, "«📱 Raqamni yuborish» tugmasini bosing, so'ng «✅ Kirish» ni bosing") {
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

	// The share is not consent (audit F1): it earns the same question a
	// linked account gets — device, masked account, two buttons.
	if err := b.HandleUpdate(ctx, contactUpdate(3101, 3101, "998901310101")); err != nil {
		t.Fatal(err)
	}
	if got, want := fake.lastMessage(), "🔐 Driver Go'ga kirish\nChrome · Android orqali +998 90 ••• •• 01 hisobiga kirish so'raldi.\n\n"+
		"Siz bo'lsangiz «✅ Kirish» ni bosing. Siz so'ramagan bo'lsangiz — «✖️ Bekor qilish» ni bosing."; got != want {
		t.Fatalf("question=%q", got)
	}
	yes, no := fake.loginButtons(t)
	if yes.Text != "✅ Kirish" || no.Text != "✖️ Bekor qilish" || strings.Contains(yes.CallbackData, st.Token) || len(yes.CallbackData) > 64 {
		t.Fatalf("buttons %+v %+v", yes, no)
	}
	if loginStatus(t, svc, st) != auth.TelegramLoginStatePending {
		t.Fatal("a phone share alone approved the login")
	}

	tap := callback(3101, yes.CallbackData)
	tap.CallbackQuery.From.FirstName, tap.CallbackQuery.From.LastName = "Ali", "Valiyev"
	if err := b.HandleUpdate(ctx, tap); err != nil {
		t.Fatal(err)
	}
	if loginStatus(t, svc, st) != auth.TelegramLoginStateApproved {
		t.Fatal("not approved")
	}
	// The question is closed in place; the outcome takes the share keyboard down.
	if got := fake.lastEdit(); got != "Javobingiz qabul qilindi." {
		t.Fatalf("edit=%q", got)
	}
	if got := fake.lastMessage(); got != "✅ Kirildi. Brauzerga qayting." {
		t.Fatalf("done=%q", got)
	}
	if !strings.Contains(fake.lastMarkup(), `"remove_keyboard":true`) {
		t.Fatalf("keyboard not removed: %s", fake.lastMarkup())
	}
	// Telegram's first AND last name make the new profile's name.
	var name string
	if err := b.Link.Pool.QueryRow(ctx, `SELECT name FROM profile WHERE phone = '+998901310101'`).Scan(&name); err != nil || name != "Ali Valiyev" {
		t.Fatalf("profile name = %q %v", name, err)
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
	if !strings.Contains(msg, "Siz bo'lsangiz «✅ Kirish» ni bosing. Siz so'ramagan bo'lsangiz — «✖️ Bekor qilish» ni bosing.") ||
		!strings.Contains(msg, "Chrome · Android orqali +998 90 ••• •• 03 hisobiga kirish so'raldi.") {
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

// Audit A1, through the dispatcher: the victim pressed START on an attacker's
// login link, then started a real password reset and shared their phone for
// it. The share must reach the reset («Ha, men» question), not the login.
func TestAuditA1_ResetShareNeverApprovesALogin(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	svc := attachAuth(t, b, q)
	if _, err := svc.Register(ctx, auth.RegisterInput{Phone: "901320101", Password: "secret123"}); err != nil {
		t.Fatal(err)
	}
	attacker := startWebsiteLogin(t, svc)
	if err := b.HandleUpdate(ctx, privateUpdate("/start login_"+attacker.Token, 3201, "Ali", "uz")); err != nil {
		t.Fatal(err)
	}
	reset, err := svc.StartPasswordReset(ctx, "901320101", "10.0.0.1", "AvtoTestBot")
	if err != nil {
		t.Fatal(err)
	}
	raw := reset.BotURL[strings.Index(reset.BotURL, "?start=")+len("?start="):]
	if err := b.HandleUpdate(ctx, privateUpdate("/start "+raw, 3201, "Ali", "uz")); err != nil {
		t.Fatal(err)
	}
	if err := b.HandleUpdate(ctx, contactUpdate(3201, 3201, "998901320101")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fake.lastMessage(), "Parolni tiklash so'raldi") || !strings.Contains(fake.lastMarkup(), cbResetYes) {
		t.Fatalf("the share must earn the reset question, got %q / %s", fake.lastMessage(), fake.lastMarkup())
	}
	for _, m := range fake.allMessages() {
		if strings.Contains(m, "Kirildi") {
			t.Fatalf("the bot announced a sign-in: %q", m)
		}
	}
	if loginStatus(t, svc, attacker) != auth.TelegramLoginStatePending {
		t.Fatal("the attacker's login was approved")
	}
	if _, err := svc.CompleteTelegramLogin(ctx, attacker.Token, attacker.BrowserSecret, "6.6.6.6"); err == nil {
		t.Fatal("the attacker's browser was signed in")
	}
}

// Audit A2, through the dispatcher: a stray contact (the Mini App's phone
// sheet echo) earns a question and approves nothing.
func TestAuditA2_StrayContactOnlyAsks(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	svc := attachAuth(t, b, q)
	attacker := startWebsiteLogin(t, svc)
	if err := b.HandleUpdate(ctx, privateUpdate("/start login_"+attacker.Token, 3202, "Ali", "uz")); err != nil {
		t.Fatal(err)
	}
	if err := b.HandleUpdate(ctx, contactUpdate(3202, 3202, "998901320202")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fake.lastMarkup(), cbLoginYes) || strings.Contains(fake.lastMessage(), "Kirildi") {
		t.Fatalf("got %q / %s", fake.lastMessage(), fake.lastMarkup())
	}
	if loginStatus(t, svc, attacker) != auth.TelegramLoginStatePending {
		t.Fatal("approved without a tap")
	}
	var n int
	if err := b.Link.Pool.QueryRow(ctx, `SELECT count(*) FROM profile WHERE phone = '+998901320202'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("profiles created before the tap: %d %v", n, err)
	}
}

// Audit F2: the second Telegram user to open a link is told so, politely, in
// their language, and gets no button of any kind.
func TestAuditF2_SecondOpenerIsToldTheLinkIsTaken(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	svc := attachAuth(t, b, q)
	st := startWebsiteLogin(t, svc)
	if err := b.HandleUpdate(ctx, privateUpdate("/start login_"+st.Token, 3301, "Ali", "uz")); err != nil {
		t.Fatal(err)
	}
	for lang, want := range map[string]string{
		"uz": "Bu havola boshqa foydalanuvchi tomonidan ochilgan.",
		"ru": "Эта ссылка уже открыта другим пользователем.",
	} {
		if err := b.HandleUpdate(ctx, privateUpdate("/start login_"+st.Token, 3302, "Vali", lang)); err != nil {
			t.Fatal(err)
		}
		if got := fake.lastMessage(); !strings.HasPrefix(got, want) {
			t.Fatalf("%s: second opener got %q", lang, got)
		}
		if m := fake.lastMarkup(); m != "" {
			t.Fatalf("second opener got a keyboard: %s", m)
		}
	}
	// Their phone share is not taken for this login.
	if err := b.HandleUpdate(ctx, contactUpdate(3302, 3302, "998901330202")); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fake.lastMarkup(), cbLoginYes) || loginStatus(t, svc, st) != auth.TelegramLoginStatePending {
		t.Fatal("the second opener's share reached the request")
	}
}

// Mutant M22 and the "private chat only" minor: nothing that decides a login
// happens outside the user's own chat with the bot — not in a group, not in a
// channel-typed chat.
func TestTelegramLogin_PrivateChatOnly(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	svc := attachAuth(t, b, q)
	reg, err := svc.Register(ctx, auth.RegisterInput{Phone: "901340101", Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: reg.Profile.ID, TgUserID: 3401, PhoneVerified: true}); err != nil {
		t.Fatal(err)
	}
	linked := startWebsiteLogin(t, svc)
	if err := b.HandleUpdate(ctx, privateUpdate("/start login_"+linked.Token, 3401, "Ali", "uz")); err != nil {
		t.Fatal(err)
	}
	yes, _ := fake.loginButtons(t)
	for _, chatType := range []string{"group", "supergroup", "channel"} {
		tap := callback(3401, yes.CallbackData)
		tap.CallbackQuery.Message.Chat.Type = chatType
		if err := b.HandleUpdate(ctx, tap); err != nil {
			t.Fatal(err)
		}
		if loginStatus(t, svc, linked) != auth.TelegramLoginStatePending {
			t.Fatalf("a tap in a %s chat approved the login", chatType)
		}
		if got := fake.lastAnswer(); got != "Bu so'rov endi amal qilmaydi." {
			t.Fatalf("%s tap answer=%q", chatType, got)
		}
	}

	// /start login_ and a contact in a channel-typed chat.
	fresh := startWebsiteLogin(t, svc)
	start := privateUpdate("/start login_"+fresh.Token, 3402, "Ali", "uz")
	start.Message.Chat.Type = "channel"
	if err := b.HandleUpdate(ctx, start); err != nil {
		t.Fatal(err)
	}
	if got := fake.lastMessage(); got != "Kirish faqat bot bilan shaxsiy chatda tasdiqlanadi." || fake.lastMarkup() != "" {
		t.Fatalf("channel start reply=%q markup=%s", got, fake.lastMarkup())
	}
	if err := b.HandleUpdate(ctx, privateUpdate("/start login_"+fresh.Token, 3402, "Ali", "uz")); err != nil {
		t.Fatal(err)
	}
	sent := len(fake.allMessages())
	share := contactUpdate(3402, 3402, "998901340202")
	share.Message.Chat.Type = "channel"
	if err := b.HandleUpdate(ctx, share); err != nil {
		t.Fatal(err)
	}
	if len(fake.allMessages()) != sent || loginStatus(t, svc, fresh) != auth.TelegramLoginStatePending {
		t.Fatal("a contact outside the private chat was acted on")
	}
}

// The telegram_login kill switch, as the learner sees it in the bot.
func TestTelegramLogin_SwitchedOff(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	svc := attachAuth(t, b, q)
	st := startWebsiteLogin(t, svc)
	set := func(on bool) {
		if _, err := b.Link.Pool.Exec(ctx, `UPDATE feature_flag SET value_json = to_jsonb($1::boolean) WHERE key = 'telegram_login'`, on); err != nil {
			t.Fatal(err)
		}
	}
	set(false)
	t.Cleanup(func() { set(true) })
	if err := b.HandleUpdate(ctx, privateUpdate("/start login_"+st.Token, 3501, "Ali", "ru")); err != nil {
		t.Fatal(err)
	}
	if got := fake.lastMessage(); got != "Вход через Telegram временно отключён. Войдите на сайте по номеру телефона и паролю." || fake.lastMarkup() != "" {
		t.Fatalf("reply=%q markup=%s", got, fake.lastMarkup())
	}
}
