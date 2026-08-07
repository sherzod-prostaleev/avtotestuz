//go:build windows

package selfinstall

import (
	"fmt"
	"os/exec"
	"strings"
)

// desktopScript is the PowerShell that finds every desktop folder and drops a
// shortcut in each.
//
// Written for PowerShell 2.0 / .NET 2.0, which is what Windows 7 ships and
// what a driving school's classroom actually runs. Three things the first
// version used are simply absent there, and because $ErrorActionPreference is
// Stop, the first one to throw killed the whole script and no shortcut was
// created anywhere:
//
//   - [Environment]::GetFolderPath('UserProfile') -- added in .NET 4.0.
//   - [Environment]::GetFolderPath('CommonDesktopDirectory') -- also .NET 4.0.
//   - [string]::IsNullOrWhiteSpace -- .NET 4.0 as well.
//
// So the folders come from WScript.Shell's SpecialFolders collection, which is
// COM and has behaved the same since Windows 95 -- and, like GetFolderPath,
// follows a OneDrive redirect -- with plain environment variables as the
// fallback. Each candidate is resolved inside its own try/catch so one
// unavailable source can never cost the others.
const desktopScript = `
$ErrorActionPreference = 'Stop'
$sh = New-Object -ComObject WScript.Shell
$dirs = @()
try { $d = $sh.SpecialFolders.Item('Desktop');         if ($d) { $dirs += $d } } catch {}
try { $d = $sh.SpecialFolders.Item('AllUsersDesktop'); if ($d) { $dirs += $d } } catch {}
try { $d = [Environment]::GetFolderPath('Desktop');          if ($d) { $dirs += $d } } catch {}
try { $d = [Environment]::GetFolderPath('DesktopDirectory'); if ($d) { $dirs += $d } } catch {}
try { if ($env:USERPROFILE) { $dirs += (Join-Path $env:USERPROFILE 'Desktop') } } catch {}
try { if ($env:PUBLIC)      { $dirs += (Join-Path $env:PUBLIC 'Desktop') } } catch {}
`

// createShortcut writes a .lnk to every desktop folder this machine has.
//
// Every desktop, not one: with OneDrive folder backup the real desktop is
// %USERPROFILE%\OneDrive\Desktop while %USERPROFILE%\Desktop still exists, so
// writing to a single hand-built path puts the icon somewhere the student
// never looks.
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

	script := desktopScript + strings.Join([]string{
		"$target = " + ps(target),
		"$name = " + ps(shortcutName+".lnk"),
		"$seen = @()",
		"foreach ($d in $dirs) {",
		"  if (-not $d) { continue }",
		"  if ($d.ToString().Trim().Length -eq 0) { continue }",
		"  if (-not (Test-Path -LiteralPath $d -PathType Container)) { continue }",
		"  $full = (Resolve-Path -LiteralPath $d).Path",
		"  if ($seen -contains $full) { continue }",
		"  $seen += $full",
		"  $lnk = Join-Path $full $name",
		"  if (Test-Path -LiteralPath $lnk) { Write-Output ('EXISTS ' + $lnk); continue }",
		"  try {",
		"    $s = $sh.CreateShortcut($lnk)",
		"    $s.TargetPath = $target",
		"    $s.WorkingDirectory = Split-Path -Parent $target",
		"    $s.IconLocation = $target + ',0'",
		"    $s.Description = " + ps(shortcutName),
		"    $s.Save()",
		"    Write-Output ('CREATED ' + $lnk)",
		// A desktop the current account may not write to is ordinary, not a
		// failure: the all-users desktop needs elevation on most machines.
		"  } catch { Write-Output ('SKIPPED ' + $lnk) }",
		"}",
		"if ($seen.Count -eq 0) { Write-Error 'no desktop folder found' }",
	}, "\n")

	out, err := runPowerShell(script)
	if err != nil {
		return "", false, err
	}

	var created, existing []string
	for _, line := range strings.Split(out, "\n") {
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
	return "", false, fmt.Errorf("no desktop folder accepted a shortcut: %s", out)
}

// removeShortcut deletes the shortcut from every desktop during -uninstall. A
// missing file is success, matching the rest of Remove.
func removeShortcut() error {
	script := desktopScript + strings.Join([]string{
		"$name = " + "'" + shortcutName + ".lnk'",
		"foreach ($d in $dirs) {",
		"  if (-not $d) { continue }",
		"  Remove-Item -LiteralPath (Join-Path $d $name) -Force -ErrorAction SilentlyContinue",
		"}",
	}, "\n")
	_, _ = runPowerShell(script) // best effort: a leftover icon must not fail an uninstall
	return nil
}

// runPowerShell executes script and returns its combined output.
//
// -Version 2 is not passed: forcing a version fails on machines where that
// engine is not installed, and the script is written to the 2.0 subset so the
// default engine -- whatever it is -- can run it.
func runPowerShell(script string) (string, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass", "-Command", script)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return text, fmt.Errorf("powershell: %w: %s", err, text)
	}
	return text, nil
}
