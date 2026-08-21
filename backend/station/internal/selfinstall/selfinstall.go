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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"avtotest.uz/station/internal/keystore"
)

// installedName is the file name the agent is copied to inside the state
// directory. It matches the name every other doc and script in this repo
// already uses for the shipped binary (see backend/station/README.md and
// backend/internal/config's StationBinaryPath default) regardless of which
// OS this particular build was compiled for -- the shipped article is
// always the Windows one.
const installedName = "avtotest-station.exe"

// stateFileName is the file internal/agent's Agent persists enrollment state
// (station id, org, label) to. It duplicates the literal in agent.go's
// unexported statePath rather than importing the agent package to reuse it:
// agent has no exported accessor for it -- unlike keystore.KeyPath, which is
// kept exported for exactly this "let another package name the file" need
// -- and pulling in agent's HTTP/enrollment dependencies here just to name
// one file would be a worse trade than one duplicated string.
const stateFileName = "station.json"

// registerAutostartFn is registerAutostart by default. Ensure calls through
// this indirection, rather than the function directly, so a test can
// substitute it and observe that autostart registration happens -- which
// matters because registerAutostart itself is a no-op off Windows (see
// selfinstall_other.go), and a no-op has no side effect a test could
// otherwise check for.
var registerAutostartFn = registerAutostart

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
	// the existing installed copy alone instead of rewriting it every time.
	//
	// Autostart is deliberately NOT gated behind this same check, though.
	// This used to be "return early, autostart already done" -- but that
	// conflated "the binary is in place" with "autostart is registered",
	// and the two can fall out of sync: if copyExecutable above succeeds on
	// some earlier run but registerAutostartFn then fails (a transient
	// registry error, a profile not fully loaded yet), the error is logged
	// once and every later boot would take this branch and never retry
	// autostart -- silently and permanently defeating the one guarantee
	// this package exists to provide. Writing the same registry value again
	// is harmless, so it is retried unconditionally below instead.
	// A stale .old from a previous upgrade, if the delete below could not run
	// because the replaced image was still mapped. Harmless to leave, so a
	// failure here is ignored.
	_ = os.Remove(target + ".old")

	didInstall := false
	if _, err := os.Stat(target); err == nil {
		// Already installed -- but not necessarily the same build. Ensure used
		// to stop here, which meant the copy that autostart launches every
		// morning stayed whatever version was installed first: a school that
		// downloaded a fixed .exe ran the fix once, from Downloads, and went
		// back to the broken build on the next reboot. Compare the bytes and
		// replace when they differ.
		same, cmpErr := sameContent(exePath, target)
		if cmpErr != nil {
			return "", false, fmt.Errorf("selfinstall: compare installed copy: %w", cmpErr)
		}
		if !same {
			// Bytes, in either direction: running an installer that is OLDER
			// than the installed copy downgrades it. That is a real
			// possibility now that the agent updates itself -- an admin
			// double-clicking last month's download from a Downloads folder
			// would undo it -- and it is left alone deliberately rather than
			// guarded with a version comparison. internal/updater checks the
			// manifest two minutes after every start, so a downgrade repairs
			// itself within minutes, whereas refusing to install would break
			// the one recovery path a school has when a build genuinely is
			// bad: hand them a known-good .exe and tell them to run it.
			if err := replaceExecutable(exePath, target); err != nil {
				return "", false, fmt.Errorf("selfinstall: replace installed copy: %w", err)
			}
			didInstall = true
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", false, fmt.Errorf("selfinstall: stat target: %w", err)
	} else {
		if err := os.MkdirAll(stateDir, 0o755); err != nil {
			return "", false, fmt.Errorf("selfinstall: create state dir: %w", err)
		}
		if err := copyExecutable(exePath, target); err != nil {
			return "", false, fmt.Errorf("selfinstall: copy into place: %w", err)
		}
		didInstall = true
	}

	if err := registerAutostartFn(target); err != nil {
		return "", false, fmt.Errorf("selfinstall: register autostart: %w", err)
	}

	return target, didInstall, nil
}

// Remove deregisters autostart and deletes the installed copy plus this
// station's local state: the sealed private key (station.key) and the
// enrollment record (station.json). Deleting the state, not just the
// binary, matters because either file surviving on a decommissioned PC
// means dropping any build of the agent back into this directory later
// would silently re-authenticate as the old station with no re-enrolment --
// the only thing standing between a decommissioned PC and a live session
// would otherwise be an operator remembering to revoke it in the admin
// panel. All three removals are idempotent: a missing registry value or a
// missing file counts as success, not an error, so running -uninstall
// twice -- or once on a PC that was never installed -- never fails.
func Remove(stateDir string) error {
	if err := unregisterAutostart(); err != nil {
		return fmt.Errorf("selfinstall: unregister autostart: %w", err)
	}
	// station.log is written beside the key (see cmd/avtotest-station/diag.go)
	// and is part of "this station's local state" the doc comment promises to
	// clear, so a decommissioned PC does not keep a log of a school it no
	// longer belongs to.
	if err := os.Remove(filepath.Join(stateDir, "station.log")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("selfinstall: remove log: %w", err)
	}
	if err := removeShortcut(); err != nil {
		return fmt.Errorf("selfinstall: remove desktop shortcut: %w", err)
	}
	if err := os.Remove(Target(stateDir)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("selfinstall: remove installed copy: %w", err)
	}
	if err := os.Remove(keystore.KeyPath(stateDir)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("selfinstall: remove station key: %w", err)
	}
	if err := os.Remove(filepath.Join(stateDir, stateFileName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("selfinstall: remove station state: %w", err)
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

// sameContent reports whether two files hold identical bytes.
//
// Content, not version string or mtime: the version is a linker flag that a
// rebuild at the same version would not change, and mtime says nothing about
// what is inside a file that rsync or a download touched.
func sameContent(a, b string) (bool, error) {
	ha, err := fileSHA256(a)
	if err != nil {
		return false, err
	}
	hb, err := fileSHA256(b)
	if err != nil {
		return false, err
	}
	return ha == hb, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// replaceExecutable swaps a new build over an installed one.
//
// Rename first, then write. Windows refuses to open a running image for
// writing but will happily rename it, so this works even when the copy being
// replaced is the one autostart launched at boot: the running process keeps
// executing from the renamed file until it exits, and the next start picks up
// the new binary. Deleting the renamed file usually fails while it is still
// mapped, which is why Ensure sweeps it on the following run.
func replaceExecutable(src, dst string) error {
	old := dst + ".old"
	_ = os.Remove(old)
	if err := os.Rename(dst, old); err != nil {
		return err
	}
	if err := copyExecutable(src, dst); err != nil {
		// Put the working binary back rather than leave the PC with none.
		_ = os.Rename(old, dst)
		return err
	}
	_ = os.Remove(old)
	return nil
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
