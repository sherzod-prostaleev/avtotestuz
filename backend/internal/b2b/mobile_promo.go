package b2b

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/url"
	"strings"
	"unicode"

	"github.com/boombuler/barcode/qr"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type MobilePromo struct {
	Enabled   bool   `json:"enabled"`
	URL       string `json:"url"`
	QRDataURL string `json:"qr_data_url,omitempty"`
}

// ValidateMobilePromo never normalizes the value: the QR must encode the
// administrator's exact bytes, including case, escaping, query and fragment.
func ValidateMobilePromo(enabled bool, raw string) error {
	if raw == "" && !enabled {
		return nil
	}
	if raw == "" || len(raw) > 512 || strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return ErrInvalid
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || u.Opaque != "" || strings.Contains(raw, "\\") {
		return ErrInvalid
	}
	return nil
}

func (p *MobilePromo) GenerateQR() error {
	if p.URL == "" {
		return nil
	}
	if err := ValidateMobilePromo(p.Enabled, p.URL); err != nil {
		return err
	}
	code, err := qr.Encode(p.URL, qr.M, qr.Unicode)
	if err != nil {
		return err
	}
	// Four white modules on every side (quiet zone), four pixels per module.
	// Integer scaling keeps the code sharp and independent of external services.
	side := (code.Bounds().Dx() + 8) * 4
	img := image.NewGray(image.Rect(0, 0, side, side))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	for y := 0; y < code.Bounds().Dy(); y++ {
		for x := 0; x < code.Bounds().Dx(); x++ {
			draw.Draw(img, image.Rect((x+4)*4, (y+4)*4, (x+5)*4, (y+5)*4), image.NewUniform(code.At(x, y)), image.Point{}, draw.Src)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	p.QRDataURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	return nil
}

func (s Store) MobilePromo(ctx context.Context, orgID uuid.UUID) (MobilePromo, error) {
	var p MobilePromo
	err := s.Pool.QueryRow(ctx, `SELECT mobile_promo_enabled, mobile_promo_url FROM b2b_org WHERE id=$1`, orgID).Scan(&p.Enabled, &p.URL)
	if err != nil {
		return p, err
	}
	return p, p.GenerateQR()
}

// The authenticated station profile, never a caller-supplied org ID, scopes
// the banner. Revoked stations and suspended/expired schools see no advert.
func (s Store) StationMobilePromo(ctx context.Context, profileID uuid.UUID) (MobilePromo, error) {
	var p MobilePromo
	err := s.Pool.QueryRow(ctx, `SELECT o.mobile_promo_enabled, o.mobile_promo_url
 FROM b2b_station s JOIN b2b_org o ON o.id=s.org_id
 WHERE s.station_profile_id=$1 AND s.status='active' AND o.status='active'
 AND EXISTS (SELECT 1 FROM b2b_org_license l WHERE l.org_id=o.id AND l.starts_at<=now() AND l.ends_at>now())`, profileID).Scan(&p.Enabled, &p.URL)
	if errors.Is(err, pgx.ErrNoRows) {
		return MobilePromo{}, nil
	}
	if err != nil {
		return MobilePromo{}, err
	}
	if !p.Enabled {
		return MobilePromo{}, nil
	}
	return p, p.GenerateQR()
}
