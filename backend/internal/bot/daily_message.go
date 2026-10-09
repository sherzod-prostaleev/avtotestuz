package bot

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"avtotest.uz/backend/internal/auth"
	"avtotest.uz/backend/internal/db/sqlc"
)

// The daily «Kun savoli» bundle: copy, the personal line and its buttons.
// Everything here is pure so the wording and priority rules are tested
// without a database or Telegram.

// cbReminderOff is the «🔕 Eslatmalarni o'chirish» button's callback data.
const cbReminderOff = "rem:off"

// personalLine is which one-line nudge a recipient gets under the poll.
type personalLine int

const (
	lineGeneric personalLine = iota
	lineStreak
	lineDue
	lineInactive
	lineSignup
)

// segment is the dry-run / log name of a line.
func (p personalLine) segment() string {
	switch p {
	case lineStreak:
		return "streak"
	case lineDue:
		return "due"
	case lineInactive:
		return "inactive"
	case lineSignup:
		return "unlinked"
	default:
		return "generic"
	}
}

const (
	minStreakForLine = 2
	inactiveAfter    = 3 // days without activity before the comeback line
)

// pickPersonalLine chooses exactly one line: streak > due > inactive >
// generic for linked learners, the signup pitch for everyone else. today
// is a UTC calendar day, the convention streak.last_active_date uses.
//
// The streak line needs a phone-verified link: an old deep-link token could
// bind someone else's Telegram to a profile (see migration 0076), and a
// streak is the most personal number here.
func pickPersonalLine(a sqlc.ListTelegramReminderAudienceRow, today time.Time) personalLine {
	if !a.Linked {
		return lineSignup
	}
	daysIdle := -1 // never active
	if a.LastActiveDate.Valid {
		daysIdle = int(today.Sub(a.LastActiveDate.Time.UTC().Truncate(24*time.Hour)) / (24 * time.Hour))
	}
	streakAlive := daysIdle == 0 || daysIdle == 1
	switch {
	case a.PhoneVerified && streakAlive && a.StreakCurrent >= minStreakForLine:
		return lineStreak
	case a.DueCount > 0:
		return lineDue
	case daysIdle < 0 || daysIdle >= inactiveAfter:
		return lineInactive
	default:
		return lineGeneric
	}
}

// personalLineText renders a line. Numbers only ever come from the row; the
// comeback line without completed tickets quotes none.
func personalLineText(p personalLine, l lang, a sqlc.ListTelegramReminderAudienceRow) string {
	ru := l == langRu
	switch p {
	case lineStreak:
		n := int(a.StreakCurrent)
		if ru {
			return fmt.Sprintf("🔥 Ваша серия — %d %s подряд. Не прерывайте её сегодня!", n, ruPlural(n, "день", "дня", "дней"))
		}
		return fmt.Sprintf("🔥 %d kunlik seriyangiz bor — bugun uzib qo'ymang!", n)
	case lineDue:
		n := int(a.DueCount)
		if ru {
			return fmt.Sprintf("📚 Сегодня на повторение %d %s", n, ruPlural(n, "вопрос", "вопроса", "вопросов"))
		}
		return fmt.Sprintf("📚 Bugun %d ta savolni takrorlash navbati", n)
	case lineInactive:
		n := int(a.TicketsCompleted)
		switch {
		case n > 0 && ru:
			return fmt.Sprintf("🚗 Вы уже решили %d %s — продолжим сегодня?", n, ruPlural(n, "билет", "билета", "билетов"))
		case n > 0:
			return fmt.Sprintf("🚗 Siz allaqachon %d ta bilet yechgansiz — bugun davom ettiramizmi?", n)
		case ru:
			return "🚗 Давно не виделись — решите сегодня хотя бы один билет."
		default:
			return "🚗 Ancha bo'ldi ko'rinmadingiz — bugun bitta bilet yechib ko'ring."
		}
	case lineSignup:
		if h := int(auth.SignupTrialDuration / time.Hour); h > 0 {
			if ru {
				return fmt.Sprintf("🎁 Зарегистрируйтесь — %d %s VIP бесплатно", h, ruPlural(h, "час", "часа", "часов"))
			}
			return fmt.Sprintf("🎁 Ro'yxatdan o'ting — %d soat bepul VIP", h)
		}
		// No trial to promise: the generic line, never a stale offer.
	}
	if ru {
		return "🎯 По билету в день — и на экзамен с уверенностью."
	}
	return "🎯 Kuniga bitta bilet — imtihonga ishonch bilan borasiz."
}

