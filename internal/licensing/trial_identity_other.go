//go:build !windows

package licensing

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

// DeviceFingerprint uses the persistent per-user data location outside Windows.
func DeviceFingerprint() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte("hesora-trial-v1:" + filepath.Clean(base)))
	return hex.EncodeToString(sum[:]), nil
}
