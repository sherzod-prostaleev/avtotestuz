//go:build windows

package hwid

import "golang.org/x/sys/windows/registry"

// rawParts reads MachineGuid, the install-time machine identity Windows keeps
// stable across reboots and user changes but regenerates on reimage.
func rawParts() ([]string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return nil, err
	}
	defer func() { _ = k.Close() }()
	guid, _, err := k.GetStringValue("MachineGuid")
	if err != nil {
		return nil, err
	}
	return []string{"machineguid:" + guid}, nil
}
