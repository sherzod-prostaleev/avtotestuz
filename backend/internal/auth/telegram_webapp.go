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

// initDataClockSkew tolerates a phone clock slightly ahead of ours.
const initDataClockSkew = time.Minute

type WebAppUser struct {
	ID           int64
	FirstName    string
	Username     string
	LanguageCode string
}

// ValidateInitData verifies a Telegram Mini App initData string with the
// algorithm from core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
// and returns the signed user. Nothing outside the signed string is trusted.
func ValidateInitData(raw, botToken string, now time.Time, maxAge time.Duration) (WebAppUser, error) {
	if strings.TrimSpace(botToken) == "" {
		return WebAppUser{}, ErrTelegramBotUnconfigured
	}
	values, err := url.ParseQuery(raw)
	if err != nil || len(values) == 0 {
		return WebAppUser{}, ErrInitDataInvalid
	}
	var gotHash string
	keys := make([]string, 0, len(values))
	for k, vs := range values {
		// A repeated key makes "which value was signed" ambiguous.
		if len(vs) != 1 {
			return WebAppUser{}, ErrInitDataInvalid
		}
		if k == "hash" {
			gotHash = vs[0]
			continue
		}
		keys = append(keys, k)
	}
	if gotHash == "" {
		return WebAppUser{}, ErrInitDataInvalid
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
		return WebAppUser{}, ErrInitDataInvalid
	}

	authUnix, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil {
		return WebAppUser{}, ErrInitDataInvalid
	}
	authAt := time.Unix(authUnix, 0)
	if authAt.After(now.Add(initDataClockSkew)) || now.Sub(authAt) > maxAge {
		return WebAppUser{}, ErrInitDataExpired
	}

	var u struct {
		ID           int64  `json:"id"`
		FirstName    string `json:"first_name"`
		Username     string `json:"username"`
		LanguageCode string `json:"language_code"`
	}
	if err := json.Unmarshal([]byte(values.Get("user")), &u); err != nil || u.ID <= 0 {
		return WebAppUser{}, ErrInitDataInvalid
	}
	return WebAppUser{ID: u.ID, FirstName: u.FirstName, Username: u.Username, LanguageCode: u.LanguageCode}, nil
}
