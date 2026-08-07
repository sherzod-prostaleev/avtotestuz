package selfinstall

import (
	"os"
	"path/filepath"
	"testing"
)

// Ensure used to stop at "the file is there", so the copy autostart launches
// every morning stayed whatever version was installed first. A school that
// downloaded a fixed .exe ran the fix once, from Downloads, and went back to
// the broken build on the next reboot -- the whole point of shipping a fix.
func TestEnsureReplacesAnOlderInstalledBuild(t *testing.T) {
	stateDir := t.TempDir()
	target := Target(stateDir)
	if err := os.WriteFile(target, []byte("OLD BUILD"), 0o755); err != nil {
		t.Fatal(err)
	}

	restore := registerAutostartFn
	t.Cleanup(func() { registerAutostartFn = restore })
	registerAutostartFn = func(string) error { return nil }

	got, didInstall, err := Ensure(stateDir)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if got != target {
		t.Fatalf("Ensure returned %q, want %q", got, target)
	}
	if !didInstall {
		t.Fatal("didInstall = false, want true when the installed copy was a different build")
	}

	installed, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	running, err := os.ReadFile(mustExecutable(t))
	if err != nil {
		t.Fatal(err)
	}
	if string(installed) != string(running) {
		t.Fatal("the installed copy is still the old build")
	}
	if _, err := os.Stat(target + ".old"); err == nil {
		t.Fatal(".old left behind on a platform that can delete it")
	}
}

// The common case must stay cheap and side-effect free: the same build
// already in place is not an upgrade, and must not be reported as one.
func TestEnsureLeavesAnIdenticalInstalledBuildAlone(t *testing.T) {
	stateDir := t.TempDir()
	target := Target(stateDir)

	running, err := os.ReadFile(mustExecutable(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, running, 0o755); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}

	restore := registerAutostartFn
	t.Cleanup(func() { registerAutostartFn = restore })
	registerAutostartFn = func(string) error { return nil }

	if _, didInstall, err := Ensure(stateDir); err != nil {
		t.Fatal(err)
	} else if didInstall {
		t.Fatal("didInstall = true for a byte-identical installed copy")
	}

	after, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("an identical build was rewritten anyway")
	}
}

// A leftover .old from an upgrade whose delete could not run (Windows keeps a
// running image mapped) must be swept, not accumulate one file per upgrade.
func TestEnsureSweepsAStaleOldFile(t *testing.T) {
	stateDir := t.TempDir()
	target := Target(stateDir)
	running, err := os.ReadFile(mustExecutable(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, running, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := target + ".old"
	if err := os.WriteFile(stale, []byte("previous build"), 0o755); err != nil {
		t.Fatal(err)
	}

	restore := registerAutostartFn
	t.Cleanup(func() { registerAutostartFn = restore })
	registerAutostartFn = func(string) error { return nil }

	if _, _, err := Ensure(stateDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Fatalf("%s survived Ensure", filepath.Base(stale))
	}
}

func mustExecutable(t *testing.T) string {
	t.Helper()
	p, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return p
}
