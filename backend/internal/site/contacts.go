package site

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const contactsKey = "contacts"

const maxFieldLen = 500

// Contacts is the public footer shape. Empty strings mean "use i18n fallback".
type Contacts struct {
	Phone        string `json:"phone"`
	PhoneTel     string `json:"phoneTel"`
	Email        string `json:"email"`
	Address      string `json:"address"`
	Hours        string `json:"hours"`
	Telegram     string `json:"telegram"`
	TelegramURL  string `json:"telegramUrl"`
	Instagram    string `json:"instagram"`
	InstagramURL string `json:"instagramUrl"`
}

// Store reads/writes site_settings rows.
type Store struct {
	Pool *pgxpool.Pool
}

// GetContacts returns stored contacts or empty fields when missing.
func (s Store) GetContacts(ctx context.Context) (Contacts, error) {
	if s.Pool == nil {
		return Contacts{}, nil
	}
	var raw []byte
	err := s.Pool.QueryRow(ctx, `
		SELECT value_json FROM site_settings WHERE key = $1`, contactsKey).Scan(&raw)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Contacts{}, nil
		}
		return Contacts{}, fmt.Errorf("get site contacts: %w", err)
	}
	var out Contacts
	if len(raw) == 0 || string(raw) == "null" {
		return Contacts{}, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return Contacts{}, fmt.Errorf("decode site contacts: %w", err)
	}
	return normalizeContacts(out), nil
}

// PutContacts replaces the contacts document. updatedBy is an ops actor label.
func (s Store) PutContacts(ctx context.Context, in Contacts, updatedBy string) (Contacts, error) {
	if s.Pool == nil {
		return Contacts{}, fmt.Errorf("site: PutContacts requires Pool")
	}
	out := normalizeContacts(in)
	if err := validateContacts(out); err != nil {
		return Contacts{}, err
	}
	if updatedBy == "" {
		updatedBy = "ops"
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return Contacts{}, fmt.Errorf("encode site contacts: %w", err)
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO site_settings (key, value_json, updated_at, updated_by)
		VALUES ($1, $2::jsonb, $3, $4)
		ON CONFLICT (key) DO UPDATE
		  SET value_json = EXCLUDED.value_json,
		      updated_at = EXCLUDED.updated_at,
		      updated_by = EXCLUDED.updated_by`,
		contactsKey, raw, time.Now().UTC(), updatedBy,
	)
	if err != nil {
		return Contacts{}, fmt.Errorf("put site contacts: %w", err)
	}
	return out, nil
}

func normalizeContacts(c Contacts) Contacts {
	out := Contacts{
		Phone:        strings.TrimSpace(c.Phone),
		PhoneTel:     strings.TrimSpace(c.PhoneTel),
		Email:        strings.TrimSpace(c.Email),
		Address:      strings.TrimSpace(c.Address),
		Hours:        strings.TrimSpace(c.Hours),
		Telegram:     strings.TrimSpace(c.Telegram),
		TelegramURL:  strings.TrimSpace(c.TelegramURL),
		Instagram:    strings.TrimSpace(c.Instagram),
		InstagramURL: strings.TrimSpace(c.InstagramURL),
	}
	// Display phone and tel: href are often filled independently in admin.
	// Cross-fill so a single saved field still updates the live footer.
	if out.Phone == "" && out.PhoneTel != "" {
		out.Phone = out.PhoneTel
	}
	if out.PhoneTel == "" && out.Phone != "" {
		out.PhoneTel = compactPhoneTel(out.Phone)
	}
	return out
}

// compactPhoneTel keeps digits and a leading +, for tel: hrefs.
func compactPhoneTel(display string) string {
	var b strings.Builder
	for i, r := range display {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
			continue
		}
		if r == '+' && i == 0 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func validateContacts(c Contacts) error {
	fields := []struct {
		name  string
		value string
	}{
		{"phone", c.Phone},
		{"phoneTel", c.PhoneTel},
		{"email", c.Email},
		{"address", c.Address},
		{"hours", c.Hours},
		{"telegram", c.Telegram},
		{"telegramUrl", c.TelegramURL},
		{"instagram", c.Instagram},
		{"instagramUrl", c.InstagramURL},
	}
	for _, f := range fields {
		if utf8.RuneCountInString(f.value) > maxFieldLen {
			return fmt.Errorf("field %s too long", f.name)
		}
	}
	if err := validateSocialURL("telegramUrl", c.TelegramURL, map[string]bool{
		"t.me": true, "telegram.me": true,
	}); err != nil {
		return err
	}
	if err := validateSocialURL("instagramUrl", c.InstagramURL, map[string]bool{
		"instagram.com": true, "www.instagram.com": true,
	}); err != nil {
		return err
	}
	return nil
}

func validateSocialURL(field, value string, allowedHosts map[string]bool) error {
	if value == "" {
		return nil
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || !allowedHosts[strings.ToLower(u.Hostname())] || u.User != nil {
		return fmt.Errorf("field %s must be an allowed https URL", field)
	}
	return nil
}
