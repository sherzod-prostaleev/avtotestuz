// Package testenv identifies the test package that is currently running, so
// shared infrastructure (Postgres in internal/testdb, Redis in internal/redisx)
// can be partitioned per package.
//
// Both of those helpers wipe their store on setup, which is safe within a
// package but destructive across packages: a plain `go test ./...` used to have
// packages clearing each other's fixtures mid-run, producing deadlocks and
// missing-row failures that read as real bugs in the code under test.
package testenv

import (
	"os"
	"path/filepath"
	"strings"
)

// moduleDir is the directory name of the Go module root, used to find where
// the package-relative part of the working directory begins.
const moduleDir = "backend"

// PackageSlug returns a filesystem-safe identifier for the calling test
// package, e.g. "internal_billing_payme".
//
// `go test` runs each package's binary with that package's source directory as
// its working directory, which makes the path a reliable package identity
// without threading a name through every call site. Returns "unknown" if the
// working directory cannot be read — callers get one shared partition rather
// than a hard failure, which is no worse than the behaviour this replaces.
func PackageSlug() string {
	wd, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return SlugForDir(wd)
}

// SlugForDir is PackageSlug for an explicit directory; separated so it can be
// tested without changing the process working directory.
func SlugForDir(dir string) string {
	parts := strings.Split(filepath.ToSlash(dir), "/")
	// Keep the path from the module root down, so internal/billing and
	// cmd/billing cannot collapse into the same slug.
	for i, p := range parts {
		if p == moduleDir {
			parts = parts[i+1:]
			break
		}
	}
	var b strings.Builder
	for _, r := range strings.ToLower(strings.Join(parts, "_")) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	slug := strings.Trim(b.String(), "_")
	if slug == "" {
		return "unknown"
	}
	return slug
}
