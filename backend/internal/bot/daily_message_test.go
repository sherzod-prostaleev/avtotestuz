package bot

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/db/sqlc"
)

var dailyToday = time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)

func activeOn(daysAgo int) pgtype.Date {
	return pgtype.Date{Time: dailyToday.AddDate(0, 0, -daysAgo), Valid: true}
}

func TestPickPersonalLinePriority(t *testing.T) {
	cases := []struct {
		name string
		row  sqlc.ListTelegramReminderAudienceRow
		want personalLine
	}{
		{"unlinked gets the signup pitch",
			sqlc.ListTelegramReminderAudienceRow{}, lineSignup},
		{"streak beats due",
			sqlc.ListTelegramReminderAudienceRow{Linked: true, PhoneVerified: true, StreakCurrent: 5, LastActiveDate: activeOn(1), DueCount: 7}, lineStreak},
		{"streak counts when active today too",
			sqlc.ListTelegramReminderAudienceRow{Linked: true, PhoneVerified: true, StreakCurrent: 2, LastActiveDate: activeOn(0)}, lineStreak},
		{"streak needs a verified phone",
			sqlc.ListTelegramReminderAudienceRow{Linked: true, StreakCurrent: 5, LastActiveDate: activeOn(1), DueCount: 3}, lineDue},
		{"a one-day streak is not worth a line",
			sqlc.ListTelegramReminderAudienceRow{Linked: true, PhoneVerified: true, StreakCurrent: 1, LastActiveDate: activeOn(0)}, lineGeneric},
		{"a broken streak is not a streak",
			sqlc.ListTelegramReminderAudienceRow{Linked: true, PhoneVerified: true, StreakCurrent: 9, LastActiveDate: activeOn(2)}, lineGeneric},
		{"due beats inactive",
			sqlc.ListTelegramReminderAudienceRow{Linked: true, DueCount: 4, LastActiveDate: activeOn(10)}, lineDue},
		{"inactive three days",
			sqlc.ListTelegramReminderAudienceRow{Linked: true, LastActiveDate: activeOn(3)}, lineInactive},
		{"never active linked learner is inactive",
			sqlc.ListTelegramReminderAudienceRow{Linked: true}, lineInactive},
		{"two days off is still generic",
			sqlc.ListTelegramReminderAudienceRow{Linked: true, LastActiveDate: activeOn(2)}, lineGeneric},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pickPersonalLine(tc.row, dailyToday); got != tc.want {
				t.Fatalf("pickPersonalLine = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPersonalLineTextUzAndRu(t *testing.T) {
	streak := sqlc.ListTelegramReminderAudienceRow{StreakCurrent: 5}
	if got := personalLineText(lineStreak, langUz, streak); got != "🔥 5 kunlik seriyangiz bor — bugun uzib qo'ymang!" {
		t.Fatalf("uz streak = %q", got)
	}
	if got := personalLineText(lineStreak, langRu, streak); !strings.Contains(got, "5 дней") {
		t.Fatalf("ru streak = %q, want plural «5 дней»", got)
	}
	due := sqlc.ListTelegramReminderAudienceRow{DueCount: 3}
	if got := personalLineText(lineDue, langUz, due); got != "📚 Bugun 3 ta savolni takrorlash navbati" {
		t.Fatalf("uz due = %q", got)
	}
	if got := personalLineText(lineDue, langRu, sqlc.ListTelegramReminderAudienceRow{DueCount: 21}); !strings.HasSuffix(got, " 21 вопрос") {
		t.Fatalf("ru due = %q, want «21 вопрос»", got)
	}
	if got := personalLineText(lineSignup, langUz, sqlc.ListTelegramReminderAudienceRow{}); got != "🎁 Ro'yxatdan o'ting — 24 soat bepul VIP" {
		t.Fatalf("uz signup = %q", got)
	}
	if got := personalLineText(lineSignup, langRu, sqlc.ListTelegramReminderAudienceRow{}); !strings.Contains(got, "24 часа") {
		t.Fatalf("ru signup = %q", got)
	}
}

// The comeback line quotes real progress or none at all.
func TestInactiveLineNeverInventsNumbers(t *testing.T) {
	with := personalLineText(lineInactive, langUz, sqlc.ListTelegramReminderAudienceRow{TicketsCompleted: 7})
	if !strings.Contains(with, "7 ta bilet") {
		t.Fatalf("uz inactive with tickets = %q", with)
	}
	without := personalLineText(lineInactive, langUz, sqlc.ListTelegramReminderAudienceRow{})
	for _, r := range without {
		if r >= '0' && r <= '9' {
			t.Fatalf("uz inactive without progress quotes a number: %q", without)
		}
	}
	ru := personalLineText(lineInactive, langRu, sqlc.ListTelegramReminderAudienceRow{TicketsCompleted: 2})
	if !strings.Contains(ru, "2 билета") {
		t.Fatalf("ru inactive = %q", ru)
	}
}

func TestRuPlural(t *testing.T) {
	for n, want := range map[int]string{1: "день", 2: "дня", 4: "дня", 5: "дней", 11: "дней", 12: "дней", 21: "день", 22: "дня", 111: "дней"} {
		if got := ruPlural(n, "день", "дня", "дней"); got != want {
			t.Errorf("ruPlural(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestDailyKeyboardOpensMiniAppPractice(t *testing.T) {
	kb := dailyKeyboard("https://drivergo.uz/uz-Latn/tg", "https://drivergo.uz", langRu)
	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("rows = %d, want practice + opt-out", len(kb.InlineKeyboard))
	}
	practice := kb.InlineKeyboard[0][0]
	if practice.WebApp == nil || practice.URL != "" {
		t.Fatalf("practice button = %+v, want a web_app button", practice)
	}
	u, err := url.Parse(practice.WebApp.URL)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/ru/tg" || u.Query().Get("next") != "/ru/practice" {
		t.Fatalf("practice url = %s", practice.WebApp.URL)
	}
	off := kb.InlineKeyboard[1][0]
	if off.CallbackData != cbReminderOff || !strings.Contains(off.Text, "Отключить") {
		t.Fatalf("opt-out button = %+v", off)
	}
}

func TestDailyKeyboardFallsBackToSiteWhenMiniAppDisabled(t *testing.T) {
	kb := dailyKeyboard("", "https://drivergo.uz/", langUz)
	practice := kb.InlineKeyboard[0][0]
	if practice.WebApp != nil || practice.URL != "https://drivergo.uz/uz-Latn/practice" {
		t.Fatalf("practice button = %+v, want a plain site link", practice)
	}
	if practice.Text != "📝 Bugungi mashq" {
		t.Fatalf("practice text = %q", practice.Text)
	}
	if kb.InlineKeyboard[1][0].Text != "🔕 Eslatmalarni o'chirish" {
		t.Fatalf("opt-out text = %q", kb.InlineKeyboard[1][0].Text)
	}
}

func TestDailyMessagesCarryHeaderAndLine(t *testing.T) {
	line := "📚 Bugun 3 ta savolni takrorlash navbati"
	if got := dailyCaption(langUz, line); !strings.HasPrefix(got, "🧠 Kun savoli") || !strings.HasSuffix(got, line) {
		t.Fatalf("caption = %q", got)
	}
	if got := dailyFollowUp(langRu, "x"); !strings.Contains(got, "Вопрос дня") {
		t.Fatalf("ru follow-up = %q", got)
	}
}

// The poll keeps its full text or the question is skipped — never cut.
func TestDailyPollRejectsQuestionsOverLimits(t *testing.T) {
	ok := []sqlc.ListQuizAnswersRow{answerRow(1, true, "Ha"), answerRow(2, false, "Yo'q")}
	if _, err := buildPollRequest(strings.Repeat("s", 301), ok, "", 0, 0); err == nil {
		t.Fatal("301-char question must not fit")
	}
	long := []sqlc.ListQuizAnswersRow{answerRow(1, true, strings.Repeat("a", 101)), answerRow(2, false, "b")}
	if _, err := buildPollRequest("Savol?", long, "", 0, 0); err == nil {
		t.Fatal("101-char option must not fit")
	}
	eleven := make([]sqlc.ListQuizAnswersRow, 11)
	for i := range eleven {
		eleven[i] = answerRow(int16(i+1), i == 0, "v")
	}
	if _, err := buildPollRequest("Savol?", eleven, "", 0, 0); err == nil {
		t.Fatal("11 options must not fit")
	}
}

func TestExplanationForPollOnlyWhenItFits(t *testing.T) {
	short := []byte(`[{"type":"muhim","text":"Qisqa izoh."}]`)
	if got := explanationForPoll(short); got != "Qisqa izoh." {
		t.Fatalf("short = %q", got)
	}
	long := []byte(`[{"type":"muhim","text":"` + strings.Repeat("x", 201) + `"}]`)
	if got := explanationForPoll(long); got != "" {
		t.Fatalf("over-limit explanation must be dropped, not cut: %q", got)
	}
}

func TestSendWindowIsTashkentEvening(t *testing.T) {
	for _, tc := range []struct {
		h, m int
		want bool
	}{{8, 59, false}, {9, 0, false}, {18, 59, false}, {19, 0, true}, {20, 59, true}, {21, 0, false}, {23, 0, false}, {3, 0, false}} {
		now := time.Date(2026, 10, 9, tc.h, tc.m, 0, 0, tashkent)
		if got := inSendWindow(now); got != tc.want {
			t.Errorf("inSendWindow(%02d:%02d) = %v, want %v", tc.h, tc.m, got, tc.want)
		}
	}
	// 14:30 UTC is 19:30 in Tashkent.
	if !inSendWindow(time.Date(2026, 10, 9, 14, 30, 0, 0, time.UTC)) {
		t.Fatal("window must be evaluated in Tashkent time")
	}
}
