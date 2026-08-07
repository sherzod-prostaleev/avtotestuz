//go:build !windows

package selfinstall

// registerAutostart and unregisterAutostart are no-ops off Windows. This
// build is for development only: without a real autostart mechanism the
// agent does not persist across reboots, and a non-Windows agent must never
// be handed to a school.
func registerAutostart(path string) error { return nil }

func unregisterAutostart() error { return nil }

// SelfTestAutostart reports "not applicable" rather than faking a pass: off
// Windows there is no registry to round-trip against (see
// registerAutostart above), so claiming a verified success here would be
// dishonest about what this run actually checked -- the same reasoning
// checkFileIsSealed already applies to the development keystore.
func SelfTestAutostart(path string) (ok bool, info string) {
	return false, "not applicable on this build: autostart is a Windows-only registry entry (see selfinstall_other.go)"
}

// SelfTestTargetWritable reports "not applicable" for the same reason: the
// question this check exists to answer -- can an ordinary, non-elevated
// Windows user write into the real deployment directory -- has no
// non-Windows equivalent worth claiming a verified pass for (see
// selfinstall_windows.go).
func SelfTestTargetWritable(stateDir string) (ok bool, info string) {
	return false, "not applicable on this build: this checks whether the real Windows install directory needs no elevation (see selfinstall_other.go)"
}
