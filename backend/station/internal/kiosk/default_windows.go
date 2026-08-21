//go:build windows

package kiosk

import "os/exec"

// launchDefault opens url in whatever browser Windows considers the default.
//
// rundll32 url.dll,FileProtocolHandler is used rather than `cmd /c start`
// because it needs no shell, takes the URL as a single argument with no
// quoting rules to get wrong, and works unchanged from Windows 7 onwards. The
// window it opens is an ordinary browser window -- tabs and address bar
// included -- which is a worse kiosk than --app but an infinitely better
// outcome than a PC where nothing appears at all.
func launchDefault(url string) (*exec.Cmd, bool) {
	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
	if err := cmd.Start(); err != nil {
		return nil, false
	}
	return cmd, true
}
