package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testBotToken = "123456:TEST-bot-token"

// signInitData builds an initData string exactly as Telegram does, so tests
// exercise the real algorithm instead of a recorded fixture.
func signInitData(t *testing.T, botToken string, fields map[string]string) string {
	t.Helper()
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+"="+fields[k])
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(botToken))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(lines, "\n")))
	v := url.Values{}
	for k, val := range fields {
		v.Set(k, val)
	}
	v.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return v.Encode()
}

func baseFields(now time.Time) map[string]string {
	return map[string]string{
		"auth_date": strconv.FormatInt(now.Unix(), 10),
		"query_id":  "AAHdF6IQAAAAAN0XohDhrOrc",
		"user":      `{"id":279058397,"first_name":"Ali","username":"ali_uz","language_code":"uz"}`,
		"signature": "6fbdaba2c1b6f1d1f2d0a0c4a8c3b0f0",
	}
}

func TestValidateInitDataAcceptsGenuineData(t *testing.T) {
	now := time.Unix(1_760_000_000, 0)
	raw := signInitData(t, testBotToken, baseFields(now))
	u, err := ValidateInitData(raw, testBotToken, now.Add(time.Minute), InitDataMaxAge)
	if err != nil {
		t.Fatalf("valid init data rejected: %v", err)
	}
	if u.ID != 279058397 || u.FirstName != "Ali" || u.Username != "ali_uz" || u.LanguageCode != "uz" {
		t.Fatalf("user = %+v", u)
	}
}

func TestValidateInitDataKnownAnswerVector(t *testing.T) {
	// This is the public Telegram Mini App SDK test vector, verified independently
	// with a separate Python HMAC implementation. It ensures our implementation
	// matches Telegram's spec exactly.
	botToken := "5768337691:AAH5YkoiEuPk8-FZa32hStHTqXiLPtAEhx8"
	initData := "query_id=AAHdF6IQAAAAAN0XohDhrOrc&user=%7B%22id%22%3A279058397%2C%22first_name%22%3A%22Vladislav%22%2C%22last_name%22%3A%22Kibenko%22%2C%22username%22%3A%22vdkfrost%22%2C%22language_code%22%3A%22ru%22%2C%22is_premium%22%3Atrue%7D&auth_date=1662771648&hash=c501b71e775f74ce10e377dea85a7ea24ecd640b223ea86dfe453e0eaed2e2b2"
	authAt := time.Unix(1662771648, 0)
	now := authAt.Add(time.Minute)

	u, err := ValidateInitData(initData, botToken, now, InitDataMaxAge)
	if err != nil {
		t.Fatalf("valid init data rejected: %v", err)
	}
	if u.ID != 279058397 || u.FirstName != "Vladislav" || u.Username != "vdkfrost" || u.LanguageCode != "ru" {
		t.Fatalf("user = %+v, want ID=279058397 FirstName=Vladislav Username=vdkfrost LanguageCode=ru", u)
	}

	// Same string with one hash hex digit changed must be rejected
	tamperedHash := strings.Replace(initData, "c501b71e", "c501b71f", 1)
	_, err = ValidateInitData(tamperedHash, botToken, now, InitDataMaxAge)
	if !errors.Is(err, ErrInitDataInvalid) {
		t.Fatalf("tampered hash accepted, err = %v", err)
	}
}

