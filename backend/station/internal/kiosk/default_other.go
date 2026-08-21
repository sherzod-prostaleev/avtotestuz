//go:build !windows

package kiosk

import "os/exec"

// launchDefault opens url in the desktop's default handler. xdg-open only
// exists on Linux desktops, and this path is only ever exercised in
// development -- the shipped article is always the Windows one.
func launchDefault(url string) (*exec.Cmd, bool) {
	cmd := exec.Command("xdg-open", url)
	if err := cmd.Start(); err != nil {
		return nil, false
	}
	return cmd, true
}
