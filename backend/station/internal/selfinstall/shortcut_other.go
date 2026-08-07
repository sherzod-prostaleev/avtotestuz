//go:build !windows

package selfinstall

// A desktop shortcut is a Windows shell concept and the agent only ships
// there; off Windows this reports "nothing to do" so the caller's flow is
// identical on a developer machine.
func createShortcut(string) (string, bool, error) { return "", false, nil }

func removeShortcut() error { return nil }