func TestValidateInitDataRejects(t *testing.T) {
	now := time.Unix(1_760_000_000, 0)
	genuine := signInitData(t, testBotToken, baseFields(now))

	tampered := strings.Replace(genuine, "279058397", "279058398", 1)
	noHash := func() string { v, _ := url.ParseQuery(genuine); v.Del("hash"); return v.Encode() }()
	dupKey := genuine + "&auth_date=1"
	expiredFields := baseFields(now.Add(-25 * time.Hour))
	futureFields := baseFields(now.Add(5 * time.Minute))
	badUser := baseFields(now)
	badUser["user"] = `{"id":0}`
	badUserNegative := baseFields(now)
	badUserNegative["user"] = `{"id":-1}`
	noUserField := baseFields(now)
	delete(noUserField, "user")
	noAuthDate := baseFields(now)
	delete(noAuthDate, "auth_date")
	badAuthDate := baseFields(now)
	badAuthDate["auth_date"] = "not-a-number"
	// The hash is replaced AFTER signing: setting it in the field map would
	// only sign a "hash" field and then overwrite it with a valid one.
	badHash := func() string {
		v, _ := url.ParseQuery(genuine)
		v.Set("hash", "zz"+v.Get("hash")[2:])
		return v.Encode()
	}()
	noSigInCheck := func() string {
		// A hash computed WITHOUT the signature field must not validate:
		// Telegram includes every field but hash in data_check_string.
		f := baseFields(now)
		sig := f["signature"]
		delete(f, "signature")
		v, _ := url.ParseQuery(signInitData(t, testBotToken, f))
		v.Set("signature", sig)
		return v.Encode()
	}()

	cases := []struct {
		name string
		raw  string
		tok  string
		want error
	}{
		{"tampered user", tampered, testBotToken, ErrInitDataInvalid},
		{"wrong token", genuine, "999:other", ErrInitDataInvalid},
		{"missing hash", noHash, testBotToken, ErrInitDataInvalid},
		{"duplicate key", dupKey, testBotToken, ErrInitDataInvalid},
		{"empty", "", testBotToken, ErrInitDataInvalid},
		{"expired", signInitData(t, testBotToken, expiredFields), testBotToken, ErrInitDataExpired},
		{"future dated", signInitData(t, testBotToken, futureFields), testBotToken, ErrInitDataExpired},
		{"bad user", signInitData(t, testBotToken, badUser), testBotToken, ErrInitDataInvalid},
		{"negative user id", signInitData(t, testBotToken, badUserNegative), testBotToken, ErrInitDataInvalid},
		{"missing user field", signInitData(t, testBotToken, noUserField), testBotToken, ErrInitDataInvalid},
		{"missing auth_date", signInitData(t, testBotToken, noAuthDate), testBotToken, ErrInitDataInvalid},
		{"non-numeric auth_date", signInitData(t, testBotToken, badAuthDate), testBotToken, ErrInitDataInvalid},
		{"non-hex hash", badHash, testBotToken, ErrInitDataInvalid},
		{"signature excluded from check", noSigInCheck, testBotToken, ErrInitDataInvalid},
		{"no bot token", genuine, "", ErrTelegramBotUnconfigured},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateInitData(tc.raw, tc.tok, now, InitDataMaxAge)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestValidateInitDataBoundaries(t *testing.T) {
	baseNow := time.Unix(1_760_000_000, 0)

	cases := []struct {
		name        string
		signedAt    time.Time
		validatedAt time.Time
		wantErr     error
	}{
		// Exactly maxAge old: should be accepted
		{"exactly maxAge old", baseNow, baseNow.Add(InitDataMaxAge), nil},
		// maxAge + 1 second: should be expired
		{"maxAge + 1 second", baseNow, baseNow.Add(InitDataMaxAge).Add(time.Second), ErrInitDataExpired},
		// Exactly skew in future: should be accepted
		{"exactly skew in future", baseNow.Add(initDataClockSkew), baseNow, nil},
		// Skew + 1 second in future: should be expired
		{"skew + 1 second in future", baseNow.Add(initDataClockSkew).Add(time.Second), baseNow, ErrInitDataExpired},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fields := baseFields(tc.signedAt)
			raw := signInitData(t, testBotToken, fields)
			_, err := ValidateInitData(raw, testBotToken, tc.validatedAt, InitDataMaxAge)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateInitDataAcceptsUnknownFields(t *testing.T) {
	now := time.Unix(1_760_000_000, 0)
	fields := baseFields(now)
	fields["chat_type"] = "sender" // Unknown extra field
	raw := signInitData(t, testBotToken, fields)
	u, err := ValidateInitData(raw, testBotToken, now.Add(time.Minute), InitDataMaxAge)
	if err != nil {
		t.Fatalf("valid init data with unknown field rejected: %v", err)
	}
	if u.ID != 279058397 {
		t.Fatalf("user ID = %d, want 279058397", u.ID)
	}
}

func TestValidateInitDataRejectsOversizedInput(t *testing.T) {
	now := time.Unix(1_760_000_000, 0)
	// Padding is signed along with everything else, so only the size bound can reject it.
	fields := baseFields(now)
	fields["padding"] = strings.Repeat("a", InitDataMaxBytes)
	raw := signInitData(t, testBotToken, fields)
	if _, err := ValidateInitData(raw, testBotToken, now, InitDataMaxAge); !errors.Is(err, ErrInitDataInvalid) {
		t.Fatalf("err = %v, want ErrInitDataInvalid", err)
	}
}

// signContact builds a requestContact response the way Telegram signs it:
// contact JSON + auth_date, same WebAppData HMAC as initData.
func signContact(t *testing.T, botToken string, tgID int64, phone string, at time.Time) string {
	t.Helper()
	return signInitData(t, botToken, map[string]string{
		"contact":   `{"user_id":` + strconv.FormatInt(tgID, 10) + `,"phone_number":"` + phone + `","first_name":"Ali"}`,
		"auth_date": strconv.FormatInt(at.Unix(), 10),
	})
}

func TestValidateContactKnownAnswerVector(t *testing.T) {
	// Computed independently with Python's hmac/hashlib (WebAppData secret,
	// sorted "key=value" lines) over the public SDK test bot token, in the
	// format telegram-web-app.js's getRequestedContact returns.
	botToken := "5768337691:AAH5YkoiEuPk8-FZa32hStHTqXiLPtAEhx8"
	raw := "contact=%7B%22user_id%22%3A279058397%2C%22phone_number%22%3A%22998901234567%22%2C%22first_name%22%3A%22Vladislav%22%2C%22last_name%22%3A%22Kibenko%22%7D&auth_date=1662771700&hash=2e24c6752581361f420fdb406a8618397a9e83dc11ebc0032850d56e0cbda7ce"
	now := time.Unix(1662771700, 0).Add(time.Minute)
	c, err := ValidateContact(raw, botToken, now, InitDataLinkMaxAge)
	if err != nil {
		t.Fatalf("valid contact rejected: %v", err)
	}
	if c.UserID != 279058397 || c.Phone != "998901234567" {
		t.Fatalf("contact = %+v", c)
	}
	tampered := strings.Replace(raw, "998901234567", "998901234568", 1)
	if _, err := ValidateContact(tampered, botToken, now, InitDataLinkMaxAge); !errors.Is(err, ErrInitDataInvalid) {
		t.Fatalf("tampered phone accepted, err = %v", err)
	}
}

func TestValidateContactRejects(t *testing.T) {
	now := time.Unix(1_760_000_000, 0)
	genuine := signContact(t, testBotToken, 5001, "998901234567", now)
	initDataNotContact := signInitData(t, testBotToken, baseFields(now))
	noPhone := signInitData(t, testBotToken, map[string]string{
		"contact": `{"user_id":5001}`, "auth_date": strconv.FormatInt(now.Unix(), 10),
	})
	zeroUser := signInitData(t, testBotToken, map[string]string{
		"contact": `{"user_id":0,"phone_number":"998901234567"}`, "auth_date": strconv.FormatInt(now.Unix(), 10),
	})
	cases := []struct {
		name string
		raw  string
		tok  string
		at   time.Time
		want error
	}{
		{"wrong token", genuine, "999:other", now, ErrInitDataInvalid},
		{"expired", genuine, testBotToken, now.Add(InitDataLinkMaxAge + time.Second), ErrInitDataExpired},
		{"init data is not a contact", initDataNotContact, testBotToken, now, ErrInitDataInvalid},
		{"no phone", noPhone, testBotToken, now, ErrInitDataInvalid},
		{"zero user", zeroUser, testBotToken, now, ErrInitDataInvalid},
		{"empty", "", testBotToken, now, ErrInitDataInvalid},
		{"no bot token", genuine, "", now, ErrTelegramBotUnconfigured},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateContact(tc.raw, tc.tok, tc.at, InitDataLinkMaxAge); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNormalizeTelegramContactPhone(t *testing.T) {
	for in, want := range map[string]string{
		"998901234567":  "+998901234567",
		"+998901234567": "+998901234567",
		// Address-book contact cards carry the number as typed.
		"+998 90 123 45 67":   "+998901234567",
		"+998 (90) 123-45-67": "+998901234567",
	} {
		got, err := NormalizeTelegramContactPhone(in)
		if err != nil || got != want {
			t.Fatalf("%q -> %q, %v", in, got, err)
		}
	}
	// A 9-digit foreign number must never be read as a national UZ number.
	for _, in := range []string{"959012345", "901234567", "79001234567", "+7 900 123-45-67", "99890123456a", "", "abc", "+7 912 345 67 89", "+998 90 123 45"} {
		if _, err := NormalizeTelegramContactPhone(in); err == nil {
			t.Fatalf("%q accepted", in)
		}
	}
}
