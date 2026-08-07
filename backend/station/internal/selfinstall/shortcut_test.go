package selfinstall

import (
	"errors"
	"testing"
)

func TestEnsureShortcutReportsWhatItDid(t *testing.T) {
	var gotTarget string
	restore := createShortcutFn
	t.Cleanup(func() { createShortcutFn = restore })
	createShortcutFn = func(target string) (string, bool, error) {
		gotTarget = target
		return `C:\Users\Public\Desktop\DriverGo.lnk`, true, nil
	}

	path, created, err := EnsureShortcut(`C:\ProgramData\AvtoTest\station\avtotest-station.exe`)
	if err != nil {
		t.Fatalf("EnsureShortcut: %v", err)
	}
	if !created {
		t.Fatal("created = false, want true when the shortcut was written")
	}
	if path != `C:\Users\Public\Desktop\DriverGo.lnk` {
		t.Fatalf("path = %q", path)
	}
	if gotTarget != `C:\ProgramData\AvtoTest\station\avtotest-station.exe` {
		t.Fatalf("shortcut points at %q, want the installed copy", gotTarget)
	}
}

// An existing shortcut must be left alone. A school that deleted the icon on
// purpose should not find it back the next morning, and rewriting it every
// boot would also reset any customisation.
func TestEnsureShortcutLeavesAnExistingOneAlone(t *testing.T) {
	restore := createShortcutFn
	t.Cleanup(func() { createShortcutFn = restore })
	createShortcutFn = func(string) (string, bool, error) {
		return `C:\Users\Public\Desktop\DriverGo.lnk`, false, nil
	}

	_, created, err := EnsureShortcut(`C:\whatever.exe`)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("created = true, want false when the shortcut was already there")
	}
}

// The caller treats a failure as cosmetic, but it must still be able to tell
// one apart from success -- a locked-down classroom profile that cannot write
// to the desktop should show up in the log, not vanish.
func TestEnsureShortcutSurfacesFailures(t *testing.T) {
	restore := createShortcutFn
	t.Cleanup(func() { createShortcutFn = restore })
	sentinel := errors.New("access is denied")
	createShortcutFn = func(string) (string, bool, error) { return "", false, sentinel }

	_, created, err := EnsureShortcut(`C:\whatever.exe`)
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want it to wrap the underlying failure", err)
	}
	if created {
		t.Fatal("created = true on a failure")
	}
}
