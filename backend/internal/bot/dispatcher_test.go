package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

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
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "sendMessage") {
			var body struct {
				Text string `json:"text"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			f.sent = append(f.sent, body.Text)
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

func newTestBot(t *testing.T) (*Bot, *sqlc.Queries, *fakeTelegram) {
	t.Helper()
	pool := testdb.New(t)
	q := sqlc.New(pool)
	fake, client := newFakeTelegram(t)
	b := &Bot{
		Link:     NewLinkService(pool, q),
		Billing:  billing.Service{Q: q},
		Progress: progress.NewService(q),
		TG:       client,
	}
	return b, q, fake
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

func TestHandleUpdate_IgnoresNonMessageUpdates(t *testing.T) {
	b, _, fake := newTestBot(t)
	if err := b.HandleUpdate(context.Background(), Update{UpdateID: 1}); err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if fake.lastMessage() != "" {
		t.Errorf("expected no reply for a message-less update, got %q", fake.lastMessage())
	}
}
