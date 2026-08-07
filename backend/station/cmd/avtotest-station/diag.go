package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// logFileName sits beside the station key so one folder holds everything a
// support call needs.
const logFileName = "station.log"

// startLogging mirrors every log line into stateDir/station.log.
//
// A classroom PC is usually started by double-clicking the .exe, and Windows
// destroys that console the instant the process exits -- so an agent that
// dies on startup takes its own error message with it and the school reports
// only "the window disappeared". The file survives, and -uninstall deletes it
// with the rest of the state.
//
// Failing to open the log is not fatal: a read-only or full disk is a reason
// to run without a log, not a reason to refuse to run.
func startLogging(stateDir string) {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(stateDir, logFileName),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.Printf("---- avtotest-station %s starting (%s/%s, pid %d) ----",
		version, runtime.GOOS, runtime.GOARCH, os.Getpid())
}

// fatal replaces log.Fatalf on the startup path.
//
// It writes the message the same way, then -- on Windows, where a
// double-clicked console vanishes with the process -- waits for Enter so
// whoever is standing at the PC can actually read what went wrong. The wait
// is skipped when stdin is not a terminal (a GPO rollout, a scheduled task,
// a piped run), where blocking forever would be worse than exiting quietly:
// the message is in station.log either way.
func fatal(format string, args ...any) {
	log.Printf(format, args...)
	holdConsole()
	os.Exit(1)
}

func holdConsole() {
	if runtime.GOOS != "windows" {
		return
	}
	st, err := os.Stdin.Stat()
	if err != nil || st.Mode()&os.ModeCharDevice == 0 {
		return
	}
	fmt.Fprintln(os.Stderr, "\nPress Enter to close this window...")
	// Bounded so an unattended PC does not sit on this forever with a window
	// nobody will ever close.
	done := make(chan struct{})
	go func() {
		var b [1]byte
		_, _ = os.Stdin.Read(b[:])
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Minute):
	}
}
