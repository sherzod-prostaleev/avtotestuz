package bot

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/db/sqlc"
)

func dmUpdate(text string, from User) Update {
	return Update{UpdateID: 1, Message: &Message{
		MessageID: 10, Text: text, From: &from,
		Chat: Chat{ID: from.ID, Type: "private"},
	}}
}

func botUser(t *testing.T, q *sqlc.Queries, id int64) (sqlc.TelegramBotUser, bool) {
	t.Helper()
	u, err := q.GetTelegramBotUser(context.Background(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return u, true
}

func TestRegistryRecordsPrivateChatUsersOnly(t *testing.T) {
	b, q, _ := newTestBot(t)
	ctx := context.Background()
	from := User{ID: 7001, FirstName: "Aziz", Username: "aziz", LanguageCode: "ru"}
	if err := b.HandleUpdate(ctx, dmUpdate("/help", from)); err != nil {
		t.Fatal(err)
	}
	got, ok := botUser(t, q, 7001)
	if !ok {
		t.Fatal("private /help did not register the user")
	}
	if got.FirstName != "Aziz" || got.Username != "aziz" || got.LanguageCode != "ru" || !got.RemindersEnabled {
		t.Fatalf("row = %+v", got)
	}

	group := Update{Message: &Message{Text: "/quiz@bot", From: &User{ID: 7002},
		Chat: Chat{ID: -100500, Type: "supergroup"}}}
	_ = b.HandleUpdate(ctx, group)
	if _, ok := botUser(t, q, 7002); ok {
		t.Fatal("a group message must not add the sender to the DM audience")
	}

	cb := Update{CallbackQuery: &CallbackQuery{ID: "c1", From: User{ID: 7003, FirstName: "Lola"},
		Data: "unknown", Message: &Message{MessageID: 3, Chat: Chat{ID: 7003, Type: "private"}}}}
	_ = b.HandleUpdate(ctx, cb)
	if _, ok := botUser(t, q, 7003); !ok {
		t.Fatal("a private callback must register the user")
	}
}

func myChatMember(id int64, status string) Update {
	return Update{MyChatMember: &ChatMemberUpd{
		Chat:          Chat{ID: id, Type: "private"},
		From:          User{ID: id, FirstName: "Ali"},
		NewChatMember: ChatMember{Status: status},
	}}
}

func TestRegistryBlockAndUnblockViaMyChatMember(t *testing.T) {
	b, q, _ := newTestBot(t)
	ctx := context.Background()
	if err := b.HandleUpdate(ctx, myChatMember(7101, "kicked")); err != nil {
		t.Fatal(err)
	}
	got, ok := botUser(t, q, 7101)
	if !ok || !got.BlockedAt.Valid {
		t.Fatalf("kicked: row=%+v ok=%v, want blocked_at set", got, ok)
	}
	if err := b.HandleUpdate(ctx, myChatMember(7101, "member")); err != nil {
		t.Fatal(err)
	}
	if got, _ := botUser(t, q, 7101); got.BlockedAt.Valid {
		t.Fatal("member again must clear blocked_at")
	}
}

func TestEslatmaTogglesReminders(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	from := User{ID: 7201, LanguageCode: "uz"}
	if err := b.HandleUpdate(ctx, dmUpdate("/eslatma", from)); err != nil {
		t.Fatal(err)
	}
	if got, _ := botUser(t, q, 7201); got.RemindersEnabled {
		t.Fatal("first /eslatma must switch reminders off")
	}
	if msg := fake.lastMessage(); !strings.Contains(msg, "o'chirildi") || !strings.Contains(msg, "/eslatma") {
		t.Fatalf("off reply = %q", msg)
	}
	if err := b.HandleUpdate(ctx, dmUpdate("/eslatma", from)); err != nil {
		t.Fatal(err)
	}
	if got, _ := botUser(t, q, 7201); !got.RemindersEnabled {
		t.Fatal("second /eslatma must switch reminders back on")
	}
	if msg := fake.lastMessage(); !strings.Contains(msg, "yoqildi") || !strings.Contains(msg, "19:00") {
		t.Fatalf("on reply = %q", msg)
	}
}

func TestOptOutCallbackDisablesAndEditsMessage(t *testing.T) {
	b, q, fake := newTestBot(t)
	ctx := context.Background()
	cb := Update{CallbackQuery: &CallbackQuery{ID: "c9", From: User{ID: 7301, LanguageCode: "ru"},
		Data:    cbReminderOff,
		Message: &Message{MessageID: 44, Text: "🧠 ...", Chat: Chat{ID: 7301, Type: "private"}}}}
	if err := b.HandleUpdate(ctx, cb); err != nil {
		t.Fatal(err)
	}
	if got, _ := botUser(t, q, 7301); got.RemindersEnabled {
		t.Fatal("opt-out button must disable reminders")
	}
	fake.mu.Lock()
	edits := append([]string(nil), fake.edits...)
	fake.mu.Unlock()
	if len(edits) != 1 || !strings.Contains(edits[0], "/eslatma") || !strings.Contains(edits[0], "отключены") {
		t.Fatalf("edits = %q, want a ru confirmation naming /eslatma", edits)
	}
}

// A bundle with an image carries its buttons on the photo, so the
// confirmation replaces the caption, not the (absent) text.
func TestOptOutCallbackOnPhotoEditsCaption(t *testing.T) {
	b, _, fake := newTestBot(t)
	ctx := context.Background()
	cb := Update{CallbackQuery: &CallbackQuery{ID: "c10", From: User{ID: 7302},
		Data:    cbReminderOff,
		Message: &Message{MessageID: 45, Caption: "🧠 Kun savoli", Chat: Chat{ID: 7302, Type: "private"}}}}
	if err := b.HandleUpdate(ctx, cb); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	found := false
	for _, c := range fake.calls {
		if c.Method == "editMessageCaption" {
			found = strings.Contains(c.Body["caption"].(string), "/eslatma")
		}
	}
	if !found {
		t.Fatal("photo bundle opt-out must edit the caption")
	}
}

func TestHelpAndCommandMenuMentionEslatma(t *testing.T) {
	if !strings.Contains(helpUz, "/eslatma") || !strings.Contains(helpRu, "/eslatma") {
		t.Fatal("/help must mention /eslatma in both languages")
	}
	for _, set := range commandSets() {
		if set.Scope != scopeAllPrivateChats {
			continue
		}
		found := false
		for _, c := range set.Commands {
			if c.Command == "eslatma" {
				found = true
				if n := len([]rune(c.Description)); n > 30 {
					t.Errorf("eslatma description %q is %d chars", c.Description, n)
				}
			}
		}
		if !found {
			t.Errorf("private command set %q lacks eslatma", set.LanguageCode)
		}
	}
}
