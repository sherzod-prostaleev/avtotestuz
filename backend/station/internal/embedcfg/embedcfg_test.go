package embedcfg_test

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"avtotest.uz/station/internal/embedcfg"
)

// build produces a file in the same layout the backend's AppendConfig writes:
// [base][json][uint32 BE len][16-byte magic].
func build(t *testing.T, base, jsonBody string) string {
	t.Helper()
	buf := []byte(base + jsonBody)
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(jsonBody)))
	buf = append(buf, n[:]...)
	buf = append(buf, []byte("AVTOSTATIONCFG01")...)

	p := filepath.Join(t.TempDir(), "agent.exe")
	if err := os.WriteFile(p, buf, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestReadGoldenFixture reads testdata/golden.bin, a file produced by the
// real backend b2b.AppendConfig (not by the build() helper above, which
// only mimics the layout). A matching test on the backend side regenerates
// the same bytes with AppendConfig and asserts they are byte-identical to
// this fixture, so a field rename on either side — where build()'s hardcoded
// JSON literal would otherwise stay silent — breaks one of the two tests.
func TestReadGoldenFixture(t *testing.T) {
	cfg, err := embedcfg.Read("testdata/golden.bin")
	if err != nil {
		t.Fatal(err)
	}
	want := embedcfg.Config{
		Code:     "AVTO-K7M2-P9XQ",
		API:      "https://api.drivergo.uz",
		Frontend: "https://drivergo.uz",
		Org:      "1-sonli avtomaktab",
		Locale:   "uz-Latn",
	}
	if cfg != want {
		t.Fatalf("cfg=%+v, want %+v", cfg, want)
	}
}

func TestReadRoundTrip(t *testing.T) {
	p := build(t, "PRETEND-EXE-BYTES",
		`{"code":"AVTO-K7M2-P9XQ","api":"https://drivergo.uz","frontend":"https://drivergo.uz","org":"avto","locale":"uz-Latn"}`)

	cfg, err := embedcfg.Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Code != "AVTO-K7M2-P9XQ" || cfg.API != "https://drivergo.uz" || cfg.Locale != "uz-Latn" {
		t.Fatalf("cfg=%+v", cfg)
	}
}

func TestReadRejectsBadTrailers(t *testing.T) {
	t.Run("no trailer", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "plain.exe")
		if err := os.WriteFile(p, []byte("JUST-AN-EXE"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := embedcfg.Read(p); !errors.Is(err, embedcfg.ErrNoConfig) {
			t.Fatalf("err=%v, want ErrNoConfig", err)
		}
	})

	t.Run("wrong magic", func(t *testing.T) {
		p := build(t, "BASE", `{"code":"x"}`)
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		raw[len(raw)-1] = 'X'
		if err := os.WriteFile(p, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := embedcfg.Read(p); !errors.Is(err, embedcfg.ErrNoConfig) {
			t.Fatalf("err=%v, want ErrNoConfig", err)
		}
	})

	t.Run("length longer than the file", func(t *testing.T) {
		p := build(t, "BASE", `{"code":"x"}`)
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		// n must stay under maxConfigLen (8192) so this exercises the
		// n > size-trailerLen bound specifically. If n were also above
		// maxConfigLen (as the old 1<<20 literal was), that clause would
		// reject the input first and this subtest could never tell a
		// correct size bound from a broken one.
		const trailerLen = 20 // 4-byte length + 16-byte magic
		n := uint32(len(raw) - trailerLen + 1)
		if n > 8192 {
			t.Fatalf("test fixture too small: n=%d would also trip maxConfigLen", n)
		}
		binary.BigEndian.PutUint32(raw[len(raw)-20:len(raw)-16], n)
		if err := os.WriteFile(p, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := embedcfg.Read(p); err == nil {
			t.Fatal("want an error for a length past the start of the file")
		}
	})

	t.Run("length exceeds maxConfigLen", func(t *testing.T) {
		// A base large enough that size-trailerLen comfortably exceeds
		// maxConfigLen (8192), so only the maxConfigLen clause can reject
		// this — isolating it from the size-bound subtest above.
		base := strings.Repeat("B", 8192+1000)
		p := build(t, base, `{"code":"x"}`)
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		const trailerLen = 20
		n := uint32(8192 + 1)
		if int(n) > len(raw)-trailerLen {
			t.Fatalf("test fixture too small: n=%d would also trip the size bound", n)
		}
		binary.BigEndian.PutUint32(raw[len(raw)-20:len(raw)-16], n)
		if err := os.WriteFile(p, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := embedcfg.Read(p); err == nil {
			t.Fatal("want an error for a length past maxConfigLen")
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		p := build(t, "BASE", `{"code":`)
		if _, err := embedcfg.Read(p); err == nil {
			t.Fatal("want an error for malformed json")
		}
	})
}
