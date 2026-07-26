package billing

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var (
	// Avoid backticks inside raw strings; match To'ldirish / Toldirish loosely.
	reToldirish = regexp.MustCompile(`(?i)to.?ldirish|\x{043f}\x{043e}\x{043f}\x{043e}\x{043b}\x{043d}`)
	rePlusAmt   = regexp.MustCompile("\u2795\\s*([0-9][0-9\\s.,]*)\\s*UZS")
	reCardLast4 = regexp.MustCompile(`(?i)HUMOCARD\s*\*([0-9]{4})`)
	reTime      = regexp.MustCompile("\U0001F553\\s*(\\d{1,2}):(\\d{2})\\s+(\\d{1,2})\\.(\\d{1,2})\\.(\\d{4})")
	reBalance   = regexp.MustCompile("\U0001F4B0\\s*([0-9][0-9\\s.,]*)\\s*UZS")
)

// ParsedHumoPush is a Humocardbot "To'ldirish" notification.
// AmountUzs comes from the ➕ line (credit), not 💰 (often card balance).
type ParsedHumoPush struct {
	AmountUzs  int64
	PanLast4   string
	TransferAt time.Time
	BalanceUzs int64
	HasBalance bool
}

func ParseHumoCardBotPush(text string) (ParsedHumoPush, bool) {
	text = strings.TrimSpace(text)
	if text == "" || !reToldirish.MatchString(text) {
		return ParsedHumoPush{}, false
	}
	plus := rePlusAmt.FindStringSubmatch(text)
	if len(plus) < 2 {
		return ParsedHumoPush{}, false
	}
	amount, ok := parseUZSAmount(plus[1])
	if !ok || amount <= 0 {
		return ParsedHumoPush{}, false
	}
	card := reCardLast4.FindStringSubmatch(text)
	if len(card) < 2 {
		return ParsedHumoPush{}, false
	}
	tm := reTime.FindStringSubmatch(text)
	if len(tm) < 6 {
		return ParsedHumoPush{}, false
	}
	hour, _ := strconv.Atoi(tm[1])
	min, _ := strconv.Atoi(tm[2])
	day, _ := strconv.Atoi(tm[3])
	month, _ := strconv.Atoi(tm[4])
	year, _ := strconv.Atoi(tm[5])
	// Bank pushes are local UZ time (UTC+5).
	loc := time.FixedZone("UZT", 5*3600)
	transferAt := time.Date(year, time.Month(month), day, hour, min, 0, 0, loc)

	out := ParsedHumoPush{
		AmountUzs:  amount,
		PanLast4:   card[1],
		TransferAt: transferAt,
	}
	if bal := reBalance.FindStringSubmatch(text); len(bal) >= 2 {
		if v, ok := parseUZSAmount(bal[1]); ok {
			out.BalanceUzs = v
			out.HasBalance = true
		}
	}
	return out, true
}

// parseUZSAmount accepts "3.100,00", "3100,00", "3 100.00", "3100".
func parseUZSAmount(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if digits == "" {
		return 0, false
	}
	// If original has decimal comma/dot with exactly 2 fractional digits, drop them.
	normalized := strings.ReplaceAll(s, " ", "")
	normalized = strings.ReplaceAll(normalized, "\u00a0", "")
	if i := strings.LastIndexAny(normalized, ",."); i >= 0 && len(normalized)-i-1 == 2 {
		frac := normalized[i+1:]
		allDig := true
		for _, r := range frac {
			if r < '0' || r > '9' {
				allDig = false
				break
			}
		}
		if allDig && len(digits) >= 2 {
			digits = digits[:len(digits)-2]
		}
	}
	if digits == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
