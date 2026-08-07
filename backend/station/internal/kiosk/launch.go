// Package kiosk starts the local browser on the classroom station page.
package kiosk

import (
	"errors"
	"os/exec"
	"runtime"
)

// ErrNoBrowser means neither Chrome nor Edge was found.
var ErrNoBrowser = errors.New("no supported browser found (install Google Chrome or Microsoft Edge)")

// candidates lists browsers in preference order.
func candidates() []string {
	if runtime.GOOS == "windows" {
		return []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		}
	}
	return []string{"google-chrome", "chromium", "chromium-browser"}
}

// Launch opens url in an app window and returns the running process.
//
// --app without --kiosk on purpose. Full-screen kiosk mode covers the whole
// screen with no title bar, so a student cannot minimise, resize or close the
// window -- on a shared classroom PC that reads as the machine being stuck
// rather than as a focused exam, and there is no way back to Windows without
// knowing a keyboard shortcut. --app still drops the tabs, the address bar and
// the bookmarks, so there is nowhere else to browse, but keeps the ordinary
// minimise / maximise / close controls.
func Launch(url string) (*exec.Cmd, error) {
	for _, bin := range candidates() {
		path, err := exec.LookPath(bin)
		if err != nil {
			continue
		}
		cmd := exec.Command(path,
			"--app="+url,
			"--window-size=1280,860",
			"--no-first-run",
			"--disable-session-crashed-bubble",
			"--disable-features=TranslateUI",
		)
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return cmd, nil
	}
	return nil, ErrNoBrowser
}
