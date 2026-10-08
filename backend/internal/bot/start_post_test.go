package bot

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

	"avtotest.uz/backend/internal/db/sqlc"
)

const testWebAppURL = "https://drivergo.uz/uz-Latn/tg"

func privateUpdate(text string, tgUserID int64, firstName, lang string) Update {
	return Update{UpdateID: 1, Message: &Message{
		Text: text,
		From: &User{ID: tgUserID, FirstName: firstName, LanguageCode: lang},
		Chat: Chat{ID: tgUserID, Type: "private"},
	}}
}

func (f *fakeTelegram) callsTo(method string) []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []fakeCall
	for _, c := range f.calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// keyboardOf re-decodes a recorded reply_markup into the typed keyboard.
func keyboardOf(t *testing.T, body map[string]any) InlineKeyboardMarkup {
	t.Helper()
	raw, err := json.Marshal(body["reply_markup"])
	if err != nil {
		t.Fatal(err)
	}
	var m InlineKeyboardMarkup
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("reply_markup %s: %v", raw, err)
	}
	return m
}

func nextOf(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("button url %q: %v", raw, err)
	}
	return u.Query().Get("next")
}

func newPromoBot(t *testing.T) (*Bot, *sqlc.Queries, *fakeTelegram) {
	t.Helper()
	b, q, fake := newTestBot(t)
	b.WebAppURL = testWebAppURL
	b.BotUsername = "DriverGouzBot"
	b.PublicBaseURL = "https://drivergo.uz"
	return b, q, fake
}

func TestStartPost_UnlinkedPrivateGetsBannerCaptionAndMenu(t *testing.T) {
	b, _, fake := newPromoBot(t)
	if err := b.HandleUpdate(context.Background(), privateUpdate("/start", 501, "Ali", "uz")); err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	photos := fake.callsTo("sendPhoto")
	if len(photos) != 1 || len(fake.callsTo("sendMessage")) != 0 {
		t.Fatalf("calls = %+v, want exactly one sendPhoto", fake.calls)
	}
	body := photos[0].Body
	if body["photo"] != "https://drivergo.uz/bot/start-banner.jpg?v=1" {
		t.Errorf("photo = %v", body["photo"])
	}
	if body["parse_mode"] != "HTML" {
		t.Errorf("parse_mode = %v, want HTML", body["parse_mode"])
	}
	caption, _ := body["caption"].(string)
	for _, want := range []string{"Ali", "24 soat", "VIP"} {
		if !strings.Contains(caption, want) {
			t.Errorf("caption missing %q:\n%s", want, caption)
		}
	}

	kb := keyboardOf(t, body).InlineKeyboard
	if len(kb) != 5 {
		t.Fatalf("rows = %d, want 5: %+v", len(kb), kb)
	}
	open := kb[0][0]
	if open.Text != "📱 Driver Go'ni ochish" || open.WebApp == nil || open.WebApp.URL != testWebAppURL {
		t.Errorf("row1 = %+v", open)
	}
	wantNext := [][]string{
		nil,
		{"/uz-Latn/tickets", "/uz-Latn/exam"},
		{"/uz-Latn/practice", "/uz-Latn/signs"},
		{"/uz-Latn/premium", "/uz-Latn/support"},
	}
	for row := 1; row <= 3; row++ {
		if len(kb[row]) != 2 {
			t.Fatalf("row %d = %+v, want 2 buttons", row+1, kb[row])
		}
		for col, btn := range kb[row] {
			if btn.WebApp == nil || btn.URL != "" {
				t.Fatalf("row %d btn %d = %+v, want a web_app button", row+1, col, btn)
			}
			if !strings.HasPrefix(btn.WebApp.URL, testWebAppURL+"?") {
				t.Errorf("btn %q url = %q", btn.Text, btn.WebApp.URL)
			}
			if got := nextOf(t, btn.WebApp.URL); got != wantNext[row][col] {
				t.Errorf("btn %q next = %q, want %q", btn.Text, got, wantNext[row][col])
			}
		}
	}
	if kb[1][0].Text != "🎫 Biletlar" || kb[3][1].Text != "💬 Yordam" {
		t.Errorf("labels = %q / %q", kb[1][0].Text, kb[3][1].Text)
	}
	group := kb[4][0]
	if group.URL != "https://t.me/DriverGouzBot?startgroup=quiz" || group.WebApp != nil {
		t.Errorf("row5 = %+v", group)
	}
}

func TestStartPost_LinkedLearnerGetsWelcomeBack(t *testing.T) {
	b, q, fake := newPromoBot(t)
	ctx := context.Background()
	if err := b.HandleUpdate(ctx, privateUpdate("/start", 502, "Vali", "uz")); err != nil {
		t.Fatal(err)
	}
	unlinked := fake.callsTo("sendPhoto")[0].Body["caption"].(string)

	tok, err := b.Link.GenerateLinkToken(ctx, createProfile(t, q, "+998901150502"))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.HandleUpdate(ctx, update("/start "+tok.Token, 502, "vali")); err != nil {
		t.Fatal(err)
	}
	if err := b.HandleUpdate(ctx, privateUpdate("/start", 502, "Vali", "uz")); err != nil {
		t.Fatal(err)
	}
	photos := fake.callsTo("sendPhoto")
	linked := photos[len(photos)-1].Body["caption"].(string)
	if linked == unlinked {
		t.Fatal("linked learner got the unlinked caption")
	}
	if strings.Contains(linked, "24 soat") {
		t.Errorf("linked caption still pitches the signup trial:\n%s", linked)
	}
	if !strings.Contains(linked, "Vali") || !strings.Contains(linked, "qaytganingiz") {
		t.Errorf("linked caption = %q, want a welcome-back greeting", linked)
	}
}

