// This file implements -selftest and -selftest-import: a way for someone
// with no Go toolchain to prove, on the actual Windows machine the agent
// will run on, that the two things development on Linux cannot exercise —
// DPAPI sealing (keystore_windows.go) and the registry-based hardware id
// (hwid_windows.go) — genuinely work. Neither mode touches the real state
// directory; both operate on temporary directories that are removed before
// the process exits.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"os"

	"avtotest.uz/station/internal/hwid"
	"avtotest.uz/station/internal/keystore"
)

// check is one line of the self-test report.
type check struct {
	name string
	ok   bool
	info string
}

// runSelfTest exercises hwid.Collect and the real keystore.Store on this
// machine, in a scratch temp directory, and prints a plain-language verdict
// for each step plus one summary line. It returns a process exit code: 0
// only if every check passed.
func runSelfTest() int {
	fmt.Println("AvtoTest station self-test")
	fmt.Println("===========================")
	fmt.Println()
	fmt.Println("This checks, on THIS machine, whether the code that never runs during")
	fmt.Println("normal Go development on Linux (DPAPI key sealing, the Windows registry")
	fmt.Println("hardware id) actually does what it claims. It does not touch this")
	fmt.Println("machine's real enrollment; everything below runs in a temporary directory")
	fmt.Println("that is deleted when this command finishes.")
	fmt.Println()

	dir, err := os.MkdirTemp("", "avtotest-station-selftest-*")
	if err != nil {
		fmt.Printf("FATAL: could not create a temporary directory: %v\n", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(dir) }()

	var checks []check
	var priv ed25519.PrivateKey // populated by checkSealRoundTrip, read by later checks
	var raw []byte              // the sealed bytes actually written to disk

	checks = append(checks, checkHardwareID())
	c, p := checkSealRoundTrip(dir)
	checks = append(checks, c)
	priv = p

	if priv != nil {
		c, r := checkFileIsSealed(dir, priv)
		checks = append(checks, c)
		raw = r
	} else {
		checks = append(checks, check{
			name: "Key file is genuinely sealed on disk",
			ok:   false,
			info: "skipped: the seal round-trip above did not produce a key to compare against",
		})
	}

	if raw != nil {
		checks = append(checks, checkTamperRejection(dir, raw))
	} else {
		checks = append(checks, check{
			name: "Tamper rejection",
			ok:   false,
			info: "skipped: no sealed file bytes to tamper with",
		})
	}

	checks = append(checks, checkEmptyFileRejection(dir))

	failed := 0
	for i, c := range checks {
		status := "PASS"
		if !c.ok {
			status = "FAIL"
			failed++
		}
		fmt.Printf("[%d/%d] %-40s %s\n", i+1, len(checks), c.name, status)
		if c.info != "" {
			fmt.Printf("        %s\n", c.info)
		}
	}

	fmt.Println()
	if failed == 0 {
		fmt.Printf("SELFTEST RESULT: PASS — all %d checks passed on this machine.\n", len(checks))
	} else {
		fmt.Printf("SELFTEST RESULT: FAIL — %d of %d checks failed on this machine.\n", failed, len(checks))
	}

	fmt.Println()
	fmt.Println("One property cannot be checked on a single machine: that a key sealed")
	fmt.Println("here is UNREADABLE elsewhere. To finish verifying it:")
	fmt.Println()
	fmt.Println(`  1. Enroll or run this agent normally, so it writes a real`)
	fmt.Println(`     %ProgramData%\AvtoTest\station\station.key on this PC.`)
	fmt.Println(`  2. Copy that station.key file to a SECOND Windows PC (a USB drive is`)
	fmt.Println(`     fine — the whole point is that the copy should be worthless).`)
	fmt.Println(`  3. On that second PC, run:`)
	fmt.Println(`         avtotest-station.exe -selftest-import <path-to-copied-station.key>`)
	fmt.Println(`     and read its verdict carefully — for this check, an error is the`)
	fmt.Println(`     GOOD outcome.`)

	if failed > 0 {
		return 1
	}
	return 0
}

// checkHardwareID validates that hwid.Collect returns the documented shape:
// a 64-character lowercase hex sha256 digest. A failure here means the
// registry read in hwid_windows.go did not work on this machine.
func checkHardwareID() check {
	id, err := hwid.Collect()
	if err != nil {
		return check{name: "Hardware id (hwid.Collect)", ok: false,
			info: fmt.Sprintf("hwid.Collect() returned an error: %v", err)}
	}
	if !isLowerHex64(id) {
		return check{name: "Hardware id (hwid.Collect)", ok: false,
			info: fmt.Sprintf("hwid.Collect() returned %q, want 64 lowercase hex characters", id)}
	}
	return check{name: "Hardware id (hwid.Collect)", ok: true,
		info: fmt.Sprintf("hwid = %s", id)}
}

func isLowerHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

// checkSealRoundTrip generates a real key, saves it through keystore.Store
// (the exact code path production uses), reopens the store fresh, and loads
// it back. This is what proves DPAPI seal and unseal actually agree with
// each other on this machine — not just that they compile. It returns the
// original key so later checks can compare against it.
func checkSealRoundTrip(dir string) (check, ed25519.PrivateKey) {
	name := "Seal round-trip (keystore.Store)"

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return check{name: name, ok: false, info: fmt.Sprintf("could not generate a test key: %v", err)}, nil
	}

	store, err := keystore.Open(dir)
	if err != nil {
		return check{name: name, ok: false, info: fmt.Sprintf("keystore.Open: %v", err)}, nil
	}
	if err := store.Save(priv); err != nil {
		return check{name: name, ok: false, info: fmt.Sprintf("Store.Save: %v", err)}, nil
	}

	// Reopen fresh, so this exercises loading a key back from disk rather
	// than any in-memory state the first Store might be keeping.
	reopened, err := keystore.Open(dir)
	if err != nil {
		return check{name: name, ok: false, info: fmt.Sprintf("keystore.Open (reopen): %v", err)}, priv
	}
	got, err := reopened.Load()
	if err != nil {
		return check{name: name, ok: false, info: fmt.Sprintf("Store.Load after Save: %v", err)}, priv
	}
	if !bytes.Equal(got, priv) {
		return check{name: name, ok: false, info: "the key loaded back does not match the key that was saved"}, priv
	}
	return check{name: name, ok: true, info: "generated a key, saved it, reopened the store, loaded it back: identical"}, priv
}

