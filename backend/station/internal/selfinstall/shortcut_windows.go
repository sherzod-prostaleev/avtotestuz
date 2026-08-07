//go:build windows

package selfinstall

import (
	"fmt"
	"os/exec"
	"strings"
)

// createShortcut writes a .lnk to every desktop folder this machine actually
// has, through the shell's own COM object.
//
// Every desktop, not one: on a PC with OneDrive folder backup the real desktop
// is %USERPROFILE%\OneDrive\Desktop, while %USERPROFILE%\Desktop still exists
// and is what a hand-built path resolves to -- so writing to one of them puts
// the icon somewhere the student never looks. The first build did exactly
// that. Windows itself knows which is which, so the paths come from
// GetFolderPath rather than from string concatenation here, and both the
// per-user desktop and the all-users one get an icon when they are distinct
// and writable.
//
// PowerShell rather than a hand-written .lnk: the format (MS-SHLLINK) needs a
// LinkTargetIDList to resolve reliably in every Explorer version, and
// producing one by hand means reimplementing shell item ids for a cosmetic
// feature. WScript.Shell is the same API every Windows installer uses.
//
// The icon is taken from the agent's own binary (index 0), so the shortcut
// shows the DriverGo emblem compiled into it and there is no separate .ico
// file on disk to go missing.
func createShortcut(target string) (string, bool, error) {
	// Single-quoted PowerShell strings take a literal backslash; the only
	// escape needed is a doubled single quote. Paths here are machine-local
	// and not user input, but quoting them properly keeps a directory with an
	// apostrophe from breaking the script.
	ps := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

	script := strings.Join([]string{
		"$ErrorActionPreference = 'Stop'",
		"$target = " + ps(target),
		"$name = " + ps(shortcutName+".lnk"),
		// CommonDesktopDirectory is usually admin-only; Desktop follows a
		// OneDrive redirect; UserProfile\Desktop is the folder that stays
		// behind after such a redirect and is still shown by some setups.
		"$dirs = @(" + strings.Join([]string{
			"[Environment]::GetFolderPath('Desktop')",
			"(Join-Path ([Environment]::GetFolderPath('UserProfile')) 'Desktop')",
			"[Environment]::GetFolderPath('CommonDesktopDirectory')",
		}, ", ") + ")",
		"$made = @(); $seen = @()",
		"foreach ($d in $dirs) {",
		"  if ([string]::IsNullOrWhiteSpace($d)) { continue }",
		"  if (-not (Test-Path -LiteralPath $d -PathType Container)) { continue }",
		"  $full = (Resolve-Path -LiteralPath $d).Path",
		"  if ($seen -contains $full) { continue }",
		"  $seen += $full",
		"  $lnk = Join-Path $full $name",
		"  if (Test-Path -LiteralPath $lnk) { Write-Output ('EXISTS ' + $lnk); continue }",
		"  try {",
		"    $s = (New-Object -ComObject WScript.Shell).CreateShortcut($lnk)",
		"    $s.TargetPath = $target",
		"    $s.WorkingDirectory = Split-Path -Parent $target",
		"    $s.IconLocation = $target + ',0'",
		"    $s.Description = " + ps(shortcutName),
		"    $s.Save()",
		"    $made += $lnk",
		"    Write-Output ('CREATED ' + $lnk)",
		// A desktop the current account may not write to is ordinary, not a
		// failure: the all-users desktop needs elevation on most machines.
		"  } catch { Write-Output ('SKIPPED ' + $lnk) }",
		"}",
		"if ($seen.Count -eq 0) { Write-Error 'no desktop folder found' }",
	}, "\n")

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass", "-Command", script)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return "", false, fmt.Errorf("powershell: %w: %s", err, text)
	}

	var created, existing []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "CREATED "):
			created = append(created, strings.TrimPrefix(line, "CREATED "))
		case strings.HasPrefix(line, "EXISTS "):
			existing = append(existing, strings.TrimPrefix(line, "EXISTS "))
		}
	}
	if len(created) > 0 {
		return strings.Join(created, ", "), true, nil
	}
	if len(existing) > 0 {
		return strings.Join(existing, ", "), false, nil
	}
	// Every candidate was skipped -- report it rather than claim success, so
	// the console says why no icon appeared.
	return "", false, fmt.Errorf("no desktop folder accepted a shortcut: %s", text)
}

// removeShortcut deletes the shortcut from every desktop during -uninstall. A
// missing file is success, matching the rest of Remove.
func removeShortcut() error {
	script := strings.Join([]string{
		"$ErrorActionPreference = 'SilentlyContinue'",
		"$name = '" + shortcutName + ".lnk'",
		"foreach ($d in @([Environment]::GetFolderPath('Desktop'), " +
			"(Join-Path ([Environment]::GetFolderPath('UserProfile')) 'Desktop'), " +
			"[Environment]::GetFolderPath('CommonDesktopDirectory'))) {",
		"  if ([string]::IsNullOrWhiteSpace($d)) { continue }",
		"  Remove-Item -LiteralPath (Join-Path $d $name) -Force -ErrorAction SilentlyContinue",
		"}",
	}, "\n")
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass", "-Command", script)
	_ = cmd.Run() // best effort: a leftover icon must not fail an uninstall
	return nil
}
