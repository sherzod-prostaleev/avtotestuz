package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// deadConsole stands in for os.Stderr on a -H windowsgui build launched from
// autostart: there is no console, GetStdHandle returns an invalid handle, and
// every write fails.
type deadConsole struct{}

func (deadConsole) Write([]byte) (int, error) { return 0, errors.New("invalid handle") }

// TestTeeWritesTheFileEvenWithNoConsole is the whole reason tee exists.
//
// The shipped agent has no console window, so station.log is the only record
// of anything it does. io.MultiWriter(os.Stderr, f) stops at the first writer
// that errors, which on those machines is stderr -- producing an empty log
// file on exactly the PCs a support call is about.
func TestTeeWritesTheFileEvenWithNoConsole(t *testing.T) {
	var file bytes.Buffer
	w := tee{file: &file, console: deadConsole{}}

	if _, err := w.Write([]byte("station started\n")); err == nil {
		t.Log("a healthy file write reports no error, which is what log.Output ignores anyway")
	}
	if got := file.String(); got != "station started\n" {
		t.Fatalf("log file got %q, want the line written despite the dead console", got)
	}
}

// TestTeeStillReachesAConsoleWhenThereIsOne keeps -selftest and -uninstall
// working: those attach the parent console and print for a human.
func TestTeeStillReachesAConsoleWhenThereIsOne(t *testing.T) {
	var file, console bytes.Buffer
	w := tee{file: &file, console: &console}

	if _, err := w.Write([]byte("hello\n")); err != nil {
		t.Fatalf("Write() = %v", err)
	}
	if file.String() != "hello\n" || console.String() != "hello\n" {
		t.Fatalf("file=%q console=%q, want both to receive the line", file.String(), console.String())
	}
}

// TestStartLoggingFallsBackToAWritableDirectory covers the PC where a previous
// install ran under a different Windows account: %ProgramData% grants the
// creating account ownership of what it made, so the current user can be
// unable to write there -- and a support call that arrives with no log at all
// is the worst possible starting point.
func TestStartLoggingFallsBackToAWritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root defeats the unwritable-directory setup")
	}
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.MkdirAll(blocked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o700) })

	fallback := filepath.Join(root, "fallback")
	t.Setenv("LOCALAPPDATA", fallback)

	got := startLogging(blocked)

	if !strings.HasPrefix(got, fallback) {
		t.Fatalf("startLogging() = %q, want a path under the fallback %q", got, fallback)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("no log file at the reported path: %v", err)
	}
}

// TestRotateIfLargeKeepsOneGeneration. A station that cannot reach the backend
// writes a line every couple of minutes forever; without rotation that file
// grows for the life of the machine.
func TestRotateIfLargeKeepsOneGeneration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "station.log")
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), maxLogBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}

	rotateIfLarge(path)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("the oversized log was not moved aside: %v", err)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("no rotated generation kept: %v", err)
	}
}
