//go:build windows

package selfinstall

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// desktopDir picks where the shortcut goes.
//
// The Public desktop first, so every student who logs into a shared classroom
// PC sees the same icon rather than only the account that happened to run the
// installer. A locked-down profile may not be allowed to write there, so the
// current user's own desktop is the fallback -- one icon for one account
// beats no icon at all.
func desktopDir() (string, error) {
	if pub := os.Getenv("PUBLIC"); pub != "" {
		dir := filepath.Join(pub, "Desktop")
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			if f, err := os.CreateTemp(dir, ".avtotest-write-*"); err == nil {
				name := f.Name()
				_ = f.Close()
				_ = os.Remove(name)
				return dir, nil
			}
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Desktop"), nil
}

// createShortcut writes a .lnk through the shell's own COM object.
//
// PowerShell rather than a hand-written .lnk: the format (MS-SHLLINK) needs a
// LinkTargetIDList to be resolved reliably by every Explorer version, and
// producing one by hand means reimplementing shell item ids for a cosmetic
// feature. WScript.Shell is the same API every Windows installer uses.
//
// The icon is taken from the agent's own binary (index 0), so the shortcut
// shows the DriverGo emblem compiled into it and there is no separate .ico
// file on disk to go missing.
func createShortcut(target string) (string, bool, error) {
	dir, err := desktopDir()
	if err != nil {
		return "", false, err
	}
	path := filepath.Join(dir, shortcutName+".lnk")
	if _, err := os.Stat(path); err == nil {
		return path, false, nil
	} else if !os.IsNotExist(err) {
		return "", false, err
	}

	// Single-quoted PowerShell strings take a literal backslash; the only
	// escape needed is a doubled single quote. Paths here are machine-local
	// and not user input, but quoting them properly costs nothing and keeps a
	// directory with an apostrophe from breaking the script.
	ps := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

	script := strings.Join([]string{
		"$ErrorActionPreference = 'Stop'",
		"$s = (New-Object -ComObject WScript.Shell).CreateShortcut(" + ps(path) + ")",
		"$s.TargetPath = " + ps(target),
		"$s.WorkingDirectory = " + ps(filepath.Dir(target)),
		"$s.IconLocation = " + ps(target+",0"),
		"$s.Description = " + ps(shortcutName),
		"$s.Save()",
	}, "; ")

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass", "-Command", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", false, fmt.Errorf("powershell: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if _, err := os.Stat(path); err != nil {
		return "", false, fmt.Errorf("powershell reported success but %s is not there", path)
	}
	return path, true, nil
}

// removeShortcut deletes the shortcut during -uninstall. A missing file is
// success, matching the rest of Remove.
func removeShortcut() error {
	dir, err := desktopDir()
	if err != nil {
		return nil
	}
	if err := os.Remove(filepath.Join(dir, shortcutName+".lnk")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
