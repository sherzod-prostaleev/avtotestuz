package bot

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/db/sqlc"
)

func TestPasswordNotifierTellsTheLinkedChat(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	svc := attachAuth(t, b, q)
	n := &PasswordNotifier{Q: q, TG: b.TG}

	reg, err := svc.Register(ctx, auth.RegisterInput{Phone: "901350101", Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	// No Telegram link: nobody to tell, and no error.
	n.FirstPasswordSet(ctx, reg.Profile.ID)
	n.FirstPasswordSet(ctx, uuid.New())
	if len(fake.allMessages()) != 0 {
		t.Fatalf("sent without a link: %q", fake.allMessages())
	}

	if err := q.UpsertTelegramAccount(ctx, sqlc.UpsertTelegramAccountParams{ProfileID: reg.Profile.ID, TgUserID: 3601, PhoneVerified: true}); err != nil {
		t.Fatal(err)
	}
	n.FirstPasswordSet(ctx, reg.Profile.ID)
	if got := fake.lastMessage(); !strings.Contains(got, "Hisobingizga parol o'rnatildi.") ||
		!strings.Contains(got, "Bu siz bo'lmasangiz — /start orqali yordamga yozing.") {
		t.Fatalf("uz notice=%q", got)
	}
	sent := fake.callsTo("sendMessage")
	if chat, _ := sent[len(sent)-1].Body["chat_id"].(float64); int64(chat) != 3601 {
		t.Fatalf("sent to chat %v, want the linked Telegram user", sent[len(sent)-1].Body["chat_id"])
	}

	// A user the bot knows as Russian-speaking gets it in Russian.
	if err := b.HandleUpdate(ctx, privateUpdate("/help", 3601, "Ali", "ru")); err != nil {
		t.Fatal(err)
	}
	n.FirstPasswordSet(ctx, reg.Profile.ID)
	if got := fake.lastMessage(); !strings.Contains(got, "установлен пароль") || !strings.Contains(got, "/start") {
		t.Fatalf("ru notice=%q", got)
	}

	// Telegram refusing the message (bot blocked) is swallowed.
	fake.failSend = true
	n.FirstPasswordSet(ctx, reg.Profile.ID)
}
