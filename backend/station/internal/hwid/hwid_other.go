//go:build !windows

package hwid

import "os"

// rawParts reads /etc/machine-id. Development only — see keystore_other.go.
func rawParts() ([]string, error) {
	b, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return nil, err
	}
	return []string{"machine-id:" + string(b)}, nil
}
