package site

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const supportBannerKey = "support_banner"

const maxBannerMessageLen = 280

// SupportBanner is the in-app support announcement (site_settings.support_banner).
type SupportBanner struct {
	Enabled   bool   `json:"enabled"`
	Message   string `json:"message"`
	Href      string `json:"href,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
	UpdatedBy string `json:"updated_by,omitempty"`
}

// GetSupportBanner returns the stored banner or a disabled empty banner.
func (s Store) GetSupportBanner(ctx context.Context) (SupportBanner, error) {
	if s.Pool == nil {
		return SupportBanner{}, nil
	}
	var raw []byte
	err := s.Pool.QueryRow(ctx, `
		SELECT value_json FROM site_settings WHERE key = $1`, supportBannerKey).Scan(&raw)
	if err != nil {
		if err == pgx.ErrNoRows {
			return SupportBanner{}, nil
		}
		return SupportBanner{}, fmt.Errorf("get support banner: %w", err)
	}
	var out SupportBanner
	if len(raw) == 0 || string(raw) == "null" {
		return SupportBanner{}, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return SupportBanner{}, fmt.Errorf("decode support banner: %w", err)
	}
	return normalizeBanner(out), nil
}

// PutSupportBanner replaces the support banner document.
func (s Store) PutSupportBanner(ctx context.Context, in SupportBanner, updatedBy string) (SupportBanner, error) {
	if s.Pool == nil {
		return SupportBanner{}, fmt.Errorf("site: PutSupportBanner requires Pool")
	}
	out := normalizeBanner(in)
	if err := validateBanner(out); err != nil {
		return SupportBanner{}, err
	}
	if updatedBy == "" {
		updatedBy = "admin"
	}
	out.UpdatedBy = updatedBy
	out.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	raw, err := json.Marshal(out)
	if err != nil {
		return SupportBanner{}, fmt.Errorf("encode support banner: %w", err)
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO site_settings (key, value_json, updated_at, updated_by)
		VALUES ($1, $2::jsonb, now(), $3)
		ON CONFLICT (key) DO UPDATE
		  SET value_json = EXCLUDED.value_json,
		      updated_at = EXCLUDED.updated_at,
		      updated_by = EXCLUDED.updated_by`,
		supportBannerKey, raw, updatedBy,
	)
	if err != nil {
		return SupportBanner{}, fmt.Errorf("put support banner: %w", err)
	}
	return out, nil
}

func normalizeBanner(b SupportBanner) SupportBanner {
	href := strings.TrimSpace(b.Href)
	if href != "" && (!strings.HasPrefix(href, "/") || strings.HasPrefix(href, "//") || strings.Contains(href, "://")) {
		href = ""
	}
	return SupportBanner{
		Enabled:   b.Enabled,
		Message:   strings.TrimSpace(b.Message),
		Href:      href,
		UpdatedAt: strings.TrimSpace(b.UpdatedAt),
		UpdatedBy: strings.TrimSpace(b.UpdatedBy),
	}
}

func validateBanner(b SupportBanner) error {
	if b.Enabled && b.Message == "" {
		return fmt.Errorf("message required when enabled")
	}
	if utf8.RuneCountInString(b.Message) > maxBannerMessageLen {
		return fmt.Errorf("message too long")
	}
	return nil
}
