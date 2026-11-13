package datadir

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

const ApplicationDirectory = "invoice-maker"

// Directory returns %LOCALAPPDATA%\invoice-maker on Windows.
// It never places mutable user data next to the installed executable.
func Directory() (string, error) {
	root := os.Getenv("LOCALAPPDATA")
	if runtime.GOOS == "windows" && root == "" {
		return "", errors.New("متغیر LOCALAPPDATA در ویندوز تنظیم نشده است")
	}
	if root == "" {
		var err error
		root, err = os.UserCacheDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(root, ApplicationDirectory), nil
}

func Ensure() (string, error) {
	dir, err := Directory()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	if err := hidePath(dir); err != nil {
		// Hiding is a convenience, not a security boundary; do not prevent app startup.
		return dir, nil
	}
	return dir, nil
}
