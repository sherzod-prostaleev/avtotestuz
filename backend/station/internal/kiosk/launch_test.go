package kiosk

import (
	"runtime"
	"strings"
	"testing"
)

// The window flags are the whole reason this file exists. 1.3.2 shipped
// --start-maximized alone and it was not enough on Windows -- the first launch
// was maximised and every one after it was small again, because Chrome
// restores the placement it saved for the app. The explicit size is what wins
// there, so both have to survive. See Launch's comment.
func TestBrowserArgsAskForAFullScreenWindow(t *testing.T) {
	args := browserArgs("https://drivergo.uz/uz-Latn/station")

	if args[0] != "--app=https://drivergo.uz/uz-Latn/station" {
		t.Fatalf("--app must come first and carry the url, got %q", args[0])
	}
	if !has(args, "--start-maximized") {
		t.Error("--start-maximized is missing: the kiosk would open in a small window")
	}
	// --kiosk and --start-fullscreen are deliberately not used: a teacher has
	// to be able to reach Windows without knowing a keyboard shortcut.
	for _, banned := range []string{"--kiosk", "--start-fullscreen"} {
		if has(args, banned) {
			t.Errorf("%s traps the classroom with no visible way out", banned)
		}
	}
	// The old hard-coded size is what put a small window on every screen.
	for _, a := range args {
		if a == "--window-size=1280,860" {
			t.Error("the hard-coded 1280x860 window is back")
		}
	}
}

// On Windows the size is stated explicitly from the monitor's work area, which
// is the half that actually survives Chrome's saved placement. Everywhere else
// workArea declines and --start-maximized is left to do the job alone.
func TestBrowserArgsStateTheWorkAreaOnWindows(t *testing.T) {
	args := browserArgs("https://example.com")
	sized := prefixed(args, "--window-size=")
	positioned := prefixed(args, "--window-position=")

	if runtime.GOOS != "windows" {
		if sized != "" || positioned != "" {
			t.Fatalf("workArea should decline off Windows, got %q %q", sized, positioned)
		}
		return
	}
	if sized == "" || positioned == "" {
		t.Fatalf("Windows must state the work area explicitly, got %v", args)
	}
	if sized == "--window-size=0,0" {
		t.Error("a zero-sized window would open a sliver on the classroom screen")
	}
}

func has(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func prefixed(args []string, prefix string) string {
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			return a
		}
	}
	return ""
}