func TestStartPost_RussianLanguageUsesRuLocale(t *testing.T) {
	b, _, fake := newPromoBot(t)
	if err := b.HandleUpdate(context.Background(), privateUpdate("/start", 503, "Иван", "ru-RU")); err != nil {
		t.Fatal(err)
	}
	body := fake.callsTo("sendPhoto")[0].Body
	if body["photo"] != "https://drivergo.uz/bot/start-banner-ru.jpg?v=1" {
		t.Errorf("ru photo = %v", body["photo"])
	}
	caption := body["caption"].(string)
	if !strings.Contains(caption, "Иван") || !strings.Contains(caption, "24 часа") {
		t.Errorf("ru caption = %q", caption)
	}
	kb := keyboardOf(t, body).InlineKeyboard
	if kb[0][0].WebApp == nil || kb[0][0].WebApp.URL != "https://drivergo.uz/ru/tg" {
		t.Errorf("ru open = %+v", kb[0][0])
	}
	if got := nextOf(t, kb[1][0].WebApp.URL); got != "/ru/tickets" || !strings.HasPrefix(kb[1][0].WebApp.URL, "https://drivergo.uz/ru/tg?") {
		t.Errorf("ru tickets = %q (next %q)", kb[1][0].WebApp.URL, got)
	}
	if kb[1][0].Text != "🎫 Билеты" {
		t.Errorf("ru label = %q", kb[1][0].Text)
	}
}

func TestStartPost_WithoutMiniAppFallsBackToWebsiteLinks(t *testing.T) {
	b, _, fake := newPromoBot(t)
	b.WebAppURL = ""
	if err := b.HandleUpdate(context.Background(), privateUpdate("/start", 504, "Ali", "")); err != nil {
		t.Fatal(err)
	}
	body := fake.callsTo("sendPhoto")[0].Body
	kb := keyboardOf(t, body).InlineKeyboard
	if strings.Contains(mustJSON(t, body["reply_markup"]), "web_app") {
		t.Fatalf("markup has web_app with the Mini App disabled: %v", body["reply_markup"])
	}
	if kb[0][0].URL != "https://drivergo.uz/uz-Latn" {
		t.Errorf("open = %+v", kb[0][0])
	}
	if kb[1][0].URL != "https://drivergo.uz/uz-Latn/tickets" || kb[3][1].URL != "https://drivergo.uz/uz-Latn/support" {
		t.Errorf("site links = %+v / %+v", kb[1][0], kb[3][1])
	}
}

func TestStartPost_NoBotUsernameDropsGroupRow(t *testing.T) {
	b, _, fake := newPromoBot(t)
	b.BotUsername = ""
	if err := b.HandleUpdate(context.Background(), privateUpdate("/start", 505, "Ali", "uz")); err != nil {
		t.Fatal(err)
	}
	kb := keyboardOf(t, fake.callsTo("sendPhoto")[0].Body).InlineKeyboard
	if len(kb) != 4 || strings.Contains(mustJSON(t, kb), "startgroup") {
		t.Errorf("keyboard = %+v, want 4 rows and no startgroup link", kb)
	}
}

func TestStartPost_PhotoRejectedFallsBackToText(t *testing.T) {
	b, _, fake := newPromoBot(t)
	fake.failPhotoCode = 400
	if err := b.HandleUpdate(context.Background(), privateUpdate("/start", 506, "Ali", "uz")); err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	photo := fake.callsTo("sendPhoto")
	msgs := fake.callsTo("sendMessage")
	if len(photo) != 1 || len(msgs) != 1 {
		t.Fatalf("photo=%d messages=%d, want 1 and 1", len(photo), len(msgs))
	}
	if msgs[0].Body["text"] != photo[0].Body["caption"] || msgs[0].Body["parse_mode"] != "HTML" {
		t.Errorf("fallback body = %v", msgs[0].Body)
	}
	if mustJSON(t, msgs[0].Body["reply_markup"]) != mustJSON(t, photo[0].Body["reply_markup"]) {
		t.Errorf("fallback keyboard differs")
	}
}

func TestStartPost_PhotoRetryableFailureIsReturned(t *testing.T) {
	b, _, fake := newPromoBot(t)
	fake.failPhotoCode = 429
	if err := b.HandleUpdate(context.Background(), privateUpdate("/start", 507, "Ali", "uz")); err == nil {
		t.Fatal("a 429 on sendPhoto must be returned so Telegram retries")
	}
	if n := len(fake.callsTo("sendMessage")); n != 0 {
		t.Errorf("sent %d text fallbacks for a retryable failure", n)
	}
}

