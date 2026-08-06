// Package kiosk starts the local browser in full-screen kiosk mode.
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

// Launch opens url in kiosk mode and returns the running process.
func Launch(url string) (*exec.Cmd, error) {
	for _, bin := range candidates() {
		path, err := exec.LookPath(bin)
		if err != nil {
			continue
		}
		cmd := exec.Command(path,
			"--kiosk",
			"--app="+url,
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