// checkFileIsSealed reads the raw bytes keystore.Store actually wrote to
// disk and asserts they are neither equal to, nor contain as a substring,
// the plaintext private key. On Windows this is what the DPAPI ciphertext
// buys; on Linux keystore_other.go stores the key in the clear by design
// (see its doc comment), so this check FAILS there — that failure is
// correct and is the signal that the dev build is not the shippable one.
func checkFileIsSealed(dir string, priv ed25519.PrivateKey) (check, []byte) {
	name := "Key file is genuinely sealed on disk"

	raw, err := os.ReadFile(keystore.KeyPath(dir))
	if err != nil {
		return check{name: name, ok: false, info: fmt.Sprintf("could not read %s: %v", keystore.KeyPath(dir), err)}, nil
	}

	if bytes.Equal(raw, []byte(priv)) {
		return check{name: name, ok: false,
			info: "station.key on disk is byte-for-byte the plaintext private key — NOT sealed. " +
				"Expected on the non-Windows dev build (see keystore_other.go); must never be true on the Windows build."}, raw
	}
	if bytes.Contains(raw, []byte(priv)) {
		return check{name: name, ok: false,
			info: "station.key on disk contains the plaintext private key as a substring — NOT sealed."}, raw
	}
	return check{name: name, ok: true,
		info: fmt.Sprintf("station.key on disk (%d bytes) contains no copy of the plaintext key", len(raw))}, raw
}

// checkTamperRejection flips one byte of the sealed file and asserts Load
// reports keystore.ErrCorruptKey rather than returning a silently wrong key
// or panicking. A corrupted DPAPI blob must fail the DPAPI MAC check inside
// CryptUnprotectData, not be "successfully" unsealed into garbage.
func checkTamperRejection(dir string, sealed []byte) (result check) {
	name := "Tamper rejection"

	// Named return so that if Load ever panics instead of erroring — which
	// would itself be a finding worth surfacing — the deferred recover below
	// still produces a properly labeled FAIL line instead of a blank one.
	defer func() {
		if r := recover(); r != nil {
			result = check{name: name, ok: false, info: fmt.Sprintf("panic while loading a tampered key: %v", r)}
		}
	}()

	tampered := append([]byte(nil), sealed...)
	tampered[0] ^= 0xFF
	if err := os.WriteFile(keystore.KeyPath(dir), tampered, 0o600); err != nil {
		return check{name: name, ok: false, info: fmt.Sprintf("could not write tampered file: %v", err)}
	}

	store, err := keystore.Open(dir)
	if err != nil {
		return check{name: name, ok: false, info: fmt.Sprintf("keystore.Open: %v", err)}
	}

	_, err = store.Load()
	if errors.Is(err, keystore.ErrCorruptKey) {
		return check{name: name, ok: true, info: "flipped a byte in the sealed file; Load correctly returned ErrCorruptKey"}
	}
	if err == nil {
		return check{name: name, ok: false, info: "flipped a byte in the sealed file and Load returned a key anyway (no integrity check) — this must be ErrCorruptKey"}
	}
	return check{name: name, ok: false, info: fmt.Sprintf("flipped a byte in the sealed file; Load returned %v, want ErrCorruptKey", err)}
}

