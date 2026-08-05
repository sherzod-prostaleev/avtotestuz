// Package config loads service configuration from environment variables.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultJWTSecret              = "dev-secret-change-me"
	minJWTSecretLen               = 32
	minClientIPAssertionSecretLen = 32
	minDataEncryptionKeyLen       = 32
)

type Config struct {
	Env               string // dev | staging | prod
	Port              int
	DatabaseURL       string
	RedisURL          string
	DBPoolMaxConns    int32
	DBPoolMinConns    int32
	DBPoolMaxLifetime time.Duration
	DBPoolMaxIdleTime time.Duration
	DBPoolHealthCheck time.Duration
	MediaBaseURL      string // public base for image storage keys

	// PublicBaseURL is the origin users actually browse — the frontend, not
	// this API. Used to build shareable links (referral invite URLs). Without
	// it those were hardcoded to the production domain, so every invite link
	// generated in dev or staging pointed at a host that wasn't being served,
	// making the referral flow impossible to test end-to-end.
	PublicBaseURL string
	CORSOrigins   []string

	// JWTSecret signs learner and admin JWTs. Rotating it logs everyone out —
	// and nothing more, as long as DataEncryptionKey is set (see below).
	JWTSecret string

	// DataEncryptionKey derives the AES-GCM key protecting data at rest:
	// admin TOTP secrets, stored card PANs (manual_pay_card.pan_full,
	// referral_payout.card_number) and the manual-pay Telegram userbot
	// credentials (manual_tg_settings.*_enc).
	//
	// Empty falls back to JWTSecret, which is what every deployment used
	// before the two keys were split — that fallback, not a migration, is what
	// keeps existing ciphertext readable.
	//
	// ROTATING THIS KEY IS DESTRUCTIVE AND IRREVERSIBLE: every value listed
	// above becomes permanently undecryptable, so admins lose TOTP, PANs
	// cannot be revealed and the Telegram session must be recreated. Set it
	// once, to the deployment's CURRENT JWT_SECRET value, and JWT_SECRET is
	// then free to rotate on its own.
	DataEncryptionKey string

	OTPChannel string // off | sandbox | telegram
	// OTPDebugEcho echoes the generated OTP back in the HTTP response of
	// POST /auth/otp/request. It is a deliberate, explicit opt-in (default
	// off) rather than being inferred from ENV: a host left on ENV=dev by
	// accident must never turn every account into a takeover target. Only
	// honoured together with the sandbox sender, and never in staging/prod
	// (see validate).
	OTPDebugEcho            bool
	TelegramGatewayToken    string
	TelegramGatewayURL      string
	ClientIPAssertionSecret string

	// TrustedProxyCIDRs lists networks whose TCP peer address is trusted to
	// have proxied a request unmodified, so its X-Real-IP header can stand
	// in for a signed client-IP assertion (auth.ClientIPResolver.
	// ResolveAsserted's second, trusted-proxy path). Defaults to loopback
	// only: this process binds 127.0.0.1 (see deploy/nginx-drivergo.uz.conf),
	// so a request whose peer is loopback provably came through nginx, which
	// overwrites X-Real-IP with $remote_addr on every location that reaches
	// this service — a client cannot make its own X-Real-IP survive that
	// hop. Parsed once at startup; an unparseable entry fails startup rather
	// than silently dropping out of the trust boundary.
	TrustedProxyCIDRs []*net.IPNet

	// Telegram bot (M4-06) — a different Telegram product from the OTP
	// Gateway above: the Gateway API only delivers verification codes, it
	// cannot receive updates or send arbitrary messages. See
	// docs/superpowers/specs/2026-07-25-m4-06-telegram-bot-design.md §5.2.
	TelegramBotToken      string
	TelegramBotAPIBaseURL string
	TelegramBotUsername   string // no leading '@'
	TelegramBotMode       string // off | webhook | longpoll
	TelegramWebhookSecret string
	// Optional file_id for the group winner sticker. Empty skips the sticker
	// entirely — an unverified file_id would fail on every finished game.
	TelegramQuizWinnerSticker string

	PaymeMerchantID string // cashbox id for the checkout URL
	PaymeEnv        string // test | prod (selects which key)
	PaymeKeyProd    string // production cashbox KEY (Basic-auth password)
	PaymeTestKey    string // sandbox TEST_KEY

	ClickServiceID  string
	ClickMerchantID string
	ClickSecretKey  string

	// OpsAdminToken gates thin ops endpoints (payment provider kill-switches)
	// until the full M3 admin control center ships. Empty disables those routes.
	OpsAdminToken string

	// ManualPayIngestToken authenticates humo-watcher → /internal/manual-pay/*.
	// Empty disables those routes' auth (handlers reject when empty).
	ManualPayIngestToken string

	// Web Push (M4-08 / U-11). Empty public+private disables subscribe/send;
	// same optional pattern as Telegram bot username.
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	VAPIDSubject    string // mailto: or https: contact for push services

	// SentryDSN enables optional sentry-go init (U-41). Empty = disabled no-op.
	// No pager / tracing product — operators set a real DSN when they have one.
	SentryDSN string
}

