package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/progress"
	"avtotest.uz/backend/internal/redisx"
	"avtotest.uz/backend/internal/testdb"
)

// fakeTelegram stands in for api.telegram.org and records every
// sendMessage call, so dispatcher tests assert on the reply text without
// touching the network.
type fakeTelegram struct {
	mu   sync.Mutex
	sent []string
	// markups holds the raw reply_markup JSON per sendMessage ("" when none).
	markups []string
	// edits holds editMessageText texts; answers holds answerCallbackQuery texts.
	edits   []string
	answers []string
	srv     *httptest.Server
}

func newFakeTelegram(t *testing.T) (*fakeTelegram, *Client) {
	t.Helper()
	f := &fakeTelegram{}
	msgID := int64(100)
	pollSeq := 0
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.Contains(path, "sendMessage") || strings.Contains(path, "sendPhoto") {
			var body struct {
				Text    string          `json:"text"`
				Caption string          `json:"caption"`
				Markup  json.RawMessage `json:"reply_markup"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			text := body.Text
			if text == "" {
				text = body.Caption
			}
			f.mu.Lock()
			f.sent = append(f.sent, text)
			f.markups = append(f.markups, string(body.Markup))
			f.mu.Unlock()
			msgID++
			_, _ = fmt.Fprintf(w, `{"ok":true,"result":{"message_id":%d}}`, msgID)
			return
		}
		if strings.Contains(path, "sendPoll") {
			// A poll is the "message" a question arrives as now, so record its
			// question text the same way sendMessage/sendPhoto do — otherwise
			// callers asserting on f.sent would see a poll-only question as if
			// nothing had been sent at all.
			var body struct {
				Question string `json:"question"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			f.sent = append(f.sent, body.Question)
			pollSeq++
			f.mu.Unlock()
			msgID++
			_, _ = fmt.Fprintf(w,
				`{"ok":true,"result":{"message_id":%d,"poll":{"id":"fake-poll-%d"}}}`, msgID, pollSeq)
			return
		}
		if strings.Contains(path, "editMessageText") || strings.Contains(path, "answerCallbackQuery") {
			var body struct {
				Text string `json:"text"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			if strings.Contains(path, "editMessageText") {
				f.edits = append(f.edits, body.Text)
			} else {
				f.answers = append(f.answers, body.Text)
			}
			f.mu.Unlock()
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	t.Cleanup(f.srv.Close)
	return f, NewClient(f.srv.URL, "test-token", f.srv.Client())
}

func (f *fakeTelegram) lastMessage() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) == 0 {
		return ""
	}
	return f.sent[len(f.sent)-1]
}

func (f *fakeTelegram) allMessages() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.sent))
	copy(out, f.sent)
	return out
}

func newTestBot(t *testing.T) (*Bot, *sqlc.Queries, *fakeTelegram) {
	t.Helper()
	pool := testdb.New(t)
	q := sqlc.New(pool)
	fake, client := newFakeTelegram(t)
	b := wireTestBot(pool, q, client)
	return b, q, fake
}

func wireTestBot(pool *pgxpool.Pool, q *sqlc.Queries, client *Client) *Bot {
	quiz := &QuizService{
		Q:             q,
		Pool:          pool,
		TG:            client,
		MediaBaseURL:  "http://media.test",
		PublicBaseURL: "http://localhost:3000",
	}
	return &Bot{
		Link:          NewLinkService(pool, q),
		Quiz:          quiz,
		Billing:       billing.Service{Q: q},
		Progress:      progress.NewService(q),
		TG:            client,
		PublicBaseURL: "http://localhost:3000",
	}
}

func update(text string, tgUserID int64, username string) Update {
	return Update{
		UpdateID: 1,
		Message: &Message{
			Text: text,
			From: &User{ID: tgUserID, Username: username},
			Chat: Chat{ID: tgUserID},
		},
	}
}

func TestHandleUpdate_StartUnlinked(t *testing.T) {
	b, _, fake := newTestBot(t)
	if err := b.HandleUpdate(context.Background(), update("/start", 1, "u1")); err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if !strings.Contains(fake.lastMessage(), "Telegram bilan bog'lash") {
		t.Errorf("reply = %q, want unlinked greeting", fake.lastMessage())
	}
}

func TestHandleUpdate_StartWithValidTokenLinks(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	profile, err := q.CreateProfile(ctx, sqlc.CreateProfileParams{Phone: "+998901120001"})
	if err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	tok, err := b.Link.GenerateLinkToken(ctx, profile.ID)
	if err != nil {
		t.Fatalf("GenerateLinkToken: %v", err)
	}

	if err := b.HandleUpdate(ctx, update("/start "+tok.Token, 42, "alice")); err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if !strings.Contains(fake.lastMessage(), "ulandi") {
		t.Errorf("reply = %q, want success message", fake.lastMessage())
	}

	account, err := q.GetTelegramAccountByTgUserID(ctx, 42)
	if err != nil {
		t.Fatalf("GetTelegramAccountByTgUserID: %v", err)
	}
	if account.ProfileID != profile.ID {
		t.Fatalf("ProfileID = %v, want %v", account.ProfileID, profile.ID)
	}
}

func TestHandleUpdate_LinkCommandEquivalentToStartPayload(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	profile, _ := q.CreateProfile(ctx, sqlc.CreateProfileParams{Phone: "+998901120002"})
	tok, _ := b.Link.GenerateLinkToken(ctx, profile.ID)

	if err := b.HandleUpdate(ctx, update("/link "+tok.Token, 43, "bob")); err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if !strings.Contains(fake.lastMessage(), "ulandi") {
		t.Errorf("reply = %q, want success message", fake.lastMessage())
	}
}

func TestHandleUpdate_StartWithExpiredTokenFailsGracefully(t *testing.T) {
	b, _, fake := newTestBot(t)
	ctx := context.Background()

	if err := b.HandleUpdate(ctx, update("/start this-token-does-not-exist", 44, "carol")); err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if !strings.Contains(fake.lastMessage(), "noto'g'ri") {
		t.Errorf("reply = %q, want a friendly invalid-token message", fake.lastMessage())
	}
}

func TestHandleUpdate_LinkWithoutTokenPromptsUsage(t *testing.T) {
	b, _, fake := newTestBot(t)
	if err := b.HandleUpdate(context.Background(), update("/link", 45, "dave")); err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if !strings.Contains(fake.lastMessage(), "token topilmadi") {
		t.Errorf("reply = %q, want usage prompt", fake.lastMessage())
	}
}

func TestHandleUpdate_StatusUnlinked(t *testing.T) {
	b, _, fake := newTestBot(t)
	if err := b.HandleUpdate(context.Background(), update("/status", 46, "erin")); err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if !strings.Contains(fake.lastMessage(), "ulanmagan") {
		t.Errorf("reply = %q, want not-linked message", fake.lastMessage())
	}
}

func TestHandleUpdate_StatusLinkedShowsVIPAndStreak(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	profile, _ := q.CreateProfile(ctx, sqlc.CreateProfileParams{Phone: "+998901120003"})
	tok, _ := b.Link.GenerateLinkToken(ctx, profile.ID)
	if _, err := b.Link.RedeemLinkToken(ctx, tok.Token, 47, "frank"); err != nil {
		t.Fatalf("RedeemLinkToken: %v", err)
	}
	if _, err := b.Billing.GrantDays(ctx, profile.ID, 30, "admin", "test", uuid.NullUUID{}); err != nil {
		t.Fatalf("GrantDays: %v", err)
	}

	if err := b.HandleUpdate(ctx, update("/status", 47, "frank")); err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	msg := fake.lastMessage()
	if !strings.Contains(msg, "VIP") || !strings.Contains(msg, "Streak") {
		t.Errorf("reply = %q, want VIP and streak lines", msg)
	}
}

func TestHandleUpdate_UnknownCommandFallback(t *testing.T) {
	b, _, fake := newTestBot(t)
	if err := b.HandleUpdate(context.Background(), update("/bogus", 48, "grace")); err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if !strings.Contains(fake.lastMessage(), "Noma'lum buyruq") {
		t.Errorf("reply = %q, want unknown-command fallback", fake.lastMessage())
	}
}

func TestHandleUpdate_GroupStartShowsQuizHelp(t *testing.T) {
	b, _, fake := newTestBot(t)
	u := update("/start", 99, "guser")
	u.Message.Chat = Chat{ID: -1001, Type: "supergroup", Title: "Test guruh"}
	if err := b.HandleUpdate(context.Background(), u); err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if !strings.Contains(fake.lastMessage(), "/quiz") {
		t.Errorf("reply = %q, want group quiz help", fake.lastMessage())
	}
}

func TestHandleUpdate_Unlink(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	profile, _ := q.CreateProfile(ctx, sqlc.CreateProfileParams{Phone: "+998901120099"})
	tok, _ := b.Link.GenerateLinkToken(ctx, profile.ID)
	if _, err := b.Link.RedeemLinkToken(ctx, tok.Token, 77, "unz"); err != nil {
		t.Fatal(err)
	}
	if err := b.HandleUpdate(ctx, update("/unlink", 77, "unz")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fake.lastMessage(), "uzildi") {
		t.Errorf("reply = %q", fake.lastMessage())
	}
	if _, err := q.GetTelegramAccountByTgUserID(ctx, 77); err == nil {
		t.Fatal("expected account deleted")
	}
}

func TestHandleUpdate_IgnoresNonMessageUpdates(t *testing.T) {
	b, _, fake := newTestBot(t)
	if err := b.HandleUpdate(context.Background(), Update{UpdateID: 1}); err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if fake.lastMessage() != "" {
		t.Errorf("expected no reply for a message-less update, got %q", fake.lastMessage())
	}
}

func attachAuth(t *testing.T, b *Bot, q *sqlc.Queries) *auth.Service {
	t.Helper()
	c := redisx.NewTest(t)
	svc := auth.NewService(q, b.Link.Pool, auth.Limiter{R: c}, auth.SandboxSender{Log: zap.NewNop()}, []byte("test-secret-at-least-32-bytes!!"), "test")
	b.Auth = svc
	return svc
}

func TestHandleUpdate_PasswordResetLinkedAsksConfirm(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	svc := attachAuth(t, b, q)

	reg, err := svc.Register(ctx, auth.RegisterInput{Phone: "901000020", Password: "secret123", Name: "R"})
	if err != nil {
		t.Fatal(err)
	}
	tok, err := b.Link.GenerateLinkToken(ctx, reg.Profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Link.RedeemLinkToken(ctx, tok.Token, 2020, "resetu"); err != nil {
		t.Fatal(err)
	}
	start, err := svc.StartPasswordReset(ctx, "901000020", "10.0.0.1", "AvtoTestBot")
	if err != nil {
		t.Fatal(err)
	}
	raw := start.BotURL[strings.Index(start.BotURL, "pwr_")+4:]

	if err := b.HandleUpdate(ctx, update("/start "+auth.FormatPasswordResetStartPayload(raw), 2020, "resetu")); err != nil {
		t.Fatal(err)
	}
	// Linked or not, opening the link only asks the question.
	if want := "Parolni tiklash so'raldi: +998 90 ••• •• 20."; !strings.Contains(fake.lastMessage(), want) {
		t.Fatalf("reply=%q want %q", fake.lastMessage(), want)
	}
	if st := svc.PasswordResetStatus(ctx, raw).State; st != auth.ResetStatePending {
		t.Fatalf("status after /start=%s want pending", st)
	}
	yes, _ := fake.confirmButtons(t)
	if err := b.HandleUpdate(ctx, callback(2020, yes)); err != nil {
		t.Fatal(err)
	}
	if got := fake.lastEdit(); !strings.Contains(got, "Tasdiqlandi") {
		t.Fatalf("edit=%q", got)
	}
	if st := svc.PasswordResetStatus(ctx, raw).State; st != auth.ResetStateVerified {
		t.Fatalf("status after «Ha, men»=%s want verified", st)
	}
}

func TestHandleUpdate_PasswordResetContactMatch(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	svc := attachAuth(t, b, q)

	if _, err := svc.Register(ctx, auth.RegisterInput{Phone: "901000021", Password: "secret123", Name: "R"}); err != nil {
		t.Fatal(err)
	}
	start, err := svc.StartPasswordReset(ctx, "901000021", "10.0.0.2", "AvtoTestBot")
	if err != nil {
		t.Fatal(err)
	}
	raw := start.BotURL[strings.Index(start.BotURL, "pwr_")+4:]

	if err := b.HandleUpdate(ctx, update("/start "+auth.FormatPasswordResetStartPayload(raw), 2121, "reset2")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fake.lastMessage(), "telefon") {
		t.Fatalf("need-contact reply=%q", fake.lastMessage())
	}

	u := Update{
		UpdateID: 2,
		Message: &Message{
			From: &User{ID: 2121, Username: "reset2"},
			Chat: Chat{ID: 2121, Type: "private"},
			Contact: &Contact{
				PhoneNumber: "+998901000021",
				UserID:      2121,
			},
		},
	}
	if err := b.HandleUpdate(ctx, u); err != nil {
		t.Fatal(err)
	}
	// The contact alone (which the Mini App's phone share also produces)
	// must only ask, never verify.
	if got := fake.lastMessage(); !strings.Contains(got, "+998 90 ••• •• 21") || !strings.Contains(got, "«Ha, men»") {
		t.Fatalf("contact reply=%q", got)
	}
	if st := svc.PasswordResetStatus(ctx, raw).State; st != auth.ResetStatePending {
		t.Fatalf("status after contact=%s want pending", st)
	}
	yes, no := fake.confirmButtons(t)
	if strings.Contains(yes, raw) || strings.Contains(no, raw) {
		t.Fatal("callback_data carries the raw reset token")
	}

	// A tap from another Telegram user (e.g. the message forwarded) is a no-op.
	if err := b.HandleUpdate(ctx, callback(9999, yes)); err != nil {
		t.Fatal(err)
	}
	if got := fake.lastAnswer(); got == "" {
		t.Fatal("foreign tap got no answerCallbackQuery text")
	}
	if fake.lastEdit() != "" {
		t.Fatalf("foreign tap edited the message: %q", fake.lastEdit())
	}
	if st := svc.PasswordResetStatus(ctx, raw).State; st != auth.ResetStatePending {
		t.Fatalf("status after foreign tap=%s want pending", st)
	}

	if err := b.HandleUpdate(ctx, callback(2121, no)); err != nil {
		t.Fatal(err)
	}
	if got := fake.lastEdit(); !strings.Contains(got, "o'zgartirilmadi") {
		t.Fatalf("«Yo'q» edit=%q", got)
	}
	if st := svc.PasswordResetStatus(ctx, raw).State; st != auth.ResetStateInvalid {
		t.Fatalf("status after «Yo'q»=%s want invalid", st)
	}

	// «Ha, men» replayed after the cancel changes nothing.
	edits := len(fake.allEdits())
	if err := b.HandleUpdate(ctx, callback(2121, yes)); err != nil {
		t.Fatal(err)
	}
	if len(fake.allEdits()) != edits {
		t.Fatal("stale tap edited the message")
	}
	if st := svc.PasswordResetStatus(ctx, raw).State; st != auth.ResetStateInvalid {
		t.Fatalf("status after replay=%s want invalid", st)
	}
}

// callback is a private-chat inline button tap by tgUserID.
func callback(tgUserID int64, data string) Update {
	return Update{
		UpdateID: 9,
		CallbackQuery: &CallbackQuery{
			ID:      "cq",
			From:    User{ID: tgUserID},
			Message: &Message{MessageID: 500, Chat: Chat{ID: tgUserID, Type: "private"}},
			Data:    data,
		},
	}
}

// confirmButtons returns the «Ha, men» / «Yo'q» callback_data of the last
// message sent.
func (f *fakeTelegram) confirmButtons(t *testing.T) (yes, no string) {
	t.Helper()
	var m InlineKeyboardMarkup
	if err := json.Unmarshal([]byte(f.lastMarkup()), &m); err != nil {
		t.Fatalf("markup=%q: %v", f.lastMarkup(), err)
	}
	if len(m.InlineKeyboard) != 1 || len(m.InlineKeyboard[0]) != 2 {
		t.Fatalf("markup=%+v want one row of two buttons", m)
	}
	y, n := m.InlineKeyboard[0][0], m.InlineKeyboard[0][1]
	if y.Text != "Ha, men" || n.Text != "Yo'q" {
		t.Fatalf("buttons=%q/%q", y.Text, n.Text)
	}
	if len(y.CallbackData) > 64 || len(n.CallbackData) > 64 {
		t.Fatal("callback_data over Telegram's 64-byte limit")
	}
	return y.CallbackData, n.CallbackData
}

func (f *fakeTelegram) allEdits() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.edits...)
}

func (f *fakeTelegram) lastEdit() string {
	e := f.allEdits()
	if len(e) == 0 {
		return ""
	}
	return e[len(e)-1]
}

func (f *fakeTelegram) lastAnswer() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.answers) == 0 {
		return ""
	}
	return f.answers[len(f.answers)-1]
}

// Sharing a phone from the Mini App (WebApp.requestContact) sends the contact
// to this chat too; with no reset pending the bot must stay quiet instead of
// answering "link invalid" to someone who never asked for a reset.
func TestHandleUpdate_ContactWithoutPendingResetIsSilent(t *testing.T) {
	b, q, fake := newTestBot(t)
	attachAuth(t, b, q)
	u := Update{
		UpdateID: 3,
		Message: &Message{
			From:    &User{ID: 2222, Username: "miniapp"},
			Chat:    Chat{ID: 2222, Type: "private"},
			Contact: &Contact{PhoneNumber: "998901000022", UserID: 2222},
		},
	}
	if err := b.HandleUpdate(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if got := fake.allMessages(); len(got) != 0 {
		t.Fatalf("bot replied %q", got)
	}
}

func (f *fakeTelegram) lastMarkup() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.markups) == 0 {
		return ""
	}
	return f.markups[len(f.markups)-1]
}

func TestHandleUpdate_StartPrivateShowsWebAppButton(t *testing.T) {
	b, _, fake := newTestBot(t)
	b.WebAppURL = "https://drivergo.uz/uz-Latn/tg"
	u := update("/start", 301, "mini")
	u.Message.Chat.Type = "private"
	if err := b.HandleUpdate(context.Background(), u); err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	var m InlineKeyboardMarkup
	if err := json.Unmarshal([]byte(fake.lastMarkup()), &m); err != nil {
		t.Fatalf("markup = %q: %v", fake.lastMarkup(), err)
	}
	btn := m.InlineKeyboard[0][0]
	if btn.WebApp == nil || btn.WebApp.URL != "https://drivergo.uz/uz-Latn/tg" || btn.Text != "📱 DriverGo'ni ochish" {
		t.Fatalf("button = %+v", btn)
	}
}

func TestHandleUpdate_StartWithoutWebAppURLHasNoMarkup(t *testing.T) {
	b, _, fake := newTestBot(t)
	if err := b.HandleUpdate(context.Background(), update("/start", 302, "plain")); err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if mk := fake.lastMarkup(); mk != "" && mk != "null" {
		t.Errorf("markup = %q, want none", mk)
	}
}

func TestHandleUpdate_StartPayloadAndGroupGetNoWebAppButton(t *testing.T) {
	b, _, fake := newTestBot(t)
	b.WebAppURL = "https://drivergo.uz/uz-Latn/tg"

	// Link payload flow stays unchanged.
	if err := b.HandleUpdate(context.Background(), update("/start no-such-token", 303, "p")); err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if strings.Contains(fake.lastMarkup(), "web_app") {
		t.Errorf("payload /start markup = %q, want no web_app", fake.lastMarkup())
	}

	// Telegram rejects web_app buttons in groups.
	g := update("/start", 304, "g")
	g.Message.Chat = Chat{ID: -1002, Type: "supergroup", Title: "G"}
	if err := b.HandleUpdate(context.Background(), g); err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if strings.Contains(fake.lastMarkup(), "web_app") {
		t.Errorf("group /start markup = %q, want no web_app", fake.lastMarkup())
	}
}