// checkEmptyFileRejection truncates station.key to zero bytes — the shape
// left behind by an interrupted write, a full disk, or an AV quarantine —
// and asserts Load returns ErrCorruptKey rather than panicking. keystore.go
// guards this explicitly because the platform seal/unseal functions index
// their input unconditionally.
func checkEmptyFileRejection(dir string) check {
	name := "Empty file rejection"

	if err := os.WriteFile(keystore.KeyPath(dir), nil, 0o600); err != nil {
		return check{name: name, ok: false, info: fmt.Sprintf("could not write empty file: %v", err)}
	}

	store, err := keystore.Open(dir)
	if err != nil {
		return check{name: name, ok: false, info: fmt.Sprintf("keystore.Open: %v", err)}
	}

	_, err = store.Load()
	if errors.Is(err, keystore.ErrCorruptKey) {
		return check{name: name, ok: true, info: "truncated the key file to 0 bytes; Load correctly returned ErrCorruptKey"}
	}
	if err == nil {
		return check{name: name, ok: false, info: "truncated the key file to 0 bytes and Load returned a key anyway — this must be ErrCorruptKey"}
	}
	return check{name: name, ok: false, info: fmt.Sprintf("truncated the key file to 0 bytes; Load returned %v, want ErrCorruptKey", err)}
}

// runSelfTestImport takes a station.key copied from another machine and
// attempts to unseal it here, in a scratch directory — it never writes to
// srcPath itself. This is the half of the security property that a single
// machine cannot demonstrate on its own.
//
// Read the wording of the result carefully: succeeding at unsealing the
// imported key is the BAD outcome — it means the key is not actually bound
// to the machine that created it. An error from Load is the GOOD outcome:
// it means DPAPI refused to unseal ciphertext that belongs to different
// hardware, which is exactly the protection this agent depends on.
func runSelfTestImport(srcPath string) int {
	fmt.Println("AvtoTest station self-test: cross-machine import check")
	fmt.Println("========================================================")
	fmt.Println()
	fmt.Printf("Reading %s (read-only; this file is not modified) ...\n", srcPath)

	raw, err := os.ReadFile(srcPath)
	if err != nil {
		fmt.Printf("\nERROR: could not read %s: %v\n", srcPath, err)
		fmt.Println("This is a plain file-access problem (wrong path, permissions, etc.),")
		fmt.Println("not a result about the security property. Fix the path and try again.")
		return 2
	}

	dir, err := os.MkdirTemp("", "avtotest-station-selftest-import-*")
	if err != nil {
		fmt.Printf("\nERROR: could not create a temporary directory: %v\n", err)
		return 2
	}
	defer func() { _ = os.RemoveAll(dir) }()

	if err := os.WriteFile(keystore.KeyPath(dir), raw, 0o600); err != nil {
		fmt.Printf("\nERROR: could not stage the copied file for testing: %v\n", err)
		return 2
	}

	store, err := keystore.Open(dir)
	if err != nil {
		fmt.Printf("\nERROR: keystore.Open: %v\n", err)
		return 2
	}

	key, err := store.Load()
	fmt.Println()
	if err == nil && len(key) == ed25519.PrivateKeySize {
		fmt.Println("RESULT: SECURITY FAILURE.")
		fmt.Println()
		fmt.Println("This machine was able to read the private key out of a station.key file")
		fmt.Println("that was copied from a DIFFERENT machine. The DPAPI machine binding did")
		fmt.Println("NOT hold. A station's identity is supposed to be unusable anywhere but")
		fmt.Println("the PC that created it — that guarantee is broken if you are seeing this.")
		fmt.Println()
		fmt.Println("Do not treat this as a harmless pass. Stop and report it.")
		return 1
	}

	fmt.Println("RESULT: correctly bound to its original machine.")
	fmt.Println()
	fmt.Printf("Unsealing failed on this machine (%v), which is the GOOD outcome: the\n", err)
	fmt.Println("copied key is unreadable here. The DPAPI machine binding held — a")
	fmt.Println("station.key stolen or copied from this PC is worthless anywhere else.")
	return 0
}