// PaymeKey returns the Basic-auth password (cashbox KEY) for the current
// PaymeEnv. Empty means Payme is not configured — the webhook rejects all
// calls with -32504.
func (c Config) PaymeKey() string {
	if c.PaymeEnv == "prod" {
		return c.PaymeKeyProd
	}
	return c.PaymeTestKey
}

// PaymeCheckoutHost is the base checkout host (same for test and prod; the
// sandbox tester at test.paycom.uz drives the webhook, not this URL).
func (c Config) PaymeCheckoutHost() string {
	return "https://checkout.paycom.uz"
}

// ResolveDataKey picks the key that at-rest KEKs are derived from:
// DATA_ENCRYPTION_KEY when the deployment sets one, otherwise the JWT secret,
// which is what sealed every ciphertext written before the two were split.
// cmd/encryptpan resolves it the same way — writing PAN ciphertext under a key
// the API would not derive is silent, permanent data loss.
func ResolveDataKey(dataEncryptionKey, jwtSecret string) string {
	if strings.TrimSpace(dataEncryptionKey) != "" {
		return dataEncryptionKey
	}
	return jwtSecret
}

// DataKey is the resolved at-rest encryption key. See DataEncryptionKey for
// why rotating it destroys data and rotating JWTSecret does not.
func (c Config) DataKey() string {
	return ResolveDataKey(c.DataEncryptionKey, c.JWTSecret)
}