func TestStartPost_EscapesNameAndFitsCaptionLimit(t *testing.T) {
	b, _, fake := newPromoBot(t)
	name := "<b>" + strings.Repeat("Ж", 300) + "&"
	if err := b.HandleUpdate(context.Background(), privateUpdate("/start", 508, name, "ru")); err != nil {
		t.Fatal(err)
	}
	caption := fake.callsTo("sendPhoto")[0].Body["caption"].(string)
	if strings.Contains(caption, "<b><b>") || strings.Contains(caption, "<b>ЖЖ") {
		t.Errorf("name was not escaped: %q", caption[:80])
	}
	if !strings.Contains(caption, "&lt;b&gt;") {
		t.Errorf("caption = %q, want an escaped name", caption[:80])
	}
	for _, c := range []string{caption, startCaption(langUz, name, false), startCaption(langUz, name, true), startCaption(langRu, name, true)} {
		if n := utf8.RuneCountInString(c); n > 1024 {
			t.Errorf("caption is %d chars, Telegram allows 1024", n)
		}
	}
}

func TestStartPost_MissingFirstNameUsesNeutralGreeting(t *testing.T) {
	b, _, fake := newPromoBot(t)
	if err := b.HandleUpdate(context.Background(), privateUpdate("/start", 509, "  ", "uz")); err != nil {
		t.Fatal(err)
	}
	if c := fake.callsTo("sendPhoto")[0].Body["caption"].(string); !strings.Contains(c, "<b>do'stim</b>") {
		t.Errorf("caption = %q", c)
	}
	// Telegram HTML needs only <, > and & escaped; Uzbek apostrophes stay.
	if c := startCaption(langUz, "O'g'iloy", false); !strings.Contains(c, "<b>O'g'iloy</b>") {
		t.Errorf("caption = %q", c)
	}
}

func TestHelp_PrivateShowsGuideAndMenu(t *testing.T) {
	b, _, fake := newPromoBot(t)
	if err := b.HandleUpdate(context.Background(), privateUpdate("/help", 510, "Ali", "uz")); err != nil {
		t.Fatal(err)
	}
	msgs := fake.callsTo("sendMessage")
	if len(msgs) != 1 {
		t.Fatalf("messages = %d", len(msgs))
	}
	text := msgs[0].Body["text"].(string)
	for _, want := range []string{"/quiz", "/status", "Biletlar"} {
		if !strings.Contains(text, want) {
			t.Errorf("help missing %q:\n%s", want, text)
		}
	}
	if msgs[0].Body["parse_mode"] != "HTML" {
		t.Errorf("parse_mode = %v", msgs[0].Body["parse_mode"])
	}
	if kb := keyboardOf(t, msgs[0].Body).InlineKeyboard; len(kb) != 5 || kb[0][0].WebApp == nil {
		t.Errorf("help keyboard = %+v", kb)
	}
}

func TestHelp_GroupGetsQuizHelpWithoutWebApp(t *testing.T) {
	b, _, fake := newPromoBot(t)
	u := update("/help@DriverGouzBot", 511, "g")
	u.Message.Chat = Chat{ID: -1003, Type: "group", Title: "G"}
	if err := b.HandleUpdate(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fake.lastMessage(), "/quiz") || strings.Contains(fake.lastMarkup(), "web_app") {
		t.Errorf("group help = %q markup %q", fake.lastMessage(), fake.lastMarkup())
	}
}

func TestUnknownCommandPointsToHelp(t *testing.T) {
	b, _, fake := newPromoBot(t)
	if err := b.HandleUpdate(context.Background(), privateUpdate("/bogus", 512, "Ali", "uz")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fake.lastMessage(), "/help") {
		t.Errorf("unknown reply = %q, want a /help pointer", fake.lastMessage())
	}
	if err := b.HandleUpdate(context.Background(), privateUpdate("/bogus", 512, "Ali", "ru")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fake.lastMessage(), "/help") || !strings.Contains(fake.lastMessage(), "Неизвестная") {
		t.Errorf("ru unknown reply = %q", fake.lastMessage())
	}
}

// Every command advertised in Telegram's menu must actually be handled:
// none of them may land on the unknown-command reply.
func TestAdvertisedCommandsAreHandled(t *testing.T) {
	b, _, fake := newPromoBot(t)
	ctx := context.Background()
	for _, set := range commandSets() {
		for i, cmd := range set.Commands {
			u := privateUpdate("/"+cmd.Command, int64(600+i), "Ali", set.LanguageCode)
			if set.Scope == scopeAllGroupChats {
				u.Message.Chat = Chat{ID: int64(-2000 - i), Type: "supergroup", Title: "G"}
			}
			before := len(fake.allMessages())
			_ = b.HandleUpdate(ctx, u)
			for _, m := range fake.allMessages()[before:] {
				if strings.Contains(m, "Noma'lum buyruq") || strings.Contains(m, "Неизвестная") {
					t.Errorf("/%s (%s) answered as unknown", cmd.Command, set.Scope)
				}
			}
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
