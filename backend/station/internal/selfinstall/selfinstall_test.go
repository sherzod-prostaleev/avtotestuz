// Package selfinstall_test exercises the platform-independent half of
// Ensure and Remove: copying the running binary into place and doing so
// idempotently. Autostart registration is a no-op off Windows (see
// selfinstall_other.go), so nothing here proves the registry half works --
// that is covered by the Windows cross-build plus the -selftest agent
// mode's autostart round-trip check, which has to be run by hand on a real
// Windows machine.
package selfinstall_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"avtotest.uz/station/internal/selfinstall"
)

// helperEnv, when set in the environment, tells TestMain that this process
// is the re-exec'd helper spawned by TestEnsureSkipsWhenAlreadyRunningFromTarget,
// not a normal `go test` invocation.
const helperEnv = "SELFINSTALL_TEST_HELPER_STATE_DIR"

// TestMain lets TestEnsureSkipsWhenAlreadyRunningFromTarget re-exec this
// very test binary from inside the directory it copies itself to. That is
// the only way to make os.Executable() genuinely report "I am running from
// Target(stateDir)" inside Ensure -- there is no way to fake the standard
// library's answer from within the same process.
func TestMain(m *testing.M) {
	if dir := os.Getenv(helperEnv); dir != "" {
		os.Exit(runEnsureHelper(dir))
	}
	os.Exit(m.Run())
}

// runEnsureHelper calls Ensure exactly as production code would, from a
// process whose own executable already sits at Target(dir), and prints the
// result on stdout in a form the parent test can parse.
func runEnsureHelper(dir string) int {
	target, didInstall, err := selfinstall.Ensure(dir)
	if err != nil {
		fmt.Fprintf(os.Stdout, "err=%v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "target=%s\ndidInstall=%v\n", target, didInstall)
	return 0
}

// TestEnsureCopiesOnceAndIsIdempotent covers the ordinary path: a fresh
// state dir gets the running binary copied into it once, and a second call
// -- the shape every later boot takes -- must not recopy or touch the
// installed file's mtime.
func TestEnsureCopiesOnceAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()

	target, didInstall, err := selfinstall.Ensure(dir)
	if err != nil {
		t.Fatalf("first Ensure() error = %v", err)
	}
	if !didInstall {
		t.Fatal("first Ensure() didInstall = false, want true")
	}
	if want := selfinstall.Target(dir); target != want {
		t.Fatalf("first Ensure() target = %q, want %q", target, want)
	}

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable(): %v", err)
	}
	selfBytes, err := os.ReadFile(self)
	if err != nil {
		t.Fatalf("reading the running test binary: %v", err)
	}
	installedBytes, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading the installed copy: %v", err)
	}
	if !bytes.Equal(selfBytes, installedBytes) {
		t.Fatal("installed copy bytes differ from the running test binary's own file")
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat installed copy: %v", err)
	}
	mtime := info.ModTime()

	// A brief pause makes a spurious rewrite detectable: without it, a
	// same-tick rewrite could land an identical mtime by coincidence on a
	// coarse filesystem clock and this test would pass for the wrong reason.
	time.Sleep(10 * time.Millisecond)

	target2, didInstall2, err := selfinstall.Ensure(dir)
	if err != nil {
		t.Fatalf("second Ensure() error = %v", err)
	}
	if didInstall2 {
		t.Fatal("second Ensure() didInstall = true, want false: a re-run must not recopy")
	}
	if target2 != target {
		t.Fatalf("second Ensure() target = %q, want %q", target2, target)
	}

	info2, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat installed copy after second Ensure: %v", err)
	}
	if !info2.ModTime().Equal(mtime) {
		t.Fatalf("installed copy mtime changed from %v to %v: second Ensure() rewrote the file", mtime, info2.ModTime())
	}
}

// TestEnsureSkipsWhenAlreadyRunningFromTarget covers the case that would
// otherwise break every boot after the first: on Windows, copying a running
// image over itself fails outright with a sharing violation. It stages a
// copy of the test binary directly at Target(dir), re-execs itself from
// that exact path (see TestMain / runEnsureHelper above), and asserts the
// helper's Ensure call detects that and returns without copying.
func TestEnsureSkipsWhenAlreadyRunningFromTarget(t *testing.T) {
	dir := t.TempDir()
	target := selfinstall.Target(dir)

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable(): %v", err)
	}
	selfBytes, err := os.ReadFile(self)
	if err != nil {
		t.Fatalf("reading the running test binary: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, selfBytes, 0o755); err != nil {
		t.Fatalf("staging a copy at the target path: %v", err)
	}

	cmd := exec.Command(target)
	cmd.Env = append(os.Environ(), helperEnv+"="+dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helper subprocess failed: %v\noutput:\n%s", err, out)
	}

	wantTarget := "target=" + target
	if !bytes.Contains(out, []byte(wantTarget)) {
		t.Fatalf("helper output = %q, want it to contain %q", out, wantTarget)
	}
	if !bytes.Contains(out, []byte("didInstall=false")) {
		t.Fatalf("helper output = %q, want didInstall=false: Ensure must not copy onto its own running image", out)
	}
}

// TestRemoveDeletesTheInstalledCopy covers reversibility: after Ensure,
// Remove must leave no target file behind, and calling it a second time --
// an operator double-clicking an uninstall shortcut twice, or uninstalling
// a PC that was never installed -- must not error just because there is
// nothing left to remove.
func TestRemoveDeletesTheInstalledCopy(t *testing.T) {
	dir := t.TempDir()

	target, _, err := selfinstall.Ensure(dir)
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("installed copy missing right after Ensure: %v", err)
	}

	if err := selfinstall.Remove(dir); err != nil {
		t.Fatalf("first Remove() error = %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target still exists after Remove(): stat err = %v", err)
	}

	if err := selfinstall.Remove(dir); err != nil {
		t.Fatalf("second Remove() error = %v, want nil (idempotent)", err)
	}
}
