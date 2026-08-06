package embedcfg_test

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
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
		binary.BigEndian.PutUint32(raw[len(raw)-20:len(raw)-16], 1<<20)
		if err := os.WriteFile(p, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := embedcfg.Read(p); err == nil {
			t.Fatal("want an error for a length past the start of the file")
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		p := build(t, "BASE", `{"code":`)
		if _, err := embedcfg.Read(p); err == nil {
			t.Fatal("want an error for malformed json")
		}
	})
}
