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

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/progress"
	"avtotest.uz/backend/internal/testdb"
)

// fakeTelegram stands in for api.telegram.org and records every
// sendMessage call, so dispatcher tests assert on the reply text without
// touching the network.
type fakeTelegram struct {
	mu   sync.Mutex
	sent []string
	srv  *httptest.Server
}

func newFakeTelegram(t *testing.T) (*fakeTelegram, *Client) {
	t.Helper()
	f := &fakeTelegram{}
	msgID := int64(100)
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.Contains(path, "sendMessage") || strings.Contains(path, "sendPhoto") {
			var body struct {
				Text    string `json:"text"`
				Caption string `json:"caption"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			text := body.Text
			if text == "" {
				text = body.Caption
			}
			f.mu.Lock()
			f.sent = append(f.sent, text)
			f.mu.Unlock()
			msgID++
			_, _ = w.Write([]byte(fmt.Sprintf(`{"ok":true,"result":{"message_id":%d}}`, msgID)))
			return
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
