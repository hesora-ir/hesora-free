//go:build windows

package licensing

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// DeviceFingerprint hashes the Windows machine identifier before it leaves the device.
func DeviceFingerprint() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", err
	}
	defer key.Close()
	value, _, err := key.GetStringValue("MachineGuid")
	if err != nil {
		return "", err
	}
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", registry.ErrNotExist
	}
	sum := sha256.Sum256([]byte("hesora-trial-v1:" + value))
	return hex.EncodeToString(sum[:]), nil
}
