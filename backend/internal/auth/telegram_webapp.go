package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInitDataInvalid = errors.New("telegram init data invalid")
	ErrInitDataExpired = errors.New("telegram init data expired")
)

// InitDataMaxAge bounds how long a Mini App launch payload can be exchanged
// for a session. Requests after sign-in ride the cookies, so this only limits
// how stale a re-auth (session expired while the Mini App stayed open) may be.
const InitDataMaxAge = 24 * time.Hour

// InitDataLinkMaxAge is the stricter bound for creating or moving a Telegram
// link through login/register. A leaked launch payload must not let someone
// re-point a victim's Telegram account at their own profile for a whole day;
// legitimate users link within minutes of opening the Mini App.
const InitDataLinkMaxAge = time.Hour

// InitDataMaxBytes bounds the launch payload we will HMAC and parse; real
// payloads are well under 1 KiB, so a larger one is abuse, not a user.
const InitDataMaxBytes = 4096

// initDataClockSkew tolerates a phone clock slightly ahead of ours.
const initDataClockSkew = time.Minute

type WebAppUser struct {
	ID           int64
	FirstName    string
	LastName     string
	Username     string
	LanguageCode string
	// StartParam is the signed start_param of a t.me/<bot>?startapp=<param>
	// launch ("" otherwise); ref_<CODE> carries a referral.
	StartParam string
	// AllowsWriteToPM: the user allowed the bot to message them (signed,
	// part of the user object Telegram puts in initData).
	AllowsWriteToPM bool
}

// verifyWebAppSignature checks a Telegram-signed query string (Mini App
// initData, or the requestContact response, which Telegram signs the same
// way) with the algorithm from
// core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app,
// then bounds its auth_date by maxAge. It returns the parsed fields; nothing
// outside the signed string is trusted.
func verifyWebAppSignature(raw, botToken string, now time.Time, maxAge time.Duration) (url.Values, error) {
	if strings.TrimSpace(botToken) == "" {
		return nil, ErrTelegramBotUnconfigured
	}
	if len(raw) > InitDataMaxBytes {
		return nil, ErrInitDataInvalid
	}
	values, err := url.ParseQuery(raw)
	if err != nil || len(values) == 0 {
		return nil, ErrInitDataInvalid
	}
	var gotHash string
	keys := make([]string, 0, len(values))
	for k, vs := range values {
		// A repeated key makes "which value was signed" ambiguous.
		if len(vs) != 1 {
			return nil, ErrInitDataInvalid
		}
		if k == "hash" {
			gotHash = vs[0]
			continue
		}
		keys = append(keys, k)
	}
	if gotHash == "" {
		return nil, ErrInitDataInvalid
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+values.Get(k))
	}

	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(botToken))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(pairs, "\n")))
	want := mac.Sum(nil)
	got, err := hex.DecodeString(gotHash)
	if err != nil || !hmac.Equal(got, want) {
		return nil, ErrInitDataInvalid
	}

	authUnix, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil {
		return nil, ErrInitDataInvalid
	}
	authAt := time.Unix(authUnix, 0)
	if authAt.After(now.Add(initDataClockSkew)) || now.Sub(authAt) > maxAge {
		return nil, ErrInitDataExpired
	}
	return values, nil
}

// ValidateInitData verifies a Telegram Mini App initData string and returns
// the signed user.
func ValidateInitData(raw, botToken string, now time.Time, maxAge time.Duration) (WebAppUser, error) {
	values, err := verifyWebAppSignature(raw, botToken, now, maxAge)
	if err != nil {
		return WebAppUser{}, err
	}
	var u struct {
		ID           int64  `json:"id"`
		FirstName    string `json:"first_name"`
		LastName     string `json:"last_name"`
		Username     string `json:"username"`
		LanguageCode string `json:"language_code"`
		AllowsWrite  bool   `json:"allows_write_to_pm"`
	}
	if err := json.Unmarshal([]byte(values.Get("user")), &u); err != nil || u.ID <= 0 {
		return WebAppUser{}, ErrInitDataInvalid
	}
	return WebAppUser{ID: u.ID, FirstName: u.FirstName, LastName: u.LastName, Username: u.Username,
		LanguageCode: u.LanguageCode, StartParam: values.Get("start_param"), AllowsWriteToPM: u.AllowsWrite}, nil
}

// WebAppContact is the phone number a Telegram user shared with the bot
// through WebApp.requestContact, as Telegram signed it.
type WebAppContact struct {
	UserID int64
	Phone  string
}

// ValidateContact verifies the `response` string WebApp.requestContact hands
// the Mini App: a query string `contact=<json>&auth_date=<unix>&hash=<hex>`
// that Telegram's server signs exactly like initData (same WebAppData HMAC).
// The format is not in Telegram's prose docs; it is what telegram-web-app.js
// returns from its getRequestedContact custom method (see the final-fix
// report for the sources). Because Telegram, not the client, vouches for the
// phone, this is the proof that a Telegram account owns a phone number.
func ValidateContact(raw, botToken string, now time.Time, maxAge time.Duration) (WebAppContact, error) {
	values, err := verifyWebAppSignature(raw, botToken, now, maxAge)
	if err != nil {
		return WebAppContact{}, err
	}
	var c struct {
		UserID      int64  `json:"user_id"`
		PhoneNumber string `json:"phone_number"`
	}
	if err := json.Unmarshal([]byte(values.Get("contact")), &c); err != nil || c.UserID <= 0 || c.PhoneNumber == "" {
		return WebAppContact{}, ErrInitDataInvalid
	}
	return WebAppContact{UserID: c.UserID, Phone: c.PhoneNumber}, nil
}

// NormalizeTelegramContactPhone turns a phone number Telegram reports for an
// account into our +998XXXXXXXXX form. Telegram's own shares are bare digits,
// but contact cards from an address book carry the number as typed
// ("+998 (90) 123-45-67"), so every non-digit is dropped first. It is stricter
// than NormalizePhone on purpose: that one also accepts a bare 9-digit
// national number, and a 9-digit foreign number (e.g. an old Myanmar +95
// 9xxxxxx) would then be read as a UZ phone and could "prove" ownership of
// someone else's profile. Here exactly 12 digits starting with 998 are required.
func NormalizeTelegramContactPhone(raw string) (string, error) {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	d := b.String()
	if len(d) != 12 || !strings.HasPrefix(d, "998") {
		return "", ErrInvalidPhone
	}
	return "+" + d, nil
}
