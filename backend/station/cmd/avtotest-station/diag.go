package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

// logFileName sits beside the station key so one folder holds everything a
// support call needs.
const logFileName = "station.log"

// maxLogBytes caps the log before it is rotated to station.log.1. A station
// that cannot reach the backend writes a line every couple of minutes and
// would otherwise grow that file for the life of the machine.
const maxLogBytes = 2 << 20

// startLogging mirrors every log line into a file and returns the path it
// chose, so the banner and the kiosk status page can both name it.
//
// A classroom PC runs this with no console at all (the binary is linked as a
// GUI application so schools do not see a black window), which makes the file
// the only record that exists. It is written beside the station key in
// %ProgramData%, with a fall-back to the user's own %LOCALAPPDATA%: ProgramData
// grants the creating account ownership of what it makes, so a PC where a
// previous install ran under a different Windows account can leave the current
// user unable to write there -- and a support call that arrives with no log at
// all is the worst possible starting point.
//
// Failing to open any log is not fatal: a read-only or full disk is a reason
// to run without a log, not a reason to refuse to run.
func startLogging(stateDir string) string {
	for _, dir := range logDirs(stateDir) {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}
		path := filepath.Join(dir, logFileName)
		rotateIfLarge(path)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			continue
		}
		log.SetOutput(tee{file: f, console: os.Stderr})
		log.Printf("---- avtotest-station %s starting (%s/%s, pid %d) ----",
			version, runtime.GOOS, runtime.GOARCH, os.Getpid())
		return path
	}
	return "(yozib bo'lmadi)"
}

// tee writes every log line to the file first and to the console second,
// treating the console as best-effort.
//
// io.MultiWriter cannot be used here, and the reason is specific to this
// binary. It stops at the first writer that errors, and the shipped agent is
// linked -H windowsgui, so a classroom PC started from autostart has no
// console at all: GetStdHandle hands back an invalid handle and every write to
// os.Stderr fails with ERROR_INVALID_HANDLE. With MultiWriter(os.Stderr, f)
// that failure would return before the file was ever touched -- silently
// producing an empty station.log on precisely the machines whose only
// diagnostic is station.log. Ordering the two would also fix it, which is
// exactly why this is a named type instead: the fix must not look like an
// arbitrary argument order somebody can tidy up later.
type tee struct {
	file    io.Writer
	console io.Writer
}

func (t tee) Write(p []byte) (int, error) {
	n, err := t.file.Write(p)
	if t.console != nil {
		_, _ = t.console.Write(p)
	}
	return n, err
}

func logDirs(stateDir string) []string {
	dirs := []string{stateDir}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		dirs = append(dirs, filepath.Join(local, "AvtoTest", "station"))
	}
	return dirs
}

func rotateIfLarge(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < maxLogBytes {
		return
	}
	_ = os.Remove(path + ".1")
	_ = os.Rename(path, path+".1")
}

// fatal ends the process, and is now reachable only before the listener is
// bound -- once the kiosk has somewhere to connect to, every remaining failure
// is reported through the status page instead of killing the classroom.
//
// It used to print to the console and then block for up to two minutes on
// "Press Enter to close". On a GUI build there is no console to print to, so
// the message goes to the log and to a message box the school can actually
// read, and the process exits immediately.
func fatal(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	fmt.Fprintln(os.Stderr, msg)
	showFatal(msg)
	os.Exit(1)
}
