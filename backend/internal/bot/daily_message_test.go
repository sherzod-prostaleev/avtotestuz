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

func pitchedOn(daysAgo int) pgtype.Date {
	return pgtype.Date{Time: dailyToday.AddDate(0, 0, -daysAgo), Valid: true}
}

func TestPickPersonalLinePriority(t *testing.T) {
	type row = sqlc.ListTelegramReminderAudienceRow
	cases := []struct {
		name string
		row  row
		want personalLine
	}{
		{"unlinked, never pitched, gets the signup pitch", row{}, lineSignup},
		{"unlinked, pitched 7 days ago, gets it again", row{LastSignupPitchOn: pitchedOn(7)}, lineSignup},
		{"unlinked, pitched 6 days ago, gets a neutral line", row{LastSignupPitchOn: pitchedOn(6)}, lineUnlinked},
		{"unlinked, pitched today, gets a neutral line", row{LastSignupPitchOn: pitchedOn(0)}, lineUnlinked},
		{"streak beats due",
			row{Linked: true, PhoneVerified: true, StreakCurrent: 5, LastActiveDate: activeOn(1), DueCount: 7}, lineStreak},
		{"streak counts when active today too",
			row{Linked: true, PhoneVerified: true, StreakCurrent: 2, LastActiveDate: activeOn(0)}, lineStreak},
		{"an unverified link gets no personal numbers at all",
			row{Linked: true, StreakCurrent: 5, LastActiveDate: activeOn(1), DueCount: 3}, lineGeneric},
		{"an unverified idle link is not told it was missed",
			row{Linked: true, LastActiveDate: activeOn(10)}, lineGeneric},
		{"a one-day streak is not worth a line",
			row{Linked: true, PhoneVerified: true, StreakCurrent: 1, LastActiveDate: activeOn(0)}, lineGeneric},
		{"a broken streak is not a streak",
			row{Linked: true, PhoneVerified: true, StreakCurrent: 9, LastActiveDate: activeOn(2)}, lineGeneric},
		{"due beats inactive",
			row{Linked: true, PhoneVerified: true, DueCount: 4, LastActiveDate: activeOn(10)}, lineDue},
		{"inactive three days",
			row{Linked: true, PhoneVerified: true, LastActiveDate: activeOn(3)}, lineInactive},
		{"never active and nothing solved: welcome, not «we missed you»",
			row{Linked: true, PhoneVerified: true}, lineWelcome},
		{"never active streak-wise but has solved tickets: comeback with the real count",
			row{Linked: true, PhoneVerified: true, TicketsCompleted: 2}, lineInactive},
		{"two days off is still generic",
			row{Linked: true, PhoneVerified: true, LastActiveDate: activeOn(2)}, lineGeneric},
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
	if got := personalLineText(lineStreak, langUz, streak, dailyToday); got != "🔥 5 kunlik seriyangiz bor — bugun uzib qo'ymang!" {
		t.Fatalf("uz streak = %q", got)
	}
	if got := personalLineText(lineStreak, langRu, streak, dailyToday); !strings.Contains(got, "5 дней") {
		t.Fatalf("ru streak = %q, want plural «5 дней»", got)
	}
	due := sqlc.ListTelegramReminderAudienceRow{DueCount: 3}
	if got := personalLineText(lineDue, langUz, due, dailyToday); got != "📚 Bugun 3 ta savol takrorlash navbatida" {
		t.Fatalf("uz due = %q", got)
	}
	if got := personalLineText(lineDue, langRu, sqlc.ListTelegramReminderAudienceRow{DueCount: 21}, dailyToday); !strings.HasSuffix(got, " 21 вопрос") {
		t.Fatalf("ru due = %q, want «21 вопрос»", got)
	}
	none := sqlc.ListTelegramReminderAudienceRow{}
	if got := personalLineText(lineSignup, langUz, none, dailyToday); got != "🎁 Hisobingiz yo'qmi? Ro'yxatdan o'ting — 24 soat VIP bepul. Hisobingiz bo'lsa, ilovada kiring va raqamingizni tasdiqlang." {
		t.Fatalf("uz signup = %q", got)
	}
	if got := personalLineText(lineSignup, langRu, none, dailyToday); got != "🎁 Ещё нет аккаунта? Зарегистрируйтесь — 24 часа VIP бесплатно. Если аккаунт есть — войдите в приложении и подтвердите номер." {
		t.Fatalf("ru signup = %q", got)
	}
	for _, l := range []lang{langUz, langRu} {
		w := personalLineText(lineWelcome, l, none, dailyToday)
		if !strings.HasPrefix(w, "👋") || strings.Contains(w, "Ancha bo'ldi") || strings.Contains(w, "Давно") {
			t.Fatalf("welcome (%v) = %q", l, w)
		}
	}
}

// The neutral line rotates by date only: everyone gets the same one on a
// given day, consecutive days differ, it never quotes a number and it is
// the same line for an unlinked user and a linked one with nothing to say.
func TestMotivationalLineRotatesByDate(t *testing.T) {
	for _, l := range []lang{langUz, langRu} {
		seen := map[string]bool{}
		prev := ""
		n := len(motivationalLines(l))
		if n < 5 || n > 7 {
			t.Fatalf("%v: %d variants, want 5–7", l, n)
		}
		for d := 0; d < n; d++ {
			day := dailyToday.AddDate(0, 0, d)
			got := personalLineText(lineGeneric, l, sqlc.ListTelegramReminderAudienceRow{}, day)
			if got != personalLineText(lineUnlinked, l, sqlc.ListTelegramReminderAudienceRow{}, day) {
				t.Fatal("unlinked and generic must share the day's neutral line")
			}
			if got != personalLineText(lineGeneric, l, sqlc.ListTelegramReminderAudienceRow{Linked: true}, day) {
				t.Fatal("the neutral line must not depend on the user")
			}
			if got == prev {
				t.Fatalf("%v: day %d repeats the previous day's line %q", l, d, got)
			}
			for _, r := range got {
				if r >= '0' && r <= '9' {
					t.Fatalf("%v: neutral line quotes a number: %q", l, got)
				}
			}
			seen[got], prev = true, got
		}
		if len(seen) != n {
			t.Fatalf("%v: %d distinct lines over %d days", l, len(seen), n)
		}
	}
}

// The comeback line quotes real progress or none at all.
func TestInactiveLineNeverInventsNumbers(t *testing.T) {
	with := personalLineText(lineInactive, langUz, sqlc.ListTelegramReminderAudienceRow{TicketsCompleted: 7}, dailyToday)
	if !strings.Contains(with, "7 ta bilet") {
		t.Fatalf("uz inactive with tickets = %q", with)
	}
	without := personalLineText(lineInactive, langUz, sqlc.ListTelegramReminderAudienceRow{}, dailyToday)
	for _, r := range without {
		if r >= '0' && r <= '9' {
			t.Fatalf("uz inactive without progress quotes a number: %q", without)
		}
	}
	ru := personalLineText(lineInactive, langRu, sqlc.ListTelegramReminderAudienceRow{TicketsCompleted: 2}, dailyToday)
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
