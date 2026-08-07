// Package embedcfg reads the configuration the admin panel appends to a
// downloaded agent, so a classroom PC needs nothing typed into it.
//
// The layout is written by the backend's b2b.AppendConfig and must stay
// byte-compatible with it:
//
//	[base bytes][config JSON][uint32 big-endian JSON length][16-byte magic]
//
// The two sides live in separate Go modules, so the format is duplicated
// rather than shared. A byte-level layout test alone cannot catch every
// drift — e.g. a renamed JSON tag on one side round-trips fine against its
// own hardcoded fixture and leaves the other side silently reading a zero
// value. To close that gap, testdata/golden.bin is a fixture produced by the
// real b2b.AppendConfig: this package's tests read it directly, and the
// backend has a test that regenerates the same bytes with AppendConfig and
// asserts they are byte-identical to it. A field rename on either side
// breaks that comparison.
package embedcfg

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

const magic = "AVTOSTATIONCFG01"

const trailerLen = 4 + len(magic)

// maxConfigLen mirrors the writer's bound; a length past it means a corrupt or
// hostile tail rather than a configuration this program wrote.
const maxConfigLen = 8 << 10

// ErrNoConfig means the file carries no appended configuration — an ordinary
// unconfigured build, which is not an error at the call site.
var ErrNoConfig = errors.New("no embedded configuration")

// Config is what the admin panel baked into this copy of the agent.
type Config struct {
	Code     string `json:"code"`
	API      string `json:"api"`
	Frontend string `json:"frontend"`
	Org      string `json:"org"`
	Locale   string `json:"locale"`
}

// Read returns the configuration appended to the file at path.
func Read(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer func() { _ = f.Close() }()

	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return Config{}, err
	}
	if size < int64(trailerLen) {
		return Config{}, ErrNoConfig
	}

	trailer := make([]byte, trailerLen)
	if _, err := f.ReadAt(trailer, size-int64(trailerLen)); err != nil {
		return Config{}, err
	}
	if string(trailer[4:]) != magic {
		return Config{}, ErrNoConfig
	}

	n := int64(binary.BigEndian.Uint32(trailer[:4]))
	if n == 0 || n > maxConfigLen || n > size-int64(trailerLen) {
		return Config{}, fmt.Errorf("embedded config length %d is not plausible for a %d-byte file", n, size)
	}

	payload := make([]byte, n)
	if _, err := f.ReadAt(payload, size-int64(trailerLen)-n); err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(payload, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse embedded config: %w", err)
	}
	return cfg, nil
}