func Load() (Config, error) {
	portStr := getenv("PORT", "8080")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid PORT %q: %w", portStr, err)
	}
	poolMax, err := parseInt32Env("DB_POOL_MAX_CONNS", 20)
	if err != nil {
		return Config{}, err
	}
	poolMin, err := parseInt32Env("DB_POOL_MIN_CONNS", 2)
	if err != nil {
		return Config{}, err
	}
	poolLifetime, err := parseDurationEnv("DB_POOL_MAX_LIFETIME", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}
	poolIdle, err := parseDurationEnv("DB_POOL_MAX_IDLE_TIME", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	poolHealth, err := parseDurationEnv("DB_POOL_HEALTH_CHECK_PERIOD", time.Minute)
	if err != nil {
		return Config{}, err
	}
	publicBaseURL := strings.TrimRight(getenv("PUBLIC_BASE_URL", "http://localhost:3000"), "/")
	trustedProxyCIDRs, err := parseCIDRListEnv("TRUSTED_PROXY_CIDRS", "127.0.0.1/32,::1/128")
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Env:               getenv("ENV", "dev"),
		Port:              port,
		DatabaseURL:       getenv("DATABASE_URL", "postgres://avtotest:avtotest@localhost:5432/avtotest?sslmode=disable"),
		RedisURL:          getenv("REDIS_URL", "redis://localhost:6379/0"),
		DBPoolMaxConns:    poolMax,
		DBPoolMinConns:    poolMin,
		DBPoolMaxLifetime: poolLifetime,
		DBPoolMaxIdleTime: poolIdle,
		DBPoolHealthCheck: poolHealth,
		MediaBaseURL:      getenv("MEDIA_BASE_URL", "http://localhost:9000/media"),
		PublicBaseURL:     publicBaseURL,
		CORSOrigins:       splitCSV(getenv("CORS_ALLOWED_ORIGINS", publicBaseURL)),

		JWTSecret:               getenv("JWT_SECRET", defaultJWTSecret),
		DataEncryptionKey:       getenv("DATA_ENCRYPTION_KEY", ""),
		OTPChannel:              getenv("OTP_CHANNEL", "sandbox"),
		OTPDebugEcho:            getenvBool("OTP_DEBUG_ECHO", false),
		TelegramGatewayToken:    getenv("TELEGRAM_GATEWAY_TOKEN", ""),
		TelegramGatewayURL:      getenv("TELEGRAM_GATEWAY_URL", "https://gatewayapi.telegram.org"),
		ClientIPAssertionSecret: getenv("CLIENT_IP_ASSERTION_SECRET", ""),
		TrustedProxyCIDRs:       trustedProxyCIDRs,

		TelegramBotToken:          getenv("TELEGRAM_BOT_TOKEN", ""),
		TelegramBotAPIBaseURL:     getenv("TELEGRAM_BOT_API_BASE_URL", "https://api.telegram.org"),
		TelegramBotUsername:       getenv("TELEGRAM_BOT_USERNAME", ""),
		TelegramBotMode:           getenv("TELEGRAM_BOT_MODE", "off"),
		TelegramWebhookSecret:     getenv("TELEGRAM_WEBHOOK_SECRET", ""),
		TelegramQuizWinnerSticker: getenv("TELEGRAM_QUIZ_WINNER_STICKER", ""),

		PaymeMerchantID: getenv("PAYME_MERCHANT_ID", ""),
		PaymeEnv:        getenv("PAYME_ENV", "test"),
		PaymeKeyProd:    getenv("PAYME_KEY", ""),
		PaymeTestKey:    getenv("PAYME_TEST_KEY", ""),

		ClickServiceID:  getenv("CLICK_SERVICE_ID", ""),
		ClickMerchantID: getenv("CLICK_MERCHANT_ID", ""),
		ClickSecretKey:  getenv("CLICK_SECRET_KEY", ""),

		OpsAdminToken:        getenv("OPS_ADMIN_TOKEN", ""),
		ManualPayIngestToken: getenv("MANUAL_PAY_INGEST_TOKEN", ""),

		VAPIDPublicKey:  getenv("VAPID_PUBLIC_KEY", ""),
		VAPIDPrivateKey: getenv("VAPID_PRIVATE_KEY", ""),
		VAPIDSubject:    getenv("VAPID_SUBJECT", "mailto:ops@avtotest.uz"),
		SentryDSN:       strings.TrimSpace(getenv("SENTRY_DSN", "")),
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	switch c.Env {
	case "dev", "staging", "prod":
	default:
		return fmt.Errorf("invalid ENV %q: must be dev, staging, or prod", c.Env)
	}

	switch c.OTPChannel {
	case "off":
		// Phone+password is the live sign-in path; OTP is not wired into the
		// UI. "off" lets a deployment run ENV=prod (and therefore every
		// startup fail-safe) without procuring an SMS/Telegram gateway.
	case "sandbox":
	case "telegram":
		if strings.TrimSpace(c.TelegramGatewayToken) == "" {
			return fmt.Errorf("OTP_CHANNEL telegram requires TELEGRAM_GATEWAY_TOKEN")
		}
	case "sms":
		return fmt.Errorf("invalid OTP_CHANNEL %q: no sender implementation is configured", c.OTPChannel)
	default:
		return fmt.Errorf("invalid OTP_CHANNEL %q: must be off, sandbox or telegram", c.OTPChannel)
	}

	if c.Env == "staging" || c.Env == "prod" {
		if strings.TrimSpace(c.OpsAdminToken) != "" {
			return fmt.Errorf("OPS_ADMIN_TOKEN is disabled when ENV=%s; use admin RBAC", c.Env)
		}
		if c.OTPChannel == "sandbox" {
			return fmt.Errorf("OTP_CHANNEL sandbox is not allowed when ENV=%s", c.Env)
		}
		// Echoing the OTP in the HTTP response is remote account takeover
		// for any phone number; it must never be reachable off a dev box.
		if c.OTPDebugEcho {
			return fmt.Errorf("OTP_DEBUG_ECHO must not be enabled when ENV=%s", c.Env)
		}
		if c.JWTSecret == defaultJWTSecret {
			return fmt.Errorf("JWT_SECRET must not use the development default when ENV=%s", c.Env)
		}
		if len(strings.TrimSpace(c.JWTSecret)) < minJWTSecretLen {
			return fmt.Errorf("JWT_SECRET must be at least %d bytes when ENV=%s", minJWTSecretLen, c.Env)
		}
		if strings.TrimSpace(c.ClientIPAssertionSecret) == "" {
			return fmt.Errorf("CLIENT_IP_ASSERTION_SECRET is required when ENV=%s", c.Env)
		}
		// Payme/Click redirect here after payment; localhost would strand payers.
		if strings.TrimSpace(c.PublicBaseURL) == "" || strings.HasPrefix(c.PublicBaseURL, "http://localhost") {
			return fmt.Errorf("PUBLIC_BASE_URL must be a real public origin when ENV=%s", c.Env)
		}
	}
	if len(c.CORSOrigins) == 0 {
		return fmt.Errorf("CORS_ALLOWED_ORIGINS must contain at least one origin")
	}
	if c.DBPoolMaxConns < 2 || c.DBPoolMinConns < 0 || c.DBPoolMinConns > c.DBPoolMaxConns {
		return fmt.Errorf("invalid DB pool bounds: min=%d max=%d", c.DBPoolMinConns, c.DBPoolMaxConns)
	}
	if c.DBPoolMaxLifetime <= 0 || c.DBPoolMaxIdleTime <= 0 || c.DBPoolHealthCheck <= 0 {
		return fmt.Errorf("DB pool durations must be positive")
	}
	for _, origin := range c.CORSOrigins {
		if origin == "*" {
			return fmt.Errorf("CORS_ALLOWED_ORIGINS must not contain wildcard origins")
		}
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("invalid CORS origin %q", origin)
		}
		if (c.Env == "staging" || c.Env == "prod") && u.Scheme != "https" {
			return fmt.Errorf("CORS origin %q must use https when ENV=%s", origin, c.Env)
		}
	}
	if secret := strings.TrimSpace(c.ClientIPAssertionSecret); secret != "" && len([]byte(secret)) < minClientIPAssertionSecretLen {
		return fmt.Errorf("CLIENT_IP_ASSERTION_SECRET must be at least %d bytes", minClientIPAssertionSecretLen)
	}
	// Checked at every ENV, not just staging/prod: this key is optional, but a
	// deployment that sets a weak one has silently weakened stored TOTP
	// secrets and PANs, and it can never be strengthened afterwards without
	// losing them.
	if key := strings.TrimSpace(c.DataEncryptionKey); key != "" && len([]byte(key)) < minDataEncryptionKeyLen {
		return fmt.Errorf("DATA_ENCRYPTION_KEY must be at least %d bytes", minDataEncryptionKeyLen)
	}

	switch c.TelegramBotMode {
	case "off":
	case "webhook":
		if strings.TrimSpace(c.TelegramBotToken) == "" {
			return fmt.Errorf("TELEGRAM_BOT_MODE webhook requires TELEGRAM_BOT_TOKEN")
		}
		if strings.TrimSpace(c.TelegramBotUsername) == "" {
			return fmt.Errorf("TELEGRAM_BOT_MODE webhook requires TELEGRAM_BOT_USERNAME")
		}
		if strings.TrimSpace(c.TelegramWebhookSecret) == "" {
			return fmt.Errorf("TELEGRAM_BOT_MODE webhook requires TELEGRAM_WEBHOOK_SECRET")
		}
	case "longpoll":
		if strings.TrimSpace(c.TelegramBotToken) == "" {
			return fmt.Errorf("TELEGRAM_BOT_MODE longpoll requires TELEGRAM_BOT_TOKEN")
		}
		if strings.TrimSpace(c.TelegramBotUsername) == "" {
			return fmt.Errorf("TELEGRAM_BOT_MODE longpoll requires TELEGRAM_BOT_USERNAME")
		}
		// Long-poll is a single-consumer model: Telegram hands updates to
		// whichever getUpdates call is currently open. Running it from more
		// than one prod instance means the others silently starve, and
		// Telegram's own docs warn against mixing it with a webhook on the
		// same bot — so it's dev-only here, not just dev-recommended.
		if c.Env == "prod" {
			return fmt.Errorf("TELEGRAM_BOT_MODE longpoll is not allowed when ENV=prod (use webhook)")
		}
	default:
		return fmt.Errorf("invalid TELEGRAM_BOT_MODE %q: must be off, webhook, or longpoll", c.TelegramBotMode)
	}

	pub := strings.TrimSpace(c.VAPIDPublicKey)
	priv := strings.TrimSpace(c.VAPIDPrivateKey)
	if (pub == "") != (priv == "") {
		return fmt.Errorf("VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY must both be set or both empty")
	}

	return nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, strings.TrimRight(value, "/"))
		}
	}
	return out
}

func parseInt32Env(key string, def int32) (int32, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def, nil
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", key, raw, err)
	}
	return int32(value), nil
}

// parseCIDRListEnv parses a comma-separated list of CIDR networks the same
// way splitCSV parses other list-shaped env vars, but — unlike splitCSV's
// callers, which just collect strings — an entry that fails to parse as a
// CIDR fails startup instead of being silently dropped. Silently dropping a
// typo'd entry here would silently shrink the trust boundary
// TrustedProxyCIDRs exists to draw, which is worse than refusing to start.
func parseCIDRListEnv(key, def string) ([]*net.IPNet, error) {
	entries := splitCSV(getenv(key, def))
	out := make([]*net.IPNet, 0, len(entries))
	for _, entry := range entries {
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid %s entry %q: %w", key, entry, err)
		}
		out = append(out, network)
	}
	return out, nil
}

func parseDurationEnv(key string, def time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", key, raw, err)
	}
	return value, nil
}

// getenvBool reads a boolean env var, accepting the same spellings as the
// rest of the codebase (see admin.TOTPEnforce). Anything unrecognised —
// including a typo — falls back to def, so a malformed value can never
// silently enable a security-relevant switch.
func getenvBool(key string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	default:
		return def
	}
}
