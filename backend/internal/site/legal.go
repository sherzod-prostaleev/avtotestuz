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

const legalKey = "legal"

// Legal docs can be long; keep a hard ceiling against abuse.
const maxLegalFieldLen = 50000

// LegalLocales are the FE UI locales that legal CMS covers.
var LegalLocales = []string{"uz-Latn", "uz-Cyrl", "ru"}

// LegalDoc is one locale's legal page bodies. Empty strings → i18n fallback.
type LegalDoc struct {
	Oferta  string `json:"oferta"`
	Privacy string `json:"privacy"`
	Refund  string `json:"refund"`
}

// LegalBundle is the CMS document stored under site_settings.legal.
type LegalBundle struct {
	Locales map[string]LegalDoc `json:"locales"`
}

// GetLegalBundle returns the full multi-locale legal CMS document.
func (s Store) GetLegalBundle(ctx context.Context) (LegalBundle, error) {
	if s.Pool == nil {
		return emptyLegalBundle(), nil
	}
	var raw []byte
	err := s.Pool.QueryRow(ctx, `
		SELECT value_json FROM site_settings WHERE key = $1`, legalKey).Scan(&raw)
	if err != nil {
		if err == pgx.ErrNoRows {
			return emptyLegalBundle(), nil
		}
		return LegalBundle{}, fmt.Errorf("get site legal: %w", err)
	}
	if len(raw) == 0 || string(raw) == "null" {
		return emptyLegalBundle(), nil
	}
	var out LegalBundle
	if err := json.Unmarshal(raw, &out); err != nil {
		return LegalBundle{}, fmt.Errorf("decode site legal: %w", err)
	}
	return normalizeLegalBundle(out), nil
}

// PutLegalBundle replaces the legal CMS document.
func (s Store) PutLegalBundle(ctx context.Context, in LegalBundle, updatedBy string) (LegalBundle, error) {
	if s.Pool == nil {
		return LegalBundle{}, fmt.Errorf("site: PutLegalBundle requires Pool")
	}
	out := normalizeLegalBundle(in)
	if err := validateLegalBundle(out); err != nil {
		return LegalBundle{}, err
	}
	if updatedBy == "" {
		updatedBy = "ops"
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return LegalBundle{}, fmt.Errorf("encode site legal: %w", err)
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO site_settings (key, value_json, updated_at, updated_by)
		VALUES ($1, $2::jsonb, $3, $4)
		ON CONFLICT (key) DO UPDATE
		  SET value_json = EXCLUDED.value_json,
		      updated_at = EXCLUDED.updated_at,
		      updated_by = EXCLUDED.updated_by`,
		legalKey, raw, time.Now().UTC(), updatedBy,
	)
	if err != nil {
		return LegalBundle{}, fmt.Errorf("put site legal: %w", err)
	}
	return out, nil
}

// GetLegalDoc returns one locale's legal bodies (empty when unset).
func (s Store) GetLegalDoc(ctx context.Context, locale string) (LegalDoc, error) {
	bundle, err := s.GetLegalBundle(ctx)
	if err != nil {
		return LegalDoc{}, err
	}
	locale = normalizeLegalLocale(locale)
	if doc, ok := bundle.Locales[locale]; ok {
		return doc, nil
	}
	return LegalDoc{}, nil
}

func emptyLegalBundle() LegalBundle {
	locales := make(map[string]LegalDoc, len(LegalLocales))
	for _, loc := range LegalLocales {
		locales[loc] = LegalDoc{}
	}
	return LegalBundle{Locales: locales}
}

func normalizeLegalLocale(locale string) string {
	locale = strings.TrimSpace(locale)
	for _, loc := range LegalLocales {
		if locale == loc {
			return loc
		}
	}
	return "uz-Latn"
}

func normalizeLegalDoc(d LegalDoc) LegalDoc {
	return LegalDoc{
		Oferta:  strings.TrimSpace(d.Oferta),
		Privacy: strings.TrimSpace(d.Privacy),
		Refund:  strings.TrimSpace(d.Refund),
	}
}

func normalizeLegalBundle(b LegalBundle) LegalBundle {
	out := emptyLegalBundle()
	if b.Locales == nil {
		return out
	}
	for _, loc := range LegalLocales {
		if doc, ok := b.Locales[loc]; ok {
			out.Locales[loc] = normalizeLegalDoc(doc)
		}
	}
	return out
}

func validateLegalBundle(b LegalBundle) error {
	for loc, doc := range b.Locales {
		fields := []struct {
			name  string
			value string
		}{
			{"oferta", doc.Oferta},
			{"privacy", doc.Privacy},
			{"refund", doc.Refund},
		}
		for _, f := range fields {
			if utf8.RuneCountInString(f.value) > maxLegalFieldLen {
				return fmt.Errorf("field %s.%s too long", loc, f.name)
			}
		}
	}
	return nil
}
