package b2b

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// configMagic marks a station binary that carries an appended configuration.
// Exactly 16 bytes; the reader in the station module matches on it verbatim,
// so changing it breaks every installer already handed out.
const configMagic = "AVTOSTATIONCFG01"

// maxConfigLen bounds what the reader will trust from a file's tail.
const maxConfigLen = 8 << 10

// InstallerConfig is what a downloaded agent needs to reach its own school
// without anything being typed on the PC.
type InstallerConfig struct {
	Code     string `json:"code"`
	API      string `json:"api"`
	Frontend string `json:"frontend"`
	Org      string `json:"org"`
	Locale   string `json:"locale"`
}

// AppendConfig streams base to w and appends cfg as a trailer:
//
//	[base bytes][config JSON][uint32 big-endian JSON length][magic]
//
// Windows tolerates trailing bytes after a PE image, so the binary stays
// runnable and its signature-free copy stays byte-identical up to the trailer.
// The length prefix means the reader seeks straight to the JSON instead of
// scanning for a marker that could occur inside the binary by chance.
func AppendConfig(w io.Writer, base io.Reader, cfg InstallerConfig) error {
	payload, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if len(payload) > maxConfigLen {
		return fmt.Errorf("installer config too large: %d bytes", len(payload))
	}
	if _, err := io.Copy(w, base); err != nil {
		return fmt.Errorf("copy station binary: %w", err)
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(payload)))
	if _, err := w.Write(n[:]); err != nil {
		return err
	}
	_, err = io.WriteString(w, configMagic)
	return err
}

// InstallerFilename builds the download name. Org names are frequently Cyrillic
// and browsers mangle non-ASCII filenames differently, so the slug is ASCII-only
// and falls back to the org id when nothing usable survives.
func InstallerFilename(orgName string, orgID uuid.UUID) string {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(orgName) {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = orgID.String()[:8]
	}
	return "avtotest-station-" + slug + ".exe"
}
