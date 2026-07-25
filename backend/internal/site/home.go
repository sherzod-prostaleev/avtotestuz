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

const homeHeroKey = "home_hero"

const maxHeroFieldLen = 500
const maxHeroHrefLen = 200

// HomeHero is the landing hero CMS document. Empty strings → i18n fallback.
type HomeHero struct {
	Headline string `json:"headline"`
	Subtitle string `json:"subtitle"`
	CTALabel string `json:"ctaLabel"`
	CTAHref  string `json:"ctaHref"`
}

// GetHomeHero returns stored hero copy or empty fields when missing.
func (s Store) GetHomeHero(ctx context.Context) (HomeHero, error) {
	if s.Pool == nil {
		return HomeHero{}, nil
	}
	var raw []byte
	err := s.Pool.QueryRow(ctx, `
		SELECT value_json FROM site_settings WHERE key = $1`, homeHeroKey).Scan(&raw)
	if err != nil {
		if err == pgx.ErrNoRows {
			return HomeHero{}, nil
		}
		return HomeHero{}, fmt.Errorf("get home hero: %w", err)
	}
	var out HomeHero
	if len(raw) == 0 || string(raw) == "null" {
		return HomeHero{}, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return HomeHero{}, fmt.Errorf("decode home hero: %w", err)
	}
	return normalizeHomeHero(out), nil
}

// PutHomeHero replaces the hero document.
func (s Store) PutHomeHero(ctx context.Context, in HomeHero, updatedBy string) (HomeHero, error) {
	if s.Pool == nil {
		return HomeHero{}, fmt.Errorf("site: PutHomeHero requires Pool")
	}
	out := normalizeHomeHero(in)
	if err := validateHomeHero(out); err != nil {
		return HomeHero{}, err
	}
	if updatedBy == "" {
		updatedBy = "ops"
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return HomeHero{}, fmt.Errorf("encode home hero: %w", err)
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO site_settings (key, value_json, updated_at, updated_by)
		VALUES ($1, $2::jsonb, $3, $4)
		ON CONFLICT (key) DO UPDATE
		  SET value_json = EXCLUDED.value_json,
		      updated_at = EXCLUDED.updated_at,
		      updated_by = EXCLUDED.updated_by`,
		homeHeroKey, raw, time.Now().UTC(), updatedBy,
	)
	if err != nil {
		return HomeHero{}, fmt.Errorf("put home hero: %w", err)
	}
	return out, nil
}

func normalizeHomeHero(h HomeHero) HomeHero {
	return HomeHero{
		Headline: strings.TrimSpace(h.Headline),
		Subtitle: strings.TrimSpace(h.Subtitle),
		CTALabel: strings.TrimSpace(h.CTALabel),
		CTAHref:  strings.TrimSpace(h.CTAHref),
	}
}

func validateHomeHero(h HomeHero) error {
	fields := []struct {
		name  string
		value string
		max   int
	}{
		{"headline", h.Headline, maxHeroFieldLen},
		{"subtitle", h.Subtitle, maxHeroFieldLen},
		{"ctaLabel", h.CTALabel, maxHeroFieldLen},
		{"ctaHref", h.CTAHref, maxHeroHrefLen},
	}
	for _, f := range fields {
		if utf8.RuneCountInString(f.value) > f.max {
			return fmt.Errorf("field %s too long", f.name)
		}
	}
	if h.CTAHref != "" {
		if !strings.HasPrefix(h.CTAHref, "/") || strings.HasPrefix(h.CTAHref, "//") {
			return fmt.Errorf("ctaHref must be a relative path")
		}
		if strings.ContainsAny(h.CTAHref, " \t\n\r") {
			return fmt.Errorf("ctaHref invalid")
		}
	}
	return nil
}
