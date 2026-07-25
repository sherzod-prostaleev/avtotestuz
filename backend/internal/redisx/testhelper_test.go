package redisx

import (
	"fmt"
	"testing"
)

// The registry is hand-maintained, so guard the invariants that make it safe.
func TestTestDBAssignmentsAreDistinctAndSafe(t *testing.T) {
	seen := map[int]string{}
	for pkg, idx := range testDBByPackage {
		// Two packages on one index flush each other's keys — the exact
		// failure this registry exists to prevent.
		if other, dup := seen[idx]; dup {
			t.Errorf("packages %q and %q share Redis database %d", other, pkg, idx)
		}
		seen[idx] = pkg

		// Database 0 is the dev/app database (REDIS_URL defaults to .../0) and
		// NewTest flushes whatever it is handed.
		if idx == 0 {
			t.Errorf("package %q is assigned Redis database 0, which is the dev database", pkg)
		}
		// Redis ships with 16 logical databases; a higher index only works on a
		// server that was reconfigured, so it would fail on a fresh checkout.
		if idx < 0 || idx > 15 {
			t.Errorf("package %q is assigned Redis database %d, outside the default 0-15 range", pkg, idx)
		}
	}
}

func TestURLWithDB(t *testing.T) {
	cases := []struct {
		base string
		idx  int
		want string
	}{
		{"redis://localhost:6379/0", 3, "redis://localhost:6379/3"},
		{"redis://localhost:6379/11", 2, "redis://localhost:6379/2"},
		// No index in the URL at all: append one rather than mangle the host.
		{"redis://localhost:6379", 4, "redis://localhost:6379/4"},
		{"redis://localhost:6379/", 4, "redis://localhost:6379/4"},
		// A password must survive untouched.
		{"redis://:secret@localhost:6379/1", 5, "redis://:secret@localhost:6379/5"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s->%d", c.base, c.idx), func(t *testing.T) {
			got, err := urlWithDB(c.base, c.idx)
			if err != nil {
				t.Fatalf("urlWithDB(%q, %d): %v", c.base, c.idx, err)
			}
			if got != c.want {
				t.Errorf("urlWithDB(%q, %d) = %q, want %q", c.base, c.idx, got, c.want)
			}
		})
	}
}
