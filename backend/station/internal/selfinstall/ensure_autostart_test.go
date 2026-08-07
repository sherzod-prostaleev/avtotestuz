package selfinstall

import "testing"

// TestEnsureRegistersAutostartEvenWhenAlreadyInstalled proves the exists
// branch in Ensure no longer skips autostart registration once the binary
// is already in place. Without this, a registerAutostart failure on the
// very first run (a transient registry error, a profile not fully loaded
// yet) would be logged once and then silently, permanently skipped on every
// later boot -- defeating the one guarantee this package exists to
// provide. registerAutostart itself is a no-op off Windows (see
// selfinstall_other.go), so it has no side effect this test could check for
// directly; it lives in package selfinstall (not selfinstall_test) so it
// can substitute registerAutostartFn, the seam Ensure calls through, to
// make the call observable.
func TestEnsureRegistersAutostartEvenWhenAlreadyInstalled(t *testing.T) {
	dir := t.TempDir()

	calls := 0
	prev := registerAutostartFn
	registerAutostartFn = func(path string) error {
		calls++
		return nil
	}
	defer func() { registerAutostartFn = prev }()

	if _, didInstall, err := Ensure(dir); err != nil {
		t.Fatalf("first Ensure() error = %v", err)
	} else if !didInstall {
		t.Fatal("first Ensure() didInstall = false, want true: nothing was installed yet")
	}
	if calls != 1 {
		t.Fatalf("registerAutostartFn calls after first Ensure() = %d, want 1", calls)
	}

	// The case this test exists for: the binary is already in place, so
	// didInstall must be false on this second call -- but autostart must
	// still be (re-)registered rather than skipped.
	if _, didInstall, err := Ensure(dir); err != nil {
		t.Fatalf("second Ensure() error = %v", err)
	} else if didInstall {
		t.Fatal("second Ensure() didInstall = true, want false: the binary was already installed")
	}
	if calls != 2 {
		t.Fatalf("registerAutostartFn calls after second Ensure() = %d, want 2: autostart must be re-registered even when the binary is already in place", calls)
	}
}