// ruPlural picks the Russian noun form for n (1 день, 2 дня, 5 дней).
func ruPlural(n int, one, few, many string) string {
	if n < 0 {
		n = -n
	}
	switch n100 := n % 100; {
	case n100 >= 11 && n100 <= 14:
		return many
	case n%10 == 1:
		return one
	case n%10 >= 2 && n%10 <= 4:
		return few
	default:
		return many
	}
}

// dailyCaption heads the photo when the question has an image (the poll
// then replies to it); dailyFollowUp is the text sent after a text-only
// poll. Either way the bundle is two messages.
func dailyCaption(l lang, line string) string {
	if l == langRu {
		return "🧠 Вопрос дня\n\n" + line
	}
	return "🧠 Kun savoli\n\n" + line
}

func dailyFollowUp(l lang, line string) string {
	if l == langRu {
		return "🧠 Это «Вопрос дня» ☝️\n\n" + line
	}
	return "🧠 Bu — bugungi «Kun savoli» ☝️\n\n" + line
}

func practiceButton(webAppURL, publicBaseURL string, l lang) InlineKeyboardButton {
	text := "📝 Bugungi mashq"
	if l == langRu {
		text = "📝 Практика на сегодня"
	}
	link, webApp := sectionURL(webAppURL, publicBaseURL, l, "practice")
	if webApp {
		return InlineKeyboardButton{Text: text, WebApp: &WebAppInfo{URL: link}}
	}
	return InlineKeyboardButton{Text: text, URL: link}
}

// dailyKeyboard is the bundle's buttons: today's practice (Mini App, or the
// website when TELEGRAM_WEBAPP_URL is cleared) and the opt-out.
func dailyKeyboard(webAppURL, publicBaseURL string, l lang) *InlineKeyboardMarkup {
	off := "🔕 Eslatmalarni o'chirish"
	if l == langRu {
		off = "🔕 Отключить напоминания"
	}
	return &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		{practiceButton(webAppURL, publicBaseURL, l)},
		{{Text: off, CallbackData: cbReminderOff}},
	}}
}

// Replies for the opt-out button and /eslatma.
func remindersOffText(l lang) string {
	if l == langRu {
		return "🔕 Ежедневные напоминания отключены. Чтобы включить снова, отправьте /eslatma."
	}
	return "🔕 Kunlik eslatmalar o'chirildi. Qayta yoqish uchun /eslatma yozing."
}

func remindersOnText(l lang) string {
	if l == langRu {
		return "🔔 Напоминания включены: каждый день в 19:00 придёт «Вопрос дня». Отключить: /eslatma"
	}
	return "🔔 Eslatmalar yoqildi: har kuni soat 19:00 da «Kun savoli» keladi. O'chirish: /eslatma"
}

func remindersGroupText(l lang) string {
	if l == langRu {
		return "Напоминания настраиваются только в личном чате с ботом."
	}
	return "Eslatmalar faqat bot bilan shaxsiy chatda sozlanadi."
}

// explanationForPoll joins an explanation's blocks into the poll's
// explanation, or returns "" when it does not fit Telegram's 200 chars:
// a legal explanation cut mid-sentence says something it does not.
func explanationForPoll(blocks []byte) string {
	var parsed []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(blocks, &parsed); err != nil {
		return ""
	}
	parts := make([]string, 0, len(parsed))
	for _, b := range parsed {
		if t := strings.TrimSpace(b.Text); t != "" {
			parts = append(parts, t)
		}
	}
	text := strings.Join(parts, " ")
	if utf8.RuneCountInString(text) > pollExplanationMax {
		return ""
	}
	return text
}

// tashkent is fixed UTC+5: Uzbekistan has no DST, and the distroless image
// ships no tzdata for time.LoadLocation.
var tashkent = time.FixedZone("Asia/Tashkent", 5*60*60)

const (
	sendFromHour  = 19 // the reminder goes out from 19:00 Tashkent…
	sendUntilHour = 21 // …and never at or after 21:00 (quiet hours until 09:00)
	quietEndHour  = 9
)

// inSendWindow reports whether t is inside 19:00–21:00 Tashkent. The quiet
// hours check is redundant with today's window but stays as its own guard,
// so moving the send hour can never move a send into the night.
func inSendWindow(t time.Time) bool {
	h := t.In(tashkent).Hour()
	if h >= sendUntilHour || h < quietEndHour {
		return false
	}
	return h >= sendFromHour
}

// dayOfTime is t's Tashkent calendar day, as the UTC midnight a DATE scans to.
func dayOfTime(t time.Time) time.Time {
	y, m, d := t.In(tashkent).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
