package selfinstall

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTargetWritableCreatesAndWritesTheRealTargetDir exercises targetWritable
// directly (it lives in package selfinstall, not selfinstall_test, because
// the function is unexported). SelfTestTargetWritable only calls it from the
// Windows build -- the non-Windows self-test stub deliberately reports "not
// applicable" instead, per the brief -- which would otherwise leave this
// small piece of genuinely portable filesystem logic completely unexercised
// by any test run on Linux.
func TestTargetWritableCreatesAndWritesTheRealTargetDir(t *testing.T) {
	base := t.TempDir()
	// A nested, not-yet-existing state dir, so this also proves MkdirAll
	// creates the missing directories rather than merely writing into one
	// that already exists.
	stateDir := filepath.Join(base, "AvtoTest", "station")

	ok, info := targetWritable(stateDir)
	if !ok {
		t.Fatalf("targetWritable() ok = false, info = %q", info)
	}

	wantDir := filepath.Dir(Target(stateDir))
	if fi, err := os.Stat(wantDir); err != nil || !fi.IsDir() {
		t.Fatalf("expected %s to exist as a directory after targetWritable(), stat err = %v", wantDir, err)
	}

	// The probe file it writes to prove write access must not be left
	// behind -- it is a diagnostic side effect, not installed state.
	entries, err := os.ReadDir(wantDir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", wantDir, err)
	}
	if len(entries) != 0 {
		t.Fatalf("targetWritable() left %d entries behind in %s, want the probe file cleaned up", len(entries), wantDir)
	}
}
