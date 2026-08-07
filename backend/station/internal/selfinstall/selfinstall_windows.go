//go:build windows

package selfinstall

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// runKeyPath and runValueName place the agent in the per-user autostart
// list.
//
// HKCU rather than HKLM is deliberate: HKLM needs administrator rights, and
// a classroom PC usually runs as an ordinary user, so requiring elevation
// would mean the agent silently fails to persist on exactly the machines it
// targets.
const (
	runKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "AvtoTestStation"
)

// quote wraps path the way the Run key expects: a literal double-quoted
// path. This is NOT fmt.Sprintf("%q", path) -- Go's %q escaping doubles
// every backslash, which would write garbage neither the shell nor the Run
// key's own launcher understands for a Windows path.
func quote(path string) string {
	return `"` + path + `"`
}

func registerAutostart(path string) error {
	return setAutostartValue(quote(path))
}

func setAutostartValue(raw string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer func() { _ = k.Close() }()
	return k.SetStringValue(runValueName, raw)
}

func unregisterAutostart() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	defer func() { _ = k.Close() }()
	if err := k.DeleteValue(runValueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}

// readAutostartValue reads the raw Run value back, reporting present=false
// rather than an error when the key or value simply does not exist -- that
// is the ordinary "nothing registered yet" case, not a failure.
func readAutostartValue() (value string, present bool, err error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	defer func() { _ = k.Close() }()
	v, _, err := k.GetStringValue(runValueName)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	return v, true, nil
}

// SelfTestAutostart exists only for the -selftest diagnostic (see
// keystore.KeyPath for the same "let diagnostics peek behind the interface"
// precedent): it saves whatever autostart value is currently registered,
// registers path, reads the value straight back out of the registry,
// restores the saved value (or removes the entry if there was none), and
// reports whether what came back matches what was written.
func SelfTestAutostart(path string) (ok bool, info string) {
	prevValue, prevPresent, err := readAutostartValue()
	if err != nil {
		return false, fmt.Sprintf("could not read the current autostart value: %v", err)
	}

	if err := registerAutostart(path); err != nil {
		return false, fmt.Sprintf("registerAutostart: %v", err)
	}

	got, present, readErr := readAutostartValue()

	var restoreErr error
	if prevPresent {
		restoreErr = setAutostartValue(prevValue)
	} else {
		restoreErr = unregisterAutostart()
	}

	if readErr != nil {
		return false, fmt.Sprintf("could not read back the registered value: %v", readErr)
	}
	if !present {
		return false, "registered a value but reading it back found nothing"
	}
	want := quote(path)
	if got != want {
		return false, fmt.Sprintf("read back %q, want %q", got, want)
	}
	if restoreErr != nil {
		return false, fmt.Sprintf("round trip matched but restoring the previous value failed: %v", restoreErr)
	}
	return true, fmt.Sprintf(`registered %s under HKCU\%s\%s, read it back unchanged, and restored the previous value`, want, runKeyPath, runValueName)
}

// SelfTestTargetWritable exists for the -selftest diagnostic: it proves the
// real directory this machine's install would use -- not a disposable
// scratch temp directory, which is writable everywhere and proves nothing
// about this specific machine -- can actually be created and written by
// whichever user is running this process, with no elevation.
func SelfTestTargetWritable(stateDir string) (ok bool, info string) {
	return targetWritable(stateDir)
}
