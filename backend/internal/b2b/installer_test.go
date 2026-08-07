package b2b_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/b2b"
)

// goldenFixturePath is the fixture committed under the station module so a
// single file backs the cross-module byte-compatibility check from both
// sides: this test regenerates it with AppendConfig, and
// station/internal/embedcfg's TestReadGoldenFixture reads it with the real
// reader. See the package comment on embedcfg.go for why a layout test
// alone (each side pinning its own hardcoded literal) does not catch a
// renamed JSON tag on one side.
const goldenFixturePath = "../../station/internal/embedcfg/testdata/golden.bin"

// TestInstallerGolden proves AppendConfig still produces exactly the bytes
// committed at goldenFixturePath. If a JSON tag on InstallerConfig is
// renamed without a matching change on the station side, this test is the
// one that fails — TestAppendConfigLayout alone would not, since it only
// round-trips into this package's own (equally renamed) type.
func TestInstallerGolden(t *testing.T) {
	cfg := b2b.InstallerConfig{
		Code:     "AVTO-K7M2-P9XQ",
		API:      "https://api.drivergo.uz",
		Frontend: "https://drivergo.uz",
		Org:      "1-sonli avtomaktab",
		Locale:   "uz-Latn",
	}
	base := []byte("FAKE-EXE-BASE-BYTES-FOR-GOLDEN-FIXTURE")

	var out bytes.Buffer
	if err := b2b.AppendConfig(&out, bytes.NewReader(base), cfg); err != nil {
		t.Fatal(err)
	}

	want, err := os.ReadFile(goldenFixturePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), want) {
		t.Fatalf("AppendConfig output diverged from %s (%d bytes) — regenerate the fixture only if this divergence is intentional:\ngot:  % x\nwant: % x",
			goldenFixturePath, len(want), out.Bytes(), want)
	}
}

func TestAppendConfigLayout(t *testing.T) {
	base := []byte("PRETEND-EXE-BYTES")
	cfg := b2b.InstallerConfig{
		Code: "AVTO-K7M2-P9XQ", API: "https://drivergo.uz",
		Frontend: "https://drivergo.uz", Org: "avto", Locale: "uz-Latn",
	}

	var out bytes.Buffer
	if err := b2b.AppendConfig(&out, bytes.NewReader(base), cfg); err != nil {
		t.Fatal(err)
	}
	blob := out.Bytes()

	// The base must be byte-identical and come first: the appended trailer is
	// what keeps the PE image itself untouched.
	if !bytes.HasPrefix(blob, base) {
		t.Fatal("base bytes were modified")
	}

	// Trailer: [json][uint32 BE len][16-byte magic]
	if got := string(blob[len(blob)-16:]); got != "AVTOSTATIONCFG01" {
		t.Fatalf("magic=%q", got)
	}
	n := binary.BigEndian.Uint32(blob[len(blob)-20 : len(blob)-16])
	jsonStart := len(blob) - 20 - int(n)
	if jsonStart != len(base) {
		t.Fatalf("json starts at %d, want %d (right after the base)", jsonStart, len(base))
	}

	var back b2b.InstallerConfig
	if err := json.Unmarshal(blob[jsonStart:len(blob)-20], &back); err != nil {
		t.Fatal(err)
	}
	if back != cfg {
		t.Fatalf("round trip changed the config: %+v", back)
	}
}

func TestInstallerFilename(t *testing.T) {
	id := uuid.MustParse("11111111-2222-3333-4444-555555555555")

	if got := b2b.InstallerFilename("1-sonli avtomaktab", id); got != "avtotest-station-1-sonli-avtomaktab.exe" {
		t.Fatalf("got %q", got)
	}
	// A name with no ASCII left after sanitising falls back to the org id, so
	// the download never lands as an unnamed or empty-slug file.
	got := b2b.InstallerFilename("Автомактаб", id)
	if !strings.HasPrefix(got, "avtotest-station-11111111") || !strings.HasSuffix(got, ".exe") {
		t.Fatalf("got %q, want the org-id fallback", got)
	}
}
