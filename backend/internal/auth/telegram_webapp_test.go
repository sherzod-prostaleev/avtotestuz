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
