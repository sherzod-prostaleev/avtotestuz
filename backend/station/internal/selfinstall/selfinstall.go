// Package selfinstall copies the running agent into the state directory and
// registers it to start automatically on login, so a classroom PC comes
// back up on its own after every reboot without an operator retyping
// anything.
//
// It only ever runs for a build that was configured for a specific school
// (embedcfg.Config.Code != ""); main.go gates the call on that, so a plain
// development build never writes an autostart entry on someone's machine.
package selfinstall

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// installedName is the file name the agent is copied to inside the state
// directory. It matches the name every other doc and script in this repo
// already uses for the shipped binary (see backend/station/README.md and
// backend/internal/config's StationBinaryPath default) regardless of which
// OS this particular build was compiled for -- the shipped article is
// always the Windows one.
const installedName = "avtotest-station.exe"

// Target returns the path the agent runs from once installed.
func Target(stateDir string) string {
	return filepath.Join(stateDir, installedName)
}

// Ensure copies the running binary into Target(stateDir) and registers
// autostart unless that has already happened. It is meant to be called on
// every startup: idempotent once installed, and guarded against copying a
// running image onto itself.
func Ensure(stateDir string) (string, bool, error) {
	target := Target(stateDir)

	exePath, err := os.Executable()
	if err != nil {
		return "", false, fmt.Errorf("selfinstall: resolve running executable: %w", err)
	}

	// A running image cannot be copied over itself -- on Windows this fails
	// outright with a sharing violation, since the OS loader holds the file
	// open for as long as the process is mapped from it. This is the case
	// that would otherwise break every boot after the first, once the agent
	// is already running from Target(stateDir).
	if samePath(exePath, target) {
		return target, false, nil
	}

	// Idempotent: once installed, a later call (the next boot, a login
	// script firing on every logon rather than just the first) must leave
	// the existing installed copy and its autostart entry alone instead of
	// rewriting them every time.
	if _, err := os.Stat(target); err == nil {
		return target, false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", false, fmt.Errorf("selfinstall: stat target: %w", err)
	}

	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return "", false, fmt.Errorf("selfinstall: create state dir: %w", err)
	}

	if err := copyExecutable(exePath, target); err != nil {
		return "", false, fmt.Errorf("selfinstall: copy into place: %w", err)
	}

	if err := registerAutostart(target); err != nil {
		return "", false, fmt.Errorf("selfinstall: register autostart: %w", err)
	}

	return target, true, nil
}

// Remove deregisters autostart and deletes the installed copy. Both halves
// are idempotent: a missing registry value or a missing file counts as
// success, not an error, so running -uninstall twice -- or once on a PC
// that was never installed -- never fails.
func Remove(stateDir string) error {
	if err := unregisterAutostart(); err != nil {
		return fmt.Errorf("selfinstall: unregister autostart: %w", err)
	}
	if err := os.Remove(Target(stateDir)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("selfinstall: remove installed copy: %w", err)
	}
	return nil
}

// samePath reports whether a and b name the same file. A clean string
// comparison is tried first -- cheap, and already correct whenever Target's
// file does not exist yet, since two paths trivially cannot name the same
// file if one of them names nothing -- and falls back to comparing what the
// paths actually stat to, so a path that reaches the same inode by a
// different-looking route (e.g. through a symlinked temp directory) is
// still recognised as the same file.
func samePath(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	fa, err := os.Stat(a)
	if err != nil {
		return false
	}
	fb, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(fa, fb)
}

// copyExecutable copies src to dst with mode 0o755 via a temp file in dst's
// own directory, renamed into place only once the copy has fully succeeded
// -- so a crash or power loss mid-copy leaves either the previous dst
// untouched or no dst at all, never a truncated binary that fails to start.
func copyExecutable(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".avtotest-station-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		// Only litter cleanup: once Rename below succeeds, tmpPath no
		// longer exists under this name and Remove is a harmless no-op.
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err = io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, dst)
}

// targetWritable proves the exact directory Ensure would create and write
// into can be created and written by whichever user is running this
// process. It exists for the -selftest diagnostic: a disposable scratch
// temp directory is trivially writable everywhere and would prove nothing
// about the real install location a given machine will actually use.
func targetWritable(stateDir string) (ok bool, info string) {
	dir := filepath.Dir(Target(stateDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Sprintf("could not create %s: %v", dir, err)
	}
	probe := filepath.Join(dir, ".avtotest-selftest-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return false, fmt.Sprintf("could not write a probe file in %s: %v", dir, err)
	}
	_ = os.Remove(probe)
	return true, fmt.Sprintf("%s can be created and written", dir)
}
